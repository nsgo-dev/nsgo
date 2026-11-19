package engine

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

// TestLaunchSortsFilesFromLinks takes a desktop launch's arguments apart: files, also as the file URLs a desktop entry's %U passes, and links of any scheme but file's, leaving drive letters, flags and plain words to the app. On Windows a launch with a link opens no files.
func TestLaunchSortsFilesFromLinks(t *testing.T) {
	dir := t.TempDir()
	doc, spaced := filepath.Join(dir, "notes.txt"), filepath.Join(dir, "日本 語.flow")
	for _, path := range []string{doc, spaced} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(spaced)}).String()
	if filepath.VolumeName(spaced) != "" {
		fileURL = (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(spaced)}).String()
	}
	saved := os.Args
	t.Cleanup(func() { os.Args = saved })
	links := []string{"umapp://connect?endpoint=https%3A%2F%2Fexample.com&key=k", "Umapp:settings", "https://example.com/a"}
	os.Args = []string{"app", "-v", "--", doc, fileURL, links[0], links[1], links[2], `C:\Users\a.txt`, "file://otherhost/a.txt", "words", dir}
	wantFiles := []string{doc, spaced}
	if runtime.GOOS == "windows" {
		// A link may have closed the quotes Windows put it in and named the files after it.
		wantFiles = nil
	}
	if got := LaunchFiles(); !slices.Equal(got, wantFiles) {
		t.Errorf("files %q, want %q", got, wantFiles)
	}
	if got := LaunchLinks(); !slices.Equal(got, links) {
		t.Errorf("links %q, want %q", got, links)
	}
	if got := Launched(); !slices.Equal(got.Files, wantFiles) || !slices.Equal(got.Links, links) {
		t.Errorf("launched %+v", got)
	}
	os.Args = []string{"app", "-v", "--", doc, fileURL, `C:\Users\a.txt`, "file://otherhost/a.txt", "words", dir}
	if got, want := LaunchFiles(), []string{doc, spaced}; !slices.Equal(got, want) {
		t.Errorf("files without links %q, want %q", got, want)
	}
}
