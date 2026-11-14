//go:build linux && !android

package linux

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/updates"
)

// updateConfig is how the app Run runs updates. Linux has no updater of its own: a check reads the app's appcast, and package updater hands a verified package to the package manager that installed the app, or offers the release's page where none did (selfUpdatable).
var updateConfig updates.Config

func init() { driver.UpdateInstall = installUpdate }

func installUpdate(path string, done func(error)) {
	go func() {
		var cmd *exec.Cmd
		switch {
		case strings.HasSuffix(path, ".deb"):
			cmd = exec.Command("pkexec", "dpkg", "--install", path)
		case strings.HasSuffix(path, ".pkg.tar.gz"):
			cmd = exec.Command("pkexec", "pacman", "--upgrade", "--noconfirm", path)
		default:
			done(fmt.Errorf("updater: unsupported Linux package %q", filepath.Base(path)))
			return
		}
		err := cmd.Run()
		done(err)
	}()
}

// dpkgPattern escapes what dpkg-query --search reads as wildcards in a path.
var dpkgPattern = strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`)

// selfUpdatable reports whether download, the package an appcast offers, can replace the running app: the package manager of its format, dpkg for a Debian package and pacman for an Arch one, owns the app's executable. An AppImage, a package of the other format or a copy outside any package gets the release's page instead, as no package manager would replace it.
func selfUpdatable(ctx context.Context, download string) bool {
	u, err := url.Parse(download)
	if err != nil {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	var cmd *exec.Cmd
	switch name := path.Base(u.Path); {
	case strings.HasSuffix(name, ".deb"):
		cmd = exec.CommandContext(ctx, "dpkg-query", "--search", dpkgPattern.Replace(exe))
	case strings.HasSuffix(name, ".pkg.tar.gz"):
		cmd = exec.CommandContext(ctx, "pacman", "--query", "--owns", exe)
	default:
		return false
	}
	return cmd.Run() == nil
}

// InstalledBy names the sandbox whose store installed the app and updates
// it, Flatpak or Snap, as they mark their apps' environments; "" for any
// other install, a package manager's included, which nsgo.yaml's
// updates.channel tells apart.
func InstalledBy() string {
	if _, err := os.Stat("/.flatpak-info"); err == nil {
		return "Flatpak"
	}
	if os.Getenv("SNAP") != "" {
		return "Snap"
	}
	return ""
}

// checkForUpdates reads the appcast off the main thread and answers with
// what it found.
func (w *window) checkForUpdates(id uint32) {
	c := updateConfig
	updates.Answer(id, func(ctx context.Context) driver.Update {
		u := updates.Find(ctx, c)
		if u.Download != "" && !selfUpdatable(ctx, u.Download) {
			u.Download = ""
		}
		return u
	}, w.engine.Dispatch)
}
