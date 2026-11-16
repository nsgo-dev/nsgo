//go:build darwin

package apple

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Keychain keeps an app's secrets as generic passwords of the keychain through Security.framework, in the process, as the item's access control trusts the binary that wrote it: kSecClassGenericPassword with the item's kSecAttrService and kSecAttrAccount, labeled with the service, as MyGo's App.Secrets and keytar keep them, so an app keeps reading what they wrote.
//
// macOS keeps them in the login keychain, which needs no entitlement; an AccessGroup takes them to the data protection keychain instead, which needs a signed app with the keychain-access-groups entitlement (unsigned builds get errSecMissingEntitlement), and only there does Accessible apply. iOS, tvOS and watchOS have only the data protection keychain.
type Keychain struct{}

const (
	errSecItemNotFound  = -25300
	errSecDuplicateItem = -25299
)

// keychainAPI is Security.framework's functions and keys, loaded with the first secret.
type keychainAPI struct {
	err                                  error
	add, copy, update, remove, message   uintptr
	class, genericPassword               uintptr
	service, account, label, accessGroup uintptr
	accessible                           uintptr
	accessibleValues                     map[driver.SecretAccessible]uintptr
	valueData, returnData                uintptr
	matchLimit, matchLimitOne            uintptr
	dataProtection, cfTrue               uintptr
}

var keychain = sync.OnceValue(func() *keychainAPI {
	lib, err := purego.Dlopen("/System/Library/Frameworks/Security.framework/Security", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return &keychainAPI{err: fmt.Errorf("keychain: Security.framework: %w: %w", err, driver.ErrNoSecretStore)}
	}
	cf := load("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation")
	k := &keychainAPI{
		add:     symbol(lib, "SecItemAdd"),
		copy:    symbol(lib, "SecItemCopyMatching"),
		update:  symbol(lib, "SecItemUpdate"),
		remove:  symbol(lib, "SecItemDelete"),
		message: symbol(lib, "SecCopyErrorMessageString"),

		class:           constant(lib, "kSecClass"),
		genericPassword: constant(lib, "kSecClassGenericPassword"),
		service:         constant(lib, "kSecAttrService"),
		account:         constant(lib, "kSecAttrAccount"),
		label:           constant(lib, "kSecAttrLabel"),
		accessGroup:     constant(lib, "kSecAttrAccessGroup"),
		accessible:      constant(lib, "kSecAttrAccessible"),
		valueData:       constant(lib, "kSecValueData"),
		returnData:      constant(lib, "kSecReturnData"),
		matchLimit:      constant(lib, "kSecMatchLimit"),
		matchLimitOne:   constant(lib, "kSecMatchLimitOne"),
		dataProtection:  constant(lib, "kSecUseDataProtectionKeychain"),
		cfTrue:          constant(cf, "kCFBooleanTrue"),
	}
	k.accessibleValues = map[driver.SecretAccessible]uintptr{
		driver.SecretWhenUnlocked:                   constant(lib, "kSecAttrAccessibleWhenUnlocked"),
		driver.SecretAfterFirstUnlock:               constant(lib, "kSecAttrAccessibleAfterFirstUnlock"),
		driver.SecretWhenUnlockedThisDeviceOnly:     constant(lib, "kSecAttrAccessibleWhenUnlockedThisDeviceOnly"),
		driver.SecretAfterFirstUnlockThisDeviceOnly: constant(lib, "kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly"),
		driver.SecretWhenPasscodeSetThisDeviceOnly:  constant(lib, "kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly"),
	}
	return k
})

// macOS's login keychain is file-based; its data protection keychain is the only one iOS, tvOS and watchOS have.
var onMac = runtime.GOOS == "darwin"

