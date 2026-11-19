//go:build windows

package windows

import (
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/internal/engine"
)

// TestInstanceHandsOffByKey keeps the default instance and two keyed ones running together; a later launch must send its files and links to the owner of its own key, even while the other owners run.
func TestInstanceHandsOffByKey(t *testing.T) {
	id := "dev.nsgo.instance-test-" + t.TempDir()
	owners := map[string]chan []string{}
	for _, key := range []string{"", "one", "two"} {
		owner, already := claimInstance(id, key, launch{})
		if already || owner == nil {
			t.Fatalf("key %q claim = (%v, %v), want an independent owner", key, owner, already)
		}
		t.Cleanup(owner.close)
		got := make(chan []string, 1)
		owners[key] = got
		owner.setDelivery(func(l launch) { got <- append(l.Files, l.Links...) })
	}
	for key, got := range owners {
		path, link := `C:\Documents\`+key+".txt", "umapp://open/"+key
		other, already := claimInstance(id, key, launch{Launch: engine.Launch{Files: []string{path}, Links: []string{link}}})
		if !already || other != nil {
			other.close()
			t.Fatalf("same key %q claim = (%v, %v), want handoff", key, other, already)
		}
		select {
		case paths := <-got:
			if len(paths) != 2 || paths[0] != path || paths[1] != link {
				t.Fatalf("key %q received %v; want %q and %q", key, paths, path, link)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("key %q did not receive its files", key)
		}
	}
}
