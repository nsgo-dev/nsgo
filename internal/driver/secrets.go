package driver

import (
	"errors"
	"sync"
)

// ErrNoSecretStore is what a SecretStore answers where the system has no secure store to keep secrets in, such as a Linux session without a Secret Service; core reports it as core.ErrSecretsUnavailable.
var ErrNoSecretStore = errors.New("the system has no secure store")

// SecretAccessible says when Apple platforms let the app read a secret, as kSecAttrAccessible does; the zero value leaves it to the system, which reads it only while the device is unlocked. Other platforms keep no such setting.
type SecretAccessible uint8

const (
	SecretAccessibleDefault SecretAccessible = iota
	// SecretWhenUnlocked is kSecAttrAccessibleWhenUnlocked.
	SecretWhenUnlocked
	// SecretAfterFirstUnlock is kSecAttrAccessibleAfterFirstUnlock, which background work reads while the device is locked.
	SecretAfterFirstUnlock
	// SecretWhenUnlockedThisDeviceOnly is kSecAttrAccessibleWhenUnlockedThisDeviceOnly: neither backups nor iCloud Keychain take it to another device.
	SecretWhenUnlockedThisDeviceOnly
	// SecretAfterFirstUnlockThisDeviceOnly is kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly.
	SecretAfterFirstUnlockThisDeviceOnly
	// SecretWhenPasscodeSetThisDeviceOnly is kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly: only while the device has a passcode, and gone when it is removed.
	SecretWhenPasscodeSetThisDeviceOnly
)

// SecretItem names a secret in the platform's secure store.
type SecretItem struct {
	// Service and Account name it: an Apple generic password's kSecAttrService and kSecAttrAccount, a Windows generic credential's target "Service/Account", the Secret Service's service and account attributes, and on Android the key of its ciphertext.
	Service, Account string
	// AccessGroup is the Apple keychain access group it is in, "" for the app's default.
	AccessGroup string
	// Accessible is when Apple platforms let the app read it, set as it is written.
	Accessible SecretAccessible
}

// SecretStore is where a platform keeps an app's secrets (core.Secrets). Its methods may be called on any goroutine and may wait for the system, as a keychain asking the user or Android's runtime does.
type SecretStore interface {
	// ReadSecret returns the value of item, ok false without one.
	ReadSecret(item SecretItem) (value string, ok bool, err error)
	// WriteSecret keeps value as item's, replacing what it held.
	WriteSecret(item SecretItem, value string) error
	// DeleteSecret removes item; removing one that is not there is no failure.
	DeleteSecret(item SecretItem) error
}

// MemorySecrets is a secure store of a test's own, in memory, so the machine's keychain is never touched. The zero value is empty and ready.
type MemorySecrets struct {
	mu    sync.Mutex
	items map[SecretItem]memorySecret
}

type memorySecret struct {
	value      string
	accessible SecretAccessible
}

// key is what names item in the store: its Accessible is how it was written, not its name.
func (item SecretItem) key() SecretItem {
	item.Accessible = SecretAccessibleDefault
	return item
}

func (m *MemorySecrets) ReadSecret(item SecretItem) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.items[item.key()]
	return s.value, ok, nil
}

func (m *MemorySecrets) WriteSecret(item SecretItem, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.items == nil {
		m.items = map[SecretItem]memorySecret{}
	}
	m.items[item.key()] = memorySecret{value, item.Accessible}
	return nil
}

func (m *MemorySecrets) DeleteSecret(item SecretItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, item.key())
	return nil
}

// Items returns a copy of what the store holds, each item with the Accessible it was last written with.
func (m *MemorySecrets) Items() map[SecretItem]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[SecretItem]string, len(m.items))
	for k, s := range m.items {
		k.Accessible = s.accessible
		out[k] = s.value
	}
	return out
}
