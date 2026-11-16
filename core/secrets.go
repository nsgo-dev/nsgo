package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// ErrSecretsUnavailable is what Secrets fails with where the platform has no secure store, such as the web, or a Linux session without a Secret Service: NSGo never keeps a secret in plain storage instead.
var ErrSecretsUnavailable = &Error{Code: CodeUnavailable, Message: "core: the platform has no secure store for secrets", Err: driver.ErrNoSecretStore}

// SecretItem names a secret in the platform's secure store, SecretStore is where a platform keeps secrets (RootOptions.Secrets), and MemorySecrets is one in memory, which tests keep theirs in (nsgotest.App.Secrets).
type (
	SecretItem    = driver.SecretItem
	SecretStore   = driver.SecretStore
	MemorySecrets = driver.MemorySecrets
)

// SecretAccessible says when Apple platforms let the app read a secret, Apple's kSecAttrAccessible; the zero value leaves it to the system, which reads it only while the device is unlocked. Other platforms ignore it.
type SecretAccessible = driver.SecretAccessible

const (
	SecretAccessibleDefault              = driver.SecretAccessibleDefault
	SecretWhenUnlocked                   = driver.SecretWhenUnlocked
	SecretAfterFirstUnlock               = driver.SecretAfterFirstUnlock
	SecretWhenUnlockedThisDeviceOnly     = driver.SecretWhenUnlockedThisDeviceOnly
	SecretAfterFirstUnlockThisDeviceOnly = driver.SecretAfterFirstUnlockThisDeviceOnly
	SecretWhenPasscodeSetThisDeviceOnly  = driver.SecretWhenPasscodeSetThisDeviceOnly
)

// Secrets is the app's secure store: strings by account under a service, which the platform protects, unlike Storage's plain file. It keeps them as generic passwords of the keychain on macOS, iOS, tvOS and watchOS, generic credentials of the Credential Manager on Windows, items of the Secret Service on Linux and values encrypted with a key of the Android Keystore in the app's private files on Android; elsewhere, as on the web, every call fails with ErrSecretsUnavailable.
//
// Service, AccessGroup and Accessible name and protect the items, so an app keeps reading what an earlier build, or another framework, wrote with the same ones. Its methods may run on any goroutine, and as the system may make them wait, as a keychain asking the user to allow a read does, an app may call them off the engine's.
type Secrets struct {
	// Service names the items with the account: kSecAttrService on Apple platforms, the first part of the generic credential's target "Service/Account" on Windows, the service attribute on Linux. Scope.Secrets sets it to the app's ID (App().ID), else its executable's name.
	Service string
	// AccessGroup is the keychain access group of the items on Apple platforms, "" for the app's default. On macOS one keeps the items in the data protection keychain rather than the login keychain, which needs a signed app whose keychain-access-groups entitlement lists the group.
	AccessGroup string
	// Accessible is when iOS, tvOS and watchOS, and macOS's data protection keychain (AccessGroup), let the app read an item it writes; macOS's login keychain ignores it, as other platforms do.
	Accessible SecretAccessible

	store driver.SecretStore
	// refused is why the Access of the scope the secrets came from refuses them, nil when it allows them.
	refused error
	// part is the service of the partition the scope was given (ProvideSecrets), "" for none.
	part string
}

// secretsKey is where ProvideSecrets keeps an owner's partition.
type secretsKey struct{}

// ProvideSecrets gives the current owner and what it owns a partition of the app's secrets, name's: their Secrets keep their items under a service of their own, the app's own Service and the partition's name, and cannot name another service or access group. A host app gives code it did not write a partition of its own this way (tsx.View), as ProvideStorage does its storage.
func (cx *Scope) ProvideSecrets(name string) {
	o := cx.current()
	if o.values == nil {
		o.values = map[any]any{}
	}
	o.values[secretsKey{}] = name
	cx.rt.secretsScoped = true
}

// Secrets returns the app's secure store under its own service, or under its partition's (ProvideSecrets). Code whose Access does not grant AccessSecrets gets one that fails with AccessDenied.
func (cx *Scope) Secrets() Secrets {
	s := Secrets{Service: secretService(), store: cx.rt.secrets}
	if err := cx.gate(serviceSecrets, ""); err != nil {
		s.refused = err
	}
	if cx.rt.secretsScoped {
		for o := cx.current(); o != nil; o = o.parent {
			if name, ok := o.values[secretsKey{}]; ok {
				s.part = s.Service + "#" + name.(string)
				s.Service = s.part
				break
			}
		}
	}
	return s
}

// secretService is the service an app's secrets are under by default: its ID, else its executable's name.
func secretService() string {
	if id := App().ID; id != "" {
		return id
	}
	exe, err := os.Executable()
	if err != nil {
		return "nsgo"
	}
	return strings.TrimSuffix(filepath.Base(exe), ".exe")
}

// item is the item of account, or why it cannot be reached.
func (s Secrets) item(account string) (driver.SecretItem, error) {
	switch {
	case s.refused != nil:
		return driver.SecretItem{}, s.refused
	case s.part != "" && (s.Service != s.part || s.AccessGroup != ""):
		return driver.SecretItem{}, AccessDenied(AccessSecrets, s.Service)
	case s.store == nil:
		return driver.SecretItem{}, ErrSecretsUnavailable
	case account == "":
		return driver.SecretItem{}, errors.New("core: a secret needs an account")
	case s.Service == "":
		return driver.SecretItem{}, errors.New("core: a secret needs a service")
	}
	return driver.SecretItem{Service: s.Service, Account: account, AccessGroup: s.AccessGroup, Accessible: s.Accessible}, nil
}

// secretFailure is err as Secrets reports it: ErrSecretsUnavailable where the platform found no secure store.
func secretFailure(err error) error {
	if errors.Is(err, driver.ErrNoSecretStore) {
		return ErrSecretsUnavailable
	}
	return err
}

// Get returns the value kept for account and whether there is one; a missing one is ("", false, nil).
func (s Secrets) Get(account string) (string, bool, error) {
	item, err := s.item(account)
	if err != nil {
		return "", false, err
	}
	v, ok, err := s.store.ReadSecret(item)
	if err != nil {
		return "", false, secretFailure(err)
	}
	return v, ok, nil
}

// Set keeps value for account, replacing what it held.
func (s Secrets) Set(account, value string) error {
	item, err := s.item(account)
	if err != nil {
		return err
	}
	return secretFailure(s.store.WriteSecret(item, value))
}

// Delete removes what is kept for account; deleting what is not there is no failure.
func (s Secrets) Delete(account string) error {
	item, err := s.item(account)
	if err != nil {
		return err
	}
	return secretFailure(s.store.DeleteSecret(item))
}
