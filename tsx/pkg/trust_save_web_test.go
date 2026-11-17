//go:build js

package pkg

import (
	"fmt"
	"syscall/js"
	"testing"
)

func trustTestDir(t *testing.T) string {
	t.Helper()
	// A browser Trust names a localStorage key, not a filesystem path.
	dir := fmt.Sprintf("%s/%s", t.Name(), js.Global().Get("crypto").Call("randomUUID").String())
	t.Cleanup(func() {
		js.Global().Get("localStorage").Call("removeItem", stateKey(dir))
	})
	return dir
}

// blockTrustWrites makes the browser refuse only this Trust's writes.
func blockTrustWrites(t *testing.T, dir string) func() {
	t.Helper()
	prototype := js.Global().Get("Storage").Get("prototype")
	original := prototype.Get("setItem")
	factory := js.Global().Get("Function").New("original", "key", `
		return function(k, value) {
			if (k === key) throw new Error("test: storage unavailable");
			return original.call(this, k, value);
		};
	`)
	prototype.Set("setItem", factory.Invoke(original, stateKey(dir)))
	blocked := true
	unblock := func() {
		if blocked {
			prototype.Set("setItem", original)
			blocked = false
		}
	}
	t.Cleanup(unblock)
	return unblock
}
