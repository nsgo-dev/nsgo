package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// In-app purchases (core's Purchase and the services beside it) go through StoreKit 2, whose products and transactions have no Objective-C interface. When nsgo.yaml sets apple.purchases, the tool compiles storekit/StoreKit.swift with Xcode's swiftc into a library among the bundle's frameworks, a framework on iOS and tvOS, as the App Store takes no other, and a plain library on macOS, which internal/platform/apple loads through purego. Apps that do not set it link none of it. Selling in the App Store needs no entitlement: In-App Purchase is on for every App ID.

//go:embed storekit/StoreKit.swift
var storeKitSource []byte

const (
	storeKit = "NSGoStoreKit"
	// storeKitDylib is the library on macOS.
	storeKitDylib = "libNSGoStoreKit.dylib"
	// storeKitMacMinimum is the oldest macOS with StoreKit 2; on macOS 11, which apps still run on, the library does not load and the app has no purchases.
	storeKitMacMinimum = "12.0"
)

// compileStoreKit compiles the library for sdk and triple, with installName its install name, kept in the user's cache by the source, the target and the toolchain, as the widgets are, so builds compile it once.
func compileStoreKit(ctx context.Context, sdkName, triple, installName string) (string, error) {
	version, err := output(ctx, "xcrun", "--sdk", sdkName, "swiftc", "--version")
	if err != nil {
		return "", fmt.Errorf("in-app purchases need Xcode's swiftc: %w", err)
	}
	sdk, err := output(ctx, "xcrun", "--sdk", sdkName, "--show-sdk-path")
	if err != nil {
		return "", err
	}
	sum := sha256.New()
	for _, part := range []string{version, sdk, triple, installName, string(storeKitSource)} {
		fmt.Fprintf(sum, "%d:%s", len(part), part)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "nsgo", "storekit", hex.EncodeToString(sum.Sum(nil)[:16]))
	lib := filepath.Join(dir, storeKit)
	if _, err := os.Stat(lib); err == nil {
		return lib, nil
	}
	stage(ctx, "compiling StoreKit's library")
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), "tmp-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	src := filepath.Join(tmp, "StoreKit.swift")
	if err := os.WriteFile(src, storeKitSource, 0o644); err != nil {
		return "", err
	}
	if err := os.Mkdir(filepath.Join(tmp, "out"), 0o755); err != nil {
		return "", err
	}
	if err := run(ctx, "xcrun", "--sdk", sdkName, "swiftc", "-target", triple, "-sdk", sdk, "-O", "-swift-version", "5",
		"-parse-as-library", "-module-name", storeKit, "-emit-library", "-Xlinker", "-install_name", "-Xlinker", installName,
		"-o", filepath.Join(tmp, "out", storeKit), src); err != nil {
		return "", fmt.Errorf("compiling StoreKit's library: %w", err)
	}
	if err := os.Rename(filepath.Join(tmp, "out"), dir); err != nil {
		if _, statErr := os.Stat(lib); statErr != nil {
			return "", err
		}
	}
	return lib, nil
}

// macStoreKitTriple is the target the macOS library is compiled for on arch.
func macStoreKitTriple(arch string) string {
	if arch == "amd64" {
		arch = "x86_64"
	}
	return arch + "-apple-macos" + storeKitMacMinimum
}

// addMacStoreKit puts the library into a macOS bundle built for arch when the app sells in-app purchases, signed ad hoc as the app is; signMacApp signs it again with an identity, as the bundle's other libraries.
func addMacStoreKit(ctx context.Context, p *project, app, arch string) error {
	if !p.Apple.Purchases {
		return nil
	}
	lib, err := compileStoreKit(ctx, "macosx", macStoreKitTriple(arch), "@rpath/"+storeKitDylib)
	if err != nil {
		return err
	}
	dylib := filepath.Join(app, "Contents", "Frameworks", storeKitDylib)
	if err := copyLibrary(lib, dylib); err != nil {
		return err
	}
	return run(ctx, "codesign", "--force", "--sign", "-", dylib)
}

// addIOSStoreKit puts the library into an iOS or tvOS bundle as a framework when the app sells in-app purchases: signed ad hoc for the simulator, left for the app's signing on devices, as the bundle's other frameworks (signFrameworks). watchOS's apps, whose host is linked into the executable (watchos.go), take none.
func addIOSStoreKit(ctx context.Context, p *project, sys *appleOS, app string, simulator bool) error {
	if !p.Apple.Purchases {
		return nil
	}
	lib, err := compileStoreKit(ctx, sys.sdkName(simulator), sys.triple(simulator), "@rpath/"+storeKit+".framework/"+storeKit)
	if err != nil {
		return err
	}
	fw := filepath.Join(app, "Frameworks", storeKit+".framework")
	if err := copyLibrary(lib, filepath.Join(fw, storeKit)); err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(fw, "Info.plist"), encodePlist(map[string]any{
		"CFBundleIdentifier":            p.ID + ".storekit",
		"CFBundleExecutable":            storeKit,
		"CFBundleName":                  storeKit,
		"CFBundlePackageType":           "FMWK",
		"CFBundleVersion":               p.BundleVersion(),
		"CFBundleShortVersionString":    p.Version,
		"CFBundleInfoDictionaryVersion": "6.0",
		"CFBundleSupportedPlatforms":    []any{sys.platformName(simulator)},
		"DTPlatformName":                sys.sdkName(simulator),
		"MinimumOSVersion":              sys.minimum,
	}), 0o644)
	if err != nil || !simulator {
		return err
	}
	return run(ctx, "codesign", "--force", "--sign", "-", fw)
}