// query returns an autoreleased dictionary that finds item.
func (k *keychainAPI) query(item driver.SecretItem) objc.ID {
	q := objc.ID(Class("NSMutableDictionary")).Send(Sel("dictionary"))
	set := func(key uintptr, v objc.ID) { q.Send(Sel("setObject:forKey:"), v, objc.ID(key)) }
	set(k.class, objc.ID(k.genericPassword))
	set(k.service, NSString(item.Service))
	set(k.account, NSString(item.Account))
	if item.AccessGroup != "" {
		set(k.accessGroup, NSString(item.AccessGroup))
		if onMac {
			set(k.dataProtection, objc.ID(k.cfTrue))
		}
	}
	return q
}

// setAccessible sets item's Accessible in d, where it applies.
func (k *keychainAPI) setAccessible(d objc.ID, item driver.SecretItem) {
	if onMac && item.AccessGroup == "" {
		return
	}
	if v := k.accessibleValues[item.Accessible]; v != 0 {
		d.Send(Sel("setObject:forKey:"), objc.ID(v), objc.ID(k.accessible))
	}
}

// status is an OSStatus as an error with the system's message, nil for success.
func (k *keychainAPI) status(op string, r uintptr) error {
	s := int32(uint32(r))
	if s == 0 {
		return nil
	}
	msg := ""
	if str, _, _ := purego.SyscallN(k.message, uintptr(s), 0); str != 0 {
		msg = GoString(objc.ID(str))
		Release(str)
	}
	if msg == "" {
		msg = "unknown error"
	}
	return fmt.Errorf("keychain: %s: %s (OSStatus %d)", op, msg, s)
}

func (Keychain) ReadSecret(item driver.SecretItem) (value string, ok bool, err error) {
	k := keychain()
	if k.err != nil {
		return "", false, k.err
	}
	withPool(func() {
		q := k.query(item)
		q.Send(Sel("setObject:forKey:"), objc.ID(k.cfTrue), objc.ID(k.returnData))
		q.Send(Sel("setObject:forKey:"), objc.ID(k.matchLimitOne), objc.ID(k.matchLimit))
		var data uintptr
		r, _, _ := purego.SyscallN(k.copy, uintptr(q), uintptr(unsafe.Pointer(&data)))
		if int32(uint32(r)) == errSecItemNotFound {
			return
		}
		if err = k.status("read", r); err != nil {
			return
		}
		if data == 0 {
			err = errors.New("keychain: read: the item holds no data")
			return
		}
		value, ok = string(Bytes(objc.ID(data))), true
		Release(data)
	})
	return value, ok, err
}

func (Keychain) WriteSecret(item driver.SecretItem, value string) (err error) {
	k := keychain()
	if k.err != nil {
		return k.err
	}
	withPool(func() {
		data := NSData([]byte(value))
		change := objc.ID(Class("NSMutableDictionary")).Send(Sel("dictionary"))
		change.Send(Sel("setObject:forKey:"), data, objc.ID(k.valueData))
		k.setAccessible(change, item)
		r, _, _ := purego.SyscallN(k.update, uintptr(k.query(item)), uintptr(change))
		if int32(uint32(r)) != errSecItemNotFound {
			err = k.status("update", r)
			return
		}
		add := k.query(item)
		add.Send(Sel("setObject:forKey:"), data, objc.ID(k.valueData))
		add.Send(Sel("setObject:forKey:"), NSString(item.Service), objc.ID(k.label))
		k.setAccessible(add, item)
		r, _, _ = purego.SyscallN(k.add, uintptr(add), 0)
		if int32(uint32(r)) == errSecDuplicateItem {
			// Another thread or process added it since the update.
			r, _, _ = purego.SyscallN(k.update, uintptr(k.query(item)), uintptr(change))
			err = k.status("update", r)
			return
		}
		err = k.status("add", r)
	})
	return err
}

func (Keychain) DeleteSecret(item driver.SecretItem) (err error) {
	k := keychain()
	if k.err != nil {
		return k.err
	}
	withPool(func() {
		r, _, _ := purego.SyscallN(k.remove, uintptr(k.query(item)))
		if int32(uint32(r)) != errSecItemNotFound {
			err = k.status("delete", r)
		}
	})
	return err
}
