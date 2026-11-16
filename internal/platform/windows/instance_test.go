//go:build windows

package windows

import (
	"testing"
	"time"
)

// TestInstanceHandsOffByKey keeps the default instance and two keyed ones running together; a later launch must send its files to the owner of its own key, even while the other owners run.
func TestInstanceHandsOffByKey(t *testing.T) {
	id := "dev.nsgo.instance-test-" + t.TempDir()
	owners := map[string]chan []string{}
	for _, key := range []string{"", "one", "two"} {
		owner, already := claimInstance(id, key, nil)
		if already || owner == nil {
			t.Fatalf("key %q claim = (%v, %v), want an independent owner", key, owner, already)
		}
		t.Cleanup(owner.close)
		got := make(chan []string, 1)
		owners[key] = got
		owner.setDelivery(func(paths []string) { got <- paths })
	}
	for key, got := range owners {
		path := `C:\Documents\` + key + ".txt"
		other, already := claimInstance(id, key, []string{path})
		if !already || other != nil {
			other.close()
			t.Fatalf("same key %q claim = (%v, %v), want handoff", key, other, already)
		}
		select {
		case paths := <-got:
			if len(paths) != 1 || paths[0] != path {
				t.Fatalf("key %q received %v; want %q", key, paths, path)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("key %q did not receive its files", key)
		}
	}
}
