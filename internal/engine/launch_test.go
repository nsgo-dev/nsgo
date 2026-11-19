package engine

import (
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestLaunchSortsFilesFromLinks takes a desktop launch's arguments apart: files, also as the file URLs a desktop entry's %U passes, and links of any scheme but file's, leaving drive letters, flags and plain words to the app.
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
	os.Args = []string{"app", "-v", "--", doc, fileURL, "umapp://connect?endpoint=https%3A%2F%2Fexample.com&key=k", "Umapp:settings",
		"https://example.com/a", `C:\Users\a.txt`, "file://otherhost/a.txt", "words", dir}
	if got, want := LaunchFiles(), []string{doc, spaced}; !slices.Equal(got, want) {
		t.Errorf("files %q, want %q", got, want)
	}
	if got, want := LaunchLinks(), []string{"umapp://connect?endpoint=https%3A%2F%2Fexample.com&key=k", "Umapp:settings", "https://example.com/a"}; !slices.Equal(got, want) {
		t.Errorf("links %q, want %q", got, want)
	}
	if got := Launched(); !slices.Equal(got.Files, LaunchFiles()) || !slices.Equal(got.Links, LaunchLinks()) {
		t.Errorf("launched %+v", got)
	}
}
