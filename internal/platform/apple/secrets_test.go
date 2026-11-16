//go:build darwin

package apple

import (
	"os"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// TestKeychainKeepsReadsAndDeletes writes, reads, replaces and deletes a generic password of the machine's keychain. It touches the user's keychain, so it runs only when NSGO_TEST_KEYCHAIN is set, in a session whose login keychain is unlocked.
func TestKeychainKeepsReadsAndDeletes(t *testing.T) {
	if os.Getenv("NSGO_TEST_KEYCHAIN") == "" {
		t.Skip("set NSGO_TEST_KEYCHAIN to write a test item to this machine's keychain")
	}
	var k Keychain
	item := driver.SecretItem{Service: "dev.nsgo.test.secrets", Account: "keychain-test"}
	t.Cleanup(func() { k.DeleteSecret(item) })
	if err := k.DeleteSecret(item); err != nil {
		t.Fatalf("deleting what is not there: %v", err)
	}
	if v, ok, err := k.ReadSecret(item); ok || err != nil {
		t.Fatalf("before writing: %q %v %v", v, ok, err)
	}
	for _, value := range []string{"first", "second\x00with a zero byte"} {
		if err := k.WriteSecret(item, value); err != nil {
			t.Fatal(err)
		}
		if v, ok, err := k.ReadSecret(item); v != value || !ok || err != nil {
			t.Fatalf("after writing %q: %q %v %v", value, v, ok, err)
		}
	}
	if err := k.DeleteSecret(item); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := k.ReadSecret(item); ok || err != nil {
		t.Fatalf("after deleting: %v %v", ok, err)
	}
}
