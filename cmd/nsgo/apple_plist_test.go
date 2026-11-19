package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestAppleBuildInfoDeviceAndSimulator writes the SDK and Xcode an iOS device bundle is built with into its Info.plist, as App Store Connect requires (#889), leaves them out of a simulator's, and leaves a key apple.info sets to it.
func TestAppleBuildInfoDeviceAndSimulator(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Xcode fixtures are shell scripts")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "xcrun"), []byte(`#!/bin/sh
case "$3" in
--show-sdk-version) printf '18.5\n' ;;
--show-sdk-build-version) printf '22F76\n' ;;
*) echo "unexpected xcrun args: $*" >&2; exit 1 ;;
esac
`), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "xcodebuild"), []byte("#!/bin/sh\nprintf 'Xcode 26.0.1\\nBuild version 17A400\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := appleBuildInfo(t.Context(), iOS, false)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{
		"DTPlatformName": "iphoneos", "DTPlatformVersion": "18.5", "DTPlatformBuild": "22F76",
		"DTSDKName": "iphoneos18.5", "DTSDKBuild": "22F76", "DTXcode": "2601", "DTXcodeBuild": "17A400",
		"DTCompiler": "com.apple.compilers.llvm.clang.1_0",
	} {
		if got[key] != want {
			t.Errorf("%s = %#v, want %#v", key, got[key], want)
		}
	}
	if got := got["UIRequiredDeviceCapabilities"]; len(got.([]any)) != 1 || got.([]any)[0] != "arm64" {
		t.Errorf("device capabilities = %#v", got)
	}
	if got, err := appleBuildInfo(t.Context(), iOS, true); err != nil || got != nil {
		t.Errorf("simulator metadata = %#v, %v; want none", got, err)
	}

	p := &project{}
	p.Apple.Info = map[string]any{"DTPlatformName": "custom"}
	entries, err := appleBuildPlist(t.Context(), p, iOS, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(entries, "DTPlatformName") {
		t.Fatalf("apple.info value was not allowed to win: %s", entries)
	}
}
