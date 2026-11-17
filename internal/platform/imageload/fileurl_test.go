package imageload

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestLoadFileURL reads a local file through the file: URL a backend writes for it, /C:/ for a Windows drive.
func TestLoadFileURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "poster 日本 #%.png")
	const want = "local image bytes"
	if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	slash := filepath.ToSlash(path)
	if !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	for _, u := range []url.URL{{Scheme: "file", Path: slash}, {Scheme: "file", Host: "localhost", Path: slash}} {
		got, err := Load(context.Background(), driver.ImageSource{URI: u.String()}, nil)
		if err != nil || string(got) != want {
			t.Errorf("Load(%q) = %q, %v; want %q", u.String(), got, err, want)
		}
	}
}

// TestLoadRemoteFileURL holds a file: URL with a host other than localhost off the local disk outside Windows, where it would otherwise read the host-less path on this machine.
func TestLoadRemoteFileURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a host names a UNC share on Windows")
	}
	path := filepath.Join(t.TempDir(), "a.png")
	if err := os.WriteFile(path, []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	u := url.URL{Scheme: "file", Host: "server", Path: path}
	if got, err := Load(context.Background(), driver.ImageSource{URI: u.String()}, nil); err == nil {
		t.Errorf("Load(%q) = %q, want an error", u.String(), got)
	}
}
