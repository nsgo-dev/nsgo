//go:build windows

package windows

import (
	"log"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

var setCurrentProcessExplicitAppUserModelID = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
var addToRecentDocs = shell32.NewProc("SHAddToRecentDocs")

var (
	clsidDestinationList  = parseGUID("77f10cf0-3db5-4966-b520-b7c54fd35ed6")
	iidDestinationList    = parseGUID("6332debf-87b5-4670-90c0-5e57b408a49e")
	clsidObjectCollection = parseGUID("2d3468c1-36a7-43b6-ac24-d3f02fd9607a")
	iidObjectCollection   = parseGUID("5632b1a4-e38a-400a-928a-d4cd63230295")
)

const (
	destinationSetAppID   = 3
	destinationBeginList  = 4
	destinationAddTasks   = 7
	destinationCommitList = 8
	objectCollectionAdd   = 5
	shellLinkSetDesc      = 7
	shellLinkSetArgs      = 11
	shellLinkSetPath      = 20
)

func appUserModelID(id string) string {
	if id == "" {
		return "dev.nsgo.app"
	}
	return id
}

func setProcessAppUserModelID(id string) {
	name := syscall.StringToUTF16(appUserModelID(id))
	setCurrentProcessExplicitAppUserModelID.Call(uintptr(unsafe.Pointer(&name[0])))
	runtime.KeepAlive(name)
}

func recentFile(path string, clearRecent bool) {
	const shardPathW = 3
	if clearRecent {
		addToRecentDocs.Call(shardPathW, 0)
		return
	}
	name := syscall.StringToUTF16(path)
	addToRecentDocs.Call(shardPathW, uintptr(unsafe.Pointer(&name[0])))
	runtime.KeepAlive(name)
}

// setDockMenu writes top-level action items as Jump List tasks. Windows has
// no native representation for submenus, separators or check marks, so those
// entries are skipped while the same MenuItem API remains usable everywhere.
func (w *window) setDockMenu(entries []driver.MenuEntry) {
	if w.opts.test != nil {
		return
	}
	dest := newCOMObject(&clsidDestinationList, &iidDestinationList)
	if dest == nil {
		return
	}
	defer dest.release()
	id := syscall.StringToUTF16(appUserModelID(w.opts.ID))
	if failed(dest.call(destinationSetAppID, uintptr(unsafe.Pointer(&id[0])))) {
		return
	}
	var count uint32
	var removed *comObject
	if failed(dest.call(destinationBeginList, uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&removed)))) {
		return
	}
	if removed != nil {
		removed.release()
	}
	collection := newCOMObject(&clsidObjectCollection, &iidObjectCollection)
	if collection == nil {
		return
	}
	defer collection.release()
	exe, err := os.Executable()
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.Separator || entry.Disabled || entry.ID == 0 || len(entry.Items) != 0 {
			continue
		}
		link := newCOMObject(&clsidShellLink, &iidShellLink)
		if link == nil {
			continue
		}
		path := syscall.StringToUTF16(exe)
		args := syscall.StringToUTF16(dockMenuArgument + formatUint(entry.ID))
		desc := syscall.StringToUTF16(entry.Label)
		ok := !failed(link.call(shellLinkSetPath, uintptr(unsafe.Pointer(&path[0])))) &&
			!failed(link.call(shellLinkSetArgs, uintptr(unsafe.Pointer(&args[0])))) &&
			!failed(link.call(shellLinkSetDesc, uintptr(unsafe.Pointer(&desc[0])))) &&
			!failed(collection.call(objectCollectionAdd, link.ptr()))
		runtime.KeepAlive(path)
		runtime.KeepAlive(args)
		runtime.KeepAlive(desc)
		link.release()
		if !ok {
			continue
		}
	}
	if failed(dest.call(destinationAddTasks, collection.ptr())) {
		return
	}
	if failed(dest.call(destinationCommitList)) {
		log.Printf("nsgo: committing the Jump List failed")
	}
}

func newCOMObject(clsid, iid *guid) *comObject {
	var object *comObject
	hr, _, _ := coCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&object)))
	if failed(hr) {
		return nil
	}
	return object
}

func formatUint(n uint32) string {
	if n == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
