package main

import (
	"context"
	"debug/macho"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	// Keep the target-specific extension in apps' vendored tool dependencies.
	_ "github.com/nsgo-dev/nsgo/cmd/nsgo/shareextension"
)

const shareAppex = "NSGoShare.appex"
const shareExecutable = "NSGoShare"
const shareGroupKey = "NSGoShareAppGroup"
const shareTypesKey = "NSGoShareTypes"
const shareNotificationKey = "NSGoShareNotification"

func (p *project) iosShares() bool { return len(p.Apple.ShareTypes) > 0 && p.clipParent == "" }
func (p *project) sharePlist() string {
	if !p.iosShares() {
		return ""
	}
	return "<key>" + shareTypesKey + "</key>" + plistStrings(p.Apple.ShareTypes) + "<key>" + shareGroupKey + "</key><string>" + xmlText(p.appGroup()) + "</string><key>" + shareNotificationKey + "</key><string>" + xmlText(p.ID+".nsgo-shared") + "</string>"
}
func (p *project) shareActivation() string {
	var tests []string
	for _, uti := range p.Apple.ShareTypes {
		tests = append(tests, "ANY $attachment.registeredTypeIdentifiers UTI-CONFORMS-TO '"+uti+"'")
	}
	return "extensionItems.@count > 0 AND SUBQUERY(extensionItems, $item, $item.attachments.@count > 0 AND SUBQUERY($item.attachments, $attachment, " + strings.Join(tests, " OR ") + ").@count == $item.attachments.@count).@count == extensionItems.@count"
}
func iosSharePath(app string) string { return filepath.Join(app, "PlugIns", shareAppex) }

func shareInfo(p *project, platform map[string]any) map[string]any {
	types := make([]any, len(p.Apple.ShareTypes))
	for i, uti := range p.Apple.ShareTypes {
		types[i] = uti
	}
	out := map[string]any{
		"CFBundleIdentifier":            p.ID + ".share",
		"CFBundleName":                  p.Name,
		"CFBundleInfoDictionaryVersion": "6.0",
		"CFBundleDisplayName":           p.Name,
		"CFBundleExecutable":            shareExecutable,
		"CFBundlePackageType":           "XPC!",
		"CFBundleShortVersionString":    p.Version,
		"CFBundleVersion":               p.BundleVersion(),
		"MinimumOSVersion":              iOS.minimum,
		"UIDeviceFamily":                []any{int64(1), int64(2)},
		"NSExtension": map[string]any{
			"NSExtensionPointIdentifier": "com.apple.share-services",
			"NSExtensionPrincipalClass":  "NSGoShareController",
			"NSExtensionAttributes":      map[string]any{"NSExtensionActivationRule": p.shareActivation()},
		},
		shareGroupKey:        p.appGroup(),
		shareTypesKey:        types,
		shareNotificationKey: p.ID + ".nsgo-shared",
	}
	for k, v := range platform {
		out[k] = v
	}
	return out
}

// shareMain is the extension's main package, which exists only during the build. Its code runs from init, as a static library's does; main is never called.
const shareMain = `package main

import "C"

import "github.com/nsgo-dev/nsgo/cmd/nsgo/shareextension"

func init() { shareextension.Register() }

func main() {}
`

