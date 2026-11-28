package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	projectcfg "github.com/nsgo-dev/nsgo/project"
)

const (
	quickLookAppex          = "QuickLookPreview.appex"
	quickLookExec           = "QuickLookPreview"
	quickLookPrincipalClass = "NSGoQuickLookController"

	quickLookThumbnailAppex          = "QuickLookThumbnail.appex"
	quickLookThumbnailExec           = "QuickLookThumbnail"
	quickLookThumbnailPrincipalClass = "NSGoQuickLookThumbnailProvider"
)

// quickLookExtension is one of a macOS app's QuickLook extensions: its
// preview (quicklook) or its thumbnails (quicklook_thumbnail).
type quickLookExtension struct {
	*projectcfg.QuickLook
	// key is its nsgo.yaml section, profileKey its sign key.
	key, profileKey string
	exec, principal string
	// point is its NSExtensionPointIdentifier.
	point   string
	profile string
	preview bool
}

// quickLookExtensions are the QuickLook extensions p declares.
func (p *project) quickLookExtensions() []quickLookExtension {
	var out []quickLookExtension
	if p.QuickLook != nil {
		out = append(out, quickLookExtension{
			QuickLook: p.QuickLook, key: "quicklook", profileKey: "quicklook_profile",
			exec: quickLookExec, principal: quickLookPrincipalClass,
			point: "com.apple.quicklook.preview", profile: p.Sign.QuickLookProfile, preview: true,
		})
	}
	if p.QuickLookThumbnail != nil {
		out = append(out, quickLookExtension{
			QuickLook: p.QuickLookThumbnail, key: "quicklook_thumbnail", profileKey: "quicklook_thumbnail_profile",
			exec: quickLookThumbnailExec, principal: quickLookThumbnailPrincipalClass,
			point: "com.apple.quicklook.thumbnail", profile: p.Sign.QuickLookThumbnailProfile,
		})
	}
	return out
}

// appex is the extension's bundle in the macOS bundle app.
func (ext quickLookExtension) appex(app string) string {
	return filepath.Join(app, "Contents", "PlugIns", ext.exec+".appex")
}

// macQuickLookAppex is the QuickLook Preview Extension of a macOS bundle.
func macQuickLookAppex(app string) string {
	return filepath.Join(app, "Contents", "PlugIns", quickLookAppex)
}

// macQuickLookThumbnailAppex is the QuickLook Thumbnail Extension of a macOS bundle.
func macQuickLookThumbnailAppex(app string) string {
	return filepath.Join(app, "Contents", "PlugIns", quickLookThumbnailAppex)
}

// quickLookInfo returns Contents/Info.plist for the extension's .appex.
func (ext quickLookExtension) info(p *project, minimum string) []byte {
	types := make([]any, len(ext.Types))
	for i, uti := range ext.Types {
		types[i] = uti
	}
	attrs := map[string]any{"QLSupportedContentTypes": types}
	if ext.preview {
		attrs["QLIsDataBasedPreview"] = false
	} else {
		// Draw thumbnails at every size Finder asks for.
		attrs["QLThumbnailMinimumDimension"] = int64(0)
	}
	return encodePlist(map[string]any{
		"CFBundleName":                  p.Name,
		"CFBundleDisplayName":           p.Name,
		"CFBundleIdentifier":            ext.ID,
		"CFBundleVersion":               p.BundleVersion(),
		"CFBundleShortVersionString":    p.Version,
		"CFBundleExecutable":            ext.exec,
		"CFBundlePackageType":           "XPC!",
		"CFBundleInfoDictionaryVersion": "6.0",
		"CFBundleSupportedPlatforms":    []any{"MacOSX"},
		"LSMinimumSystemVersion":        minimum,
		"NSExtension": map[string]any{
			"NSExtensionPointIdentifier": ext.point,
			"NSExtensionPrincipalClass":  ext.principal,
			"NSExtensionAttributes":      attrs,
		},
	})
}

// addMacQuickLook builds and embeds the app's macOS QuickLook extensions
// (Contents/PlugIns/QuickLookPreview.appex and QuickLookThumbnail.appex)
// for arch, signed ad hoc with the mandatory App Sandbox entitlement.
// signMacApp and signMacStoreApp re-sign them with the release identity or
// Mac App Store profiles before signing the host app.
func addMacQuickLook(ctx context.Context, p *project, app, arch string) error {
	exts := p.quickLookExtensions()
	if len(exts) == 0 {
		return nil
	}
	minimum, err := p.appleMinimum(nil)
	if err != nil {
		return err
	}
	for _, ext := range exts {
		if err := ext.add(ctx, p, app, arch, minimum); err != nil {
			return err
		}
	}
	return nil
}

