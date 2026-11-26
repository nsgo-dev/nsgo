//go:build android && nsgotest

package android

import (
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestNativeAndroidFileHandoffDenied(t *testing.T) {
	if nsgotest.Target() != "android" {
		t.Skip("requires the Android runtime and document provider")
	}
	var cx *core.Scope
	app := nsgotest.Start(t, func(scope *core.Scope) core.Node {
		cx = scope
		return core.View{}
	})
	for _, tt := range []struct {
		name string
		file core.SharedFile
		want string
	}{
		{"outside storage", core.SharedFile{Path: "/proc/self/status"}, "is not in the app's storage, which other apps are handed files from"},
		// The system DocumentsProvider requires a URI grant, and this app never requested one.
		{"without read grant", core.SharedFile{URI: "content://com.android.externalstorage.documents/document/primary%3ADownload", MIME: "application/pdf"}, "permission"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			type result struct {
				succeeded bool
				err       error
			}
			for _, share := range []bool{false, true} {
				answer := make(chan result, 1)
				app.Post(func() {
					done := func(ok bool, err error) { answer <- result{ok, err} }
					if share {
						cx.Share(core.ShareOptions{Files: []core.SharedFile{tt.file}}, done)
					} else {
						cx.OpenFile(tt.file, done)
					}
				})
				app.Settle()
				select {
				case got := <-answer:
					if got.succeeded || got.err == nil || !strings.Contains(strings.ToLower(got.err.Error()), tt.want) {
						t.Fatalf("share=%v: got %v, %v; want error containing %q", share, got.succeeded, got.err, tt.want)
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("share=%v: no file service reply", share)
				}
			}
		})
	}
}
