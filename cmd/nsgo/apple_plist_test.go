package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppleBuildInfoDeviceAndSimulator(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(bin, "xcodebuild"), []byte("#!/bin/sh\nprintf 'Xcode 16.4\\nBuild version 16F6\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := appleBuildInfo(t.Context(), iOS, false)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]any{
		"DTPlatformName": "iphoneos", "DTPlatformVersion": "18.5", "DTPlatformBuild": "22F76",
		"DTSDKName": "iphoneos18.5", "DTSDKBuild": "22F76", "DTXcode": "1640", "DTXcodeBuild": "16F6",
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