// addIOSShare builds a small Go host using only extension-safe Foundation and UIKit APIs. Go builds it as a static library, which clang links into an executable whose entry point is Foundation's NSExtensionMain, as Xcode links an app extension: App Store Connect rejects an extension whose entry point is anything else (#2718).
func addIOSShare(ctx context.Context, p *project, app string, env []string, simulator bool) error {
	if !p.iosShares() {
		return nil
	}
	stage(ctx, "building the share extension")
	dir, err := os.MkdirTemp("", "nsgo-share-extension-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	source := filepath.Join(dir, "main.go")
	if err := os.WriteFile(source, []byte(shareMain), 0o600); err != nil {
		return err
	}
	appex := iosSharePath(app)
	if err := os.MkdirAll(appex, 0o755); err != nil {
		return err
	}
	archive := filepath.Join(dir, "share.a")
	if err := goTool(ctx, p.Dir, env, "build", "-trimpath", "-buildvcs=false", "-buildmode=c-archive", "-ldflags", "-s -w", "-o", archive, source); err != nil {
		return err
	}
	cc := envValue(env, "CC")
	if len(cc) == 0 {
		return fmt.Errorf("the share extension needs CC in the build environment")
	}
	executable := filepath.Join(appex, shareExecutable)
	if err := run(ctx, cc[0], shareLinkArgs(cc[1:], archive, executable)...); err != nil {
		return err
	}
	if err := checkShareEntry(executable); err != nil {
		return err
	}
	platform, err := appleBuildInfo(ctx, iOS, simulator)
	if err != nil {
		return err
	}
	if platform == nil {
		platform = map[string]any{}
	}
	platform["CFBundleSupportedPlatforms"] = []any{iOS.platformName(simulator)}
	if simulator {
		platform["DTPlatformName"] = "iphonesimulator"
	}
	if err := os.WriteFile(filepath.Join(appex, "Info.plist"), encodePlist(shareInfo(p, platform)), 0o644); err != nil {
		return err
	}
	if _, err := writeAppleNames(p, appex); err != nil {
		return err
	}
	if err := p.writePrivacyManifest(appex); err != nil {
		return err
	}
	if !simulator {
		return nil
	}
	path, err := writeEntitlements(p.groupEntitlements())
	if err != nil {
		return err
	}
	defer os.Remove(path)
	return run(ctx, "codesign", "--force", "--sign", "-", "--entitlements", path, appex)
}

func signIOSShare(ctx context.Context, p *project, app, identity string) error {
	appex := iosSharePath(app)
	if _, err := os.Stat(appex); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if p.Sign.ShareProfile == "" {
		return fmt.Errorf("the share extension %s.share needs sign.share_profile, its own provisioning profile granting %s", p.ID, p.appGroup())
	}
	data, err := os.ReadFile(p.Sign.ShareProfile)
	if err != nil {
		return err
	}
	profile, err := readProfile(ctx, data)
	if err != nil {
		return err
	}
	entitlements, ok := profile["Entitlements"].(map[string]any)
	if !ok {
		return fmt.Errorf("sign.share_profile has no entitlements")
	}
	id := p.ID + ".share"
	appID, _ := entitlements["application-identifier"].(string)
	if !profileAllows(appID, id) {
		return fmt.Errorf("sign.share_profile is for %s, not %s", appID, id)
	}
	if groups, _ := entitlements["com.apple.security.application-groups"].([]any); !slices.Contains(groups, any(p.appGroup())) {
		return fmt.Errorf("sign.share_profile does not grant the app group %s", p.appGroup())
	}
	entitlements["application-identifier"] = signedAppID(appID, id)
	entitlements["com.apple.security.application-groups"] = []any{p.appGroup()}
	if err := os.WriteFile(filepath.Join(appex, "embedded.mobileprovision"), data, 0o644); err != nil {
		return err
	}
	ents, err := writeEntitlements(entitlements)
	if err != nil {
		return err
	}
	defer os.Remove(ents)
	return run(ctx, "codesign", "--force", "--sign", identity, "--entitlements", ents, "--generate-entitlement-der", appex)
}

// iosSigningExtension describes an already-built extension Xcode signs and embeds as a separate target when running an app on a physical device.
type iosSigningExtension struct{ Name, ID, Prefix, Script string }

func iosSigningExtensions(p *project, sys *appleOS) []iosSigningExtension {
	var out []iosSigningExtension
	if sys != iOS || p.clipParent != "" {
		return out
	}
	if p.iosExtension() {
		out = append(out, iosSigningExtension{"Widgets", p.ID + ".widgets", "AA", iosWidgetsScript})
	}
	if p.iosShares() {
		out = append(out, iosSigningExtension{shareExecutable, p.ID + ".share", "AB", strings.ReplaceAll(iosWidgetsScript, widgetsAppex, shareAppex)})
	}
	return out
}

// envValue returns the fields of the last name=value entry in env, as a compiler variable's command and flags.
func envValue(env []string, name string) []string {
	for i := len(env) - 1; i >= 0; i-- {
		if value, ok := strings.CutPrefix(env[i], name+"="); ok {
			return strings.Fields(value)
		}
	}
	return nil
}

// shareEntry is the entry point App Store Connect requires of an app extension's executable (error 90898).
const shareEntry = "_NSExtensionMain"

// shareLinkArgs are clang's arguments that link the Go archive into the extension's executable: the whole archive, as nothing else refers to it and its initializer starts Go's runtime, with NSExtensionMain as the entry point and only the APIs an app extension may use. The frameworks are what Go's runtime and net link (iosLinked); Go's runtime objects target a newer OS than the minimum, a warning -w silences.
func shareLinkArgs(flags []string, archive, out string) []string {
	args := append(slices.Clone(flags), "-fapplication-extension", "-Wl,-application_extension", "-Wl,-e,"+shareEntry,
		"-Wl,-force_load,"+archive, "-Wl,-dead_strip", "-Wl,-x", "-Wl,-w", "-lresolv", "-o", out)
	for _, f := range iosLinked {
		args = append(args, "-framework", f)
	}
	return args
}

// checkShareEntry fails unless the executable's entry point (LC_MAIN) is the stub of NSExtensionMain, so a build App Store Connect would reject stops here.
func checkShareEntry(path string) error {
	got, err := machoEntrySymbol(path)
	if err != nil {
		return fmt.Errorf("the share extension %s: %w", filepath.Base(path), err)
	}
	if got != shareEntry {
		return fmt.Errorf("the share extension's entry point is %s, not %s, which App Store Connect requires", got, shareEntry)
	}
	return nil
}

const (
	machoLoadMain     = 0x80000028 // LC_MAIN
	machoSymbolStubs  = 0x8        // S_SYMBOL_STUBS
	machoSection64Len = 80
	machoSegment64Len = 72
)

// machoEntrySymbol names what a Mach-O executable's LC_MAIN enters: an imported function's name when the entry is the stub that calls it, as an extension's is, or the address otherwise. It reads the stub sections' indirect symbol indices (reserved1 and reserved2), which debug/macho does not expose, from the segment commands.
func machoEntrySymbol(path string) (string, error) {
	f, err := macho.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	var entry uint64
	found := false
	for _, load := range f.Loads {
		raw := load.Raw()
		if len(raw) >= 16 && f.ByteOrder.Uint32(raw) == machoLoadMain {
			entry, found = f.ByteOrder.Uint64(raw[8:]), true
		}
	}
	if !found {
		return "", errors.New("no LC_MAIN entry point")
	}
	for _, load := range f.Loads {
		seg, ok := load.(*macho.Segment)
		if !ok || seg.Cmd != macho.LoadCmdSegment64 || entry < seg.Offset || entry >= seg.Offset+seg.Filesz {
			continue
		}
		addr := seg.Addr + entry - seg.Offset
		raw := seg.Raw()
		for i := range int(seg.Nsect) {
			s := raw[machoSegment64Len+i*machoSection64Len:]
			if len(s) < machoSection64Len {
				break
			}
			start, size := f.ByteOrder.Uint64(s[32:]), f.ByteOrder.Uint64(s[40:])
			flags, first, stub := f.ByteOrder.Uint32(s[64:]), f.ByteOrder.Uint32(s[68:]), f.ByteOrder.Uint32(s[72:])
			if addr < start || addr >= start+size {
				continue
			}
			if flags&0xff != machoSymbolStubs || stub == 0 || (addr-start)%uint64(stub) != 0 || f.Dysymtab == nil || f.Symtab == nil {
				break
			}
			index := uint64(first) + (addr-start)/uint64(stub)
			if index >= uint64(len(f.Dysymtab.IndirectSyms)) {
				break
			}
			sym := uint64(f.Dysymtab.IndirectSyms[index])
			if sym >= uint64(len(f.Symtab.Syms)) {
				break
			}
			return f.Symtab.Syms[sym].Name, nil
		}
		return fmt.Sprintf("%#x", addr), nil
	}
	return "", fmt.Errorf("its entry point %#x is in no segment", entry)
}
