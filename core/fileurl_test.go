package core

import (
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// localFileURL is path's file: URL as the backends write it, /C:/ for a Windows drive.
func localFileURL(path string) string {
	p := filepath.ToSlash(path)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

func TestFileFromURLLocalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "résumé #1 with spaces?.txt")
	if goruntime.GOOS == "windows" {
		path = filepath.Join(dir, "résumé #1 with spaces.txt")
	}
	const contents = "file from a drop"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatal(err)
	}
	raw := localFileURL(path)
	f, err := FileFromURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	if f.Path != path || f.Name != filepath.Base(path) || f.Size != int64(len(contents)) || f.MIME != "text/plain" || !f.Modified.Equal(when) || f.Temporary || f.Opener != nil {
		t.Fatalf("FileFromURL(%q) = %+v", raw, f)
	}
	r, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != contents {
		t.Fatalf("File.Open() = %q, want %q", got, contents)
	}
	if f, err := FileFromURL("file://localhost" + strings.TrimPrefix(raw, "file://")); err != nil || f.Path != path {
		t.Errorf("a localhost file URL = %+v, %v", f, err)
	}
	if _, err := FileFromURL(localFileURL(dir)); err == nil || !strings.Contains(err.Error(), "folder") {
		t.Errorf("a folder's URL = %v, want a folder error", err)
	}
	if _, err := FileFromURL(localFileURL(filepath.Join(dir, "missing.txt"))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing file's URL = %v, want not exist", err)
	}
}

func TestFileFromURLBlob(t *testing.T) {
	old := driver.BlobFile
	defer func() { driver.BlobFile = old }()
	driver.BlobFile = nil
	const raw = "blob:https://example.test/123#photo%20one%231.png"
	if _, err := FileFromURL(raw); err == nil {
		t.Error("a blob: URL off the web succeeded")
	}
	var gotURL, gotName string
	driver.BlobFile = func(u, name string) File {
		gotURL, gotName = u, name
		return File{Name: name, MIME: "image/png", Size: 10, Opener: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("blob bytes")), nil }}
	}
	f, err := FileFromURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "blob:https://example.test/123" || gotName != "photo one#1.png" {
		t.Fatalf("the web backend was asked for %q named %q", gotURL, gotName)
	}
	if f.Name != "photo one#1.png" || f.Path != "" || f.Size != 10 {
		t.Fatalf("blob FileFromURL() = %+v", f)
	}
	r, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "blob bytes" {
		t.Fatalf("blob File.Open() = %q", got)
	}
	for _, bad := range []string{"blob:https://example.test/123", "blob:https://example.test/123#", "blob:https://example.test/123#..", "blob:https://example.test/123#a%2Fb", "blob:https://example.test/123?q#a.png", "blob:/123#a.png"} {
		if _, err := FileFromURL(bad); err == nil {
			t.Errorf("FileFromURL(%q) succeeded", bad)
		}
	}
}

func TestFileFromURLRejectsOtherURLs(t *testing.T) {
	for _, raw := range []string{"", "https://example.test/a.txt", "data:text/plain,hello", "about:blank", "relative.txt", "/abs/path.txt", "file:relative.txt", "file:///tmp/a b.txt", "file:///tmp/a%20b.txt?query", "file:///tmp/a.txt?", "file:///tmp/a.txt#frag", "file://user@localhost/tmp/a.txt"} {
		if _, err := FileFromURL(raw); err == nil {
			t.Errorf("FileFromURL(%q) succeeded", raw)
		}
	}
}
