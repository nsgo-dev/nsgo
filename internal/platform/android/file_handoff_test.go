//go:build android || nsgoandroidhost

package android

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestAndroidFileHandoffWire(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("NSGO_CACHE_DIR", cache)
	uri := driver.SharedFile{URI: "content://10@provider/document/report%2F2026.pdf?token=%2f", Name: "report.pdf"}
	path := driver.SharedFile{Path: filepath.Join(cache, "content", "notes.txt"), Name: "notes.txt", MIME: "text/plain"}
	data := driver.SharedFile{Data: []byte("contents"), Name: "bytes.txt", MIME: "text/plain"}
	uriOnly := &driver.MountBatch{Items: []driver.MountItem{
		{Kind: driver.ItemShare, Share: &driver.ShareRequest{Files: []driver.SharedFile{uri}}},
	}}
	var s surface
	s.stage(0, uriOnly)
	if _, err := os.Stat(filepath.Join(cache, "nsgo-share")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("URI-only share changed staging storage: stat error = %v", err)
	}

	share := driver.ShareRequest{Text: "attachments", Files: []driver.SharedFile{path, uri, data}}
	batch := &driver.MountBatch{Root: 1, Items: []driver.MountItem{
		{Kind: driver.ItemOpenFile, Count: 41, Open: &path},
		{Kind: driver.ItemOpenFile, Count: 42, Open: &uri},
		{Kind: driver.ItemShare, Count: 43, Share: &share},
	}}
	s.stage(0, batch)
	staged := batch.Items[2].Share.Files[2].Path
	if bytes, err := os.ReadFile(staged); err != nil || string(bytes) != "contents" || share.Files[2].Path != "" {
		t.Fatalf("staged bytes = %q, %v; original path = %q", bytes, err, share.Files[2].Path)
	}

	var w writer
	encode(&w, batch, false, nil, nil, nil, nil)
	r := reader{w.buf}
	if root, anim, first, items := r.u32(), r.byte(), r.byte(), r.u32(); root != 1 || anim != 0 || first != 0 || items != 3 {
		t.Fatalf("batch header = %d, %d, %d, %d", root, anim, first, items)
	}
	item := func(wantKind driver.ItemKind, wantCount uint32) {
		t.Helper()
		if kind, id, count := driver.ItemKind(r.byte()), r.u32(), r.u32(); kind != wantKind || id != 0 || count != wantCount {
			t.Fatalf("item header = %v, %d, %d; want %v, 0, %d", kind, id, count, wantKind, wantCount)
		}
	}
	file := func(source string, f driver.SharedFile) {
		t.Helper()
		if gotSource, name, mime := r.str(), r.str(), r.str(); gotSource != source || name != f.Name || mime != f.MIME {
			t.Fatalf("file = (%q, %q, %q); want (%q, %q, %q)", gotSource, name, mime, source, f.Name, f.MIME)
		}
	}
	item(driver.ItemOpenFile, 41)
	file(path.Path, path)
	item(driver.ItemOpenFile, 42)
	file("\x00"+uri.URI, uri)
	item(driver.ItemShare, 43)
	if text, files := r.str(), r.u32(); text != share.Text || files != 3 {
		t.Fatalf("share header = %q, %d", text, files)
	}
	file(path.Path, path)
	file("\x00"+uri.URI, uri)
	file(staged, data)
	if len(r.buf) != 0 {
		t.Fatalf("%d trailing bytes in encoded batch", len(r.buf))
	}
}
