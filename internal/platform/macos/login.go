//go:build darwin && !ios

package macos

import (
	"bytes"
	"encoding/xml"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// smAppService is unavailable before macOS 13. The older systems use a per-user LaunchAgent, loaded on the next login rather than immediately.
var smAppService = sync.OnceValue(func() objc.ID {
	if _, err := purego.Dlopen("/System/Library/Frameworks/ServiceManagement.framework/ServiceManagement",
		purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		return 0
	}
	return objc.ID(apple.Class("SMAppService"))
})

type launchAgent struct {
	path, id, executable string
}

func appLaunchAgent() (launchAgent, error) {
	bundle := objc.ID(apple.Class("NSBundle")).Send(sel("mainBundle"))
	id := apple.GoString(bundle.Send(sel("bundleIdentifier")))
	executable := apple.GoString(bundle.Send(sel("executablePath")))
	if !strings.HasSuffix(apple.GoString(bundle.Send(sel("bundlePath"))), ".app") ||
		id == "" || strings.ContainsAny(id, "/\\") || executable == "" {
		return launchAgent{}, errors.New("launch at login requires an app bundle with an identifier and executable")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return launchAgent{}, err
	}
	return launchAgent{filepath.Join(home, "Library", "LaunchAgents", id+".plist"), id, executable}, nil
}

// plist builds launchd's system format. The app executable is one argument, even when its path contains spaces or XML metacharacters; no shell runs it.
func (a launchAgent) plist() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString(xml.Header)
	b.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n<plist version=\"1.0\"><dict>\n<key>Label</key><string>")
	if err := xml.EscapeText(&b, []byte(a.id)); err != nil {
		return nil, err
	}
	b.WriteString("</string>\n<key>ProgramArguments</key><array><string>")
	if err := xml.EscapeText(&b, []byte(a.executable)); err != nil {
		return nil, err
	}
	b.WriteString("</string><string>--nsgo-launched-at-login</string></array>\n<key>RunAtLoad</key><true/>\n<key>LimitLoadToSessionType</key><string>Aqua</string>\n</dict></plist>\n")
	return b.Bytes(), nil
}

func (a launchAgent) set(on bool) error {
	if !on {
		err := os.Remove(a.path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	data, err := a.plist()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(a.path), ".nsgo-login-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// launchd accepts a user-owned, non-group/world-writable plist; the temporary file's 0600 mode preserves that requirement on replacement.
	return os.Rename(tmp.Name(), a.path)
}

func (a launchAgent) enabled() bool {
	data, err := os.ReadFile(a.path)
	if err != nil {
		return false
	}
	want, err := a.plist()
	return err == nil && bytes.Equal(data, want)
}

func setLaunchAtLogin(on bool) {
	a, err := appLaunchAgent()
	if err != nil {
		log.Printf("nsgo: launch at login: %v", err)
		return
	}
	cls := smAppService()
	if cls == 0 {
		if err := a.set(on); err != nil {
			log.Printf("nsgo: launch at login: %v", err)
		}
		return
	}
	service := cls.Send(sel("mainAppService"))
	var failure objc.ID
	method := "unregisterAndReturnError:"
	if on {
		method = "registerAndReturnError:"
	}
	if !objc.Send[bool](service, sel(method), unsafe.Pointer(&failure)) {
		log.Printf("nsgo: launch at login: %s", apple.GoString(failure.Send(sel("localizedDescription"))))
		if on {
			return
		}
	}
	// An OS upgrade must not leave both a LaunchAgent and SMAppService registered. A disable also removes the fallback left by that upgrade.
	if err := a.set(false); err != nil {
		log.Printf("nsgo: removing legacy login item: %v", err)
	}
}

func launchesAtLogin() bool {
	if cls := smAppService(); cls != 0 {
		const enabled = 1
		if objc.Send[int](cls.Send(sel("mainAppService")), sel("status")) == enabled {
			return true
		}
	}
	a, err := appLaunchAgent()
	return err == nil && a.enabled()
}

func loginLaunchEvent() bool {
	manager := objc.ID(apple.Class("NSAppleEventManager")).Send(sel("sharedAppleEventManager"))
	return isLoginLaunchEvent(manager.Send(sel("currentAppleEvent")))
}

func isLoginLaunchEvent(event objc.ID) bool {
	const (
		coreEvent = uint32(0x61657674) // 'aevt'
		openApp   = uint32(0x6f617070) // 'oapp'
		propData  = uint32(0x70726474) // 'prdt', keyAEPropData
		loginItem = uint32(0x6c676974) // 'lgit', keyAELaunchedAsLogInItem
	)
	if event == 0 || objc.Send[uint32](event, sel("eventClass")) != coreEvent || objc.Send[uint32](event, sel("eventID")) != openApp {
		return false
	}
	// keyAEPropData is an enum whose value names the launch reason, not a record of boolean keys. Other reasons (such as a service) stay false.
	data := event.Send(sel("paramDescriptorForKeyword:"), propData)
	return data != 0 && objc.Send[uint32](data, sel("enumCodeValue")) == loginItem
}
