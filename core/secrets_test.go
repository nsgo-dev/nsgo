package core_test

import (
	"errors"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// secretsOnly allows the secure store alone.
type secretsOnly struct{}

func (secretsOnly) Allows(req core.AccessRequest) bool { return req.Permission == core.AccessSecrets }
func (secretsOnly) WebHosts() []string                 { return nil }

func TestSecretsKeepReadAndDelete(t *testing.T) {
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	}, nsgotest.Secrets(map[string]string{"seeded": "from an earlier run"}))
	s := scope.Secrets()
	if v, ok, err := s.Get("seeded"); v != "from an earlier run" || !ok || err != nil {
		t.Fatalf("seeded: %q %v %v", v, ok, err)
	}
	if v, ok, err := s.Get("refresh-token"); v != "" || ok || err != nil {
		t.Fatalf("missing: %q %v %v", v, ok, err)
	}
	if err := s.Set("refresh-token", "one"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("refresh-token", "two"); err != nil {
		t.Fatal(err)
	}
	if v, ok, _ := s.Get("refresh-token"); v != "two" || !ok {
		t.Fatalf("after a second write: %q %v", v, ok)
	}
	// The test's store holds it, under the app's own service: the machine's keychain was never asked.
	item := driver.SecretItem{Service: s.Service, Account: "refresh-token"}
	if got := app.Secrets.Items()[item]; got != "two" {
		t.Fatalf("the test's store holds %v", app.Secrets.Items())
	}
	if s.Service == "" {
		t.Fatal("no default service")
	}
	if err := s.Delete("refresh-token"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("refresh-token"); err != nil {
		t.Fatalf("deleting again: %v", err)
	}
	if _, ok, _ := s.Get("refresh-token"); ok {
		t.Fatal("deleted, it still reads")
	}
	if err := s.Set("", "v"); err == nil {
		t.Fatal("a secret without an account")
	}
}

func TestSecretsNameTheirItems(t *testing.T) {
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	})
	// An app reading what an earlier build wrote names its service, group and accessibility.
	s := scope.Secrets()
	s.Service, s.AccessGroup, s.Accessible = "app:no-auth", "TEAM.group", core.SecretAfterFirstUnlockThisDeviceOnly
	if err := s.Set("token", "v"); err != nil {
		t.Fatal(err)
	}
	want := driver.SecretItem{Service: "app:no-auth", Account: "token", AccessGroup: "TEAM.group", Accessible: core.SecretAfterFirstUnlockThisDeviceOnly}
	if got := app.Secrets.Items(); len(got) != 1 || got[want] != "v" {
		t.Fatalf("items %v", got)
	}
	if _, ok, _ := scope.Secrets().Get("token"); ok {
		t.Fatal("the app's own service read another's item")
	}
}

// failingSecrets is a platform without a secure store.
type failingSecrets struct{}

func (failingSecrets) ReadSecret(driver.SecretItem) (string, bool, error) {
	return "", false, driver.ErrNoSecretStore
}
func (failingSecrets) WriteSecret(driver.SecretItem, string) error { return driver.ErrNoSecretStore }
func (failingSecrets) DeleteSecret(driver.SecretItem) error        { return driver.ErrNoSecretStore }

func TestSecretsUnavailable(t *testing.T) {
	for name, store := range map[string]driver.SecretStore{"no store": nil, "a store reporting none": failingSecrets{}} {
		cx := core.NewRoot(core.RootOptions{Secrets: store})
		s := cx.Secrets()
		_, _, err := s.Get("a")
		if !errors.Is(err, core.ErrSecretsUnavailable) || core.ErrorCode(err) != core.CodeUnavailable {
			t.Errorf("%s: Get: %v", name, err)
		}
		if err := s.Set("a", "v"); !errors.Is(err, core.ErrSecretsUnavailable) {
			t.Errorf("%s: Set: %v", name, err)
		}
		if err := s.Delete("a"); !errors.Is(err, core.ErrSecretsUnavailable) {
			t.Errorf("%s: Delete: %v", name, err)
		}
		cx.Dispose()
	}
}

func TestSecretsHeldToAccessAndPartitioned(t *testing.T) {
	store := &driver.MemorySecrets{}
	root := core.NewRoot(core.RootOptions{Secrets: store})
	defer root.Dispose()
	app := root.Secrets()
	if err := app.Set("token", "the app's"); err != nil {
		t.Fatal(err)
	}

	denied := root.Child()
	denied.ProvideAccess(denyAll{})
	if err := denied.Secrets().Set("token", "x"); core.ErrorCode(err) != core.CodeAccessDenied {
		t.Fatalf("refused code wrote: %v", err)
	}
	if _, _, err := denied.Secrets().Get("token"); core.ErrorCode(err) != core.CodeAccessDenied {
		t.Fatalf("refused code read: %v", err)
	}

	one, two := root.Child(), root.Child()
	for name, cx := range map[string]*core.Scope{"one": one, "two": two} {
		cx.ProvideAccess(secretsOnly{})
		cx.ProvideSecrets(name)
	}
	s1, s2 := one.Secrets(), two.Secrets()
	if s1.Service == app.Service || s1.Service == s2.Service {
		t.Fatalf("partition services %q %q, the app's %q", s1.Service, s2.Service, app.Service)
	}
	if _, ok, err := s1.Get("token"); ok || err != nil {
		t.Fatalf("a partition read the app's secret: %v %v", ok, err)
	}
	if err := s1.Set("token", "one's"); err != nil {
		t.Fatal(err)
	}
	if v, _, _ := s2.Get("token"); v != "" {
		t.Fatalf("two read %q", v)
	}
	if v, _, _ := app.Get("token"); v != "the app's" {
		t.Fatalf("the app's secret is %q", v)
	}
	// A partition cannot reach out of itself.
	out := one.Secrets()
	out.Service = app.Service
	if _, _, err := out.Get("token"); core.ErrorCode(err) != core.CodeAccessDenied {
		t.Fatalf("a partition read another service: %v", err)
	}
	out = one.Secrets()
	out.AccessGroup = "TEAM.shared"
	if err := out.Set("token", "x"); core.ErrorCode(err) != core.CodeAccessDenied {
		t.Fatalf("a partition named an access group: %v", err)
	}
	if n := len(store.Items()); n != 2 {
		t.Fatalf("the store holds %v", store.Items())
	}
}