func (ext quickLookExtension) add(ctx context.Context, p *project, app, arch, minimum string) error {
	if ext.preview {
		stage(ctx, "building the QuickLook extension")
	} else {
		stage(ctx, "building the QuickLook thumbnail extension")
	}
	appex := ext.appex(app)
	contents := filepath.Join(appex, "Contents")
	bin := filepath.Join(contents, "MacOS", ext.exec)
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		return err
	}
	if err := p.buildQuickLook(ctx, ext, arch, minimum, bin); err != nil {
		return err
	}
	// go build -o writes a package archive for a package that is not main.
	if data, err := os.ReadFile(bin); err != nil || bytes.HasPrefix(data, []byte("!<arch>\n")) {
		return fmt.Errorf("%s: %s built no executable; is it a main package?", ext.key, ext.Main)
	}
	resources := filepath.Join(contents, "Resources")
	if err := copyResources(p, resources); err != nil {
		return err
	}
	if err := os.MkdirAll(resources, 0o755); err != nil {
		return err
	}
	if err := p.writePrivacyManifest(resources); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), ext.info(p, minimum), 0o644); err != nil {
		return err
	}
	return signMacQuickLook(ctx, p, appex, "-")
}

func (p *project) buildQuickLook(ctx context.Context, ext quickLookExtension, arch, minimum, out string) error {
	env := []string{"GOOS=darwin", "GOARCH=" + arch, "CGO_ENABLED=0"}
	tags, err := goTags(ctx, p, env)
	if err != nil {
		return err
	}
	args := append([]string{"build"}, tagsArgs(tags)...)
	var ldflags []string
	if p.Release {
		args = append(args, "-trimpath", "-buildvcs=false")
		ldflags = append(ldflags, "-s", "-w")
	}
	ldflags = append(ldflags, "-macos="+minimum)
	allFlags := append([]string{}, p.Build.LDFlags...)
	allFlags = append(allFlags, p.identityLdflags())
	if p.Release {
		allFlags = append(allFlags, "-X "+corePath+".appRelease=1")
	}
	if p.Appearance != "" {
		allFlags = append(allFlags, "-X "+nsgoModule+".appearance="+p.Appearance)
	}
	allFlags = append(allFlags, ldflags...)
	args = append(args, "-ldflags", strings.Join(allFlags, " "))
	if err := goTool(ctx, p.Dir, env, append(args, "-o", out, "--", ext.Main)...); err != nil {
		return fmt.Errorf("%s: %s: %w", ext.key, ext.Main, err)
	}
	return nil
}

// signMacQuickLooks signs a macOS bundle's QuickLook extensions with
// identity, before the host app.
func signMacQuickLooks(ctx context.Context, p *project, app, identity string) error {
	for _, ext := range p.quickLookExtensions() {
		if err := signMacQuickLook(ctx, p, ext.appex(app), identity); err != nil {
			return err
		}
	}
	return nil
}

// signMacQuickLook signs a macOS bundle's QuickLook extension with the
// mandatory App Sandbox entitlement. It signs before the host app so the app's
// signature seals it without overwriting its entitlements.
func signMacQuickLook(ctx context.Context, p *project, appex, identity string) error {
	ents, err := writeEntitlements(map[string]any{
		"com.apple.security.app-sandbox": true,
	})
	if err != nil {
		return err
	}
	defer os.Remove(ents)
	args := []string{"--force", "--sign", identity, "--entitlements", ents}
	if identity != "-" && p.notarizing() {
		args = append(args, "--options", "runtime", "--timestamp")
	}
	return run(ctx, "codesign", append(args, appex)...)
}

// signMacStoreQuickLook provisions and signs a QuickLook extension for the
// Mac App Store using its sign profile from the same team as the app.
func signMacStoreQuickLook(ctx context.Context, ext quickLookExtension, app, team string) error {
	data, err := os.ReadFile(ext.profile)
	if err != nil {
		return err
	}
	profile, err := readProfile(ctx, data)
	if err != nil {
		return fmt.Errorf("sign.%s: %w", ext.profileKey, err)
	}
	granted, _ := profile["Entitlements"].(map[string]any)
	id := ext.ID
	appID, _ := granted["com.apple.application-identifier"].(string)
	if !profileAllows(appID, id) {
		return fmt.Errorf("sign.%s is for %s, not %s", ext.profileKey, appID, id)
	}
	extTeam, _ := granted["com.apple.developer.team-identifier"].(string)
	if extTeam == "" || extTeam != team {
		return fmt.Errorf("sign.%s must belong to the app profile's team %s", ext.profileKey, team)
	}
	identity, err := profileIdentity(ctx, profile)
	if err != nil {
		return err
	}
	ents, err := writeEntitlements(map[string]any{
		"com.apple.security.app-sandbox":      true,
		"com.apple.application-identifier":    signedAppID(appID, id),
		"com.apple.developer.team-identifier": team,
	})
	if err != nil {
		return err
	}
	defer os.Remove(ents)
	appex := ext.appex(app)
	if err := os.WriteFile(filepath.Join(appex, "Contents", "embedded.provisionprofile"), data, 0o644); err != nil {
		return err
	}
	return run(ctx, "codesign", "--force", "--sign", identity, "--entitlements", ents, "--generate-entitlement-der", appex)
}
