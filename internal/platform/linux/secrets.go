//go:build linux && !android

package linux

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// secretService keeps an app's secrets as items of the Secret Service, org.freedesktop.secrets on the session bus, which GNOME Keyring and KWallet serve, over GDBus: an item of the default collection with the service and account attributes and libsecret's generic schema, labeled with the service, as keytar and libsecret's secret-tool keep them, so an app keeps reading what they wrote. Values cross the bus in a plain session, as the bus is the user's own. A locked collection or item is unlocked with the service's prompt, whose answer the main loop hears. Without a Secret Service the calls fail with driver.ErrNoSecretStore. Linux has no access groups or accessibility, which it ignores.
type secretService struct{}

const (
	secretName            = "org.freedesktop.secrets"
	secretPath            = "/org/freedesktop/secrets"
	secretDefault         = "/org/freedesktop/secrets/aliases/default"
	secretServiceIface    = "org.freedesktop.Secret.Service"
	secretItemIface       = "org.freedesktop.Secret.Item"
	secretCollectionIface = "org.freedesktop.Secret.Collection"
	secretPromptIface     = "org.freedesktop.Secret.Prompt"
	// secretPromptWait is how long a prompt may wait for the user.
	secretPromptWait = 2 * time.Minute
)

var (
	gVariantGetFixedArray func(v uintptr, n *uintptr, elementSize uintptr) uintptr
	gVariantGetBool       func(v uintptr) int32
	bindSecrets           = sync.OnceFunc(func() {
		trayBind()
		bindGlobalHotKeys()
		purego.RegisterLibFunc(&gVariantGetFixedArray, glibLib, "g_variant_get_fixed_array")
		purego.RegisterLibFunc(&gVariantGetBool, glibLib, "g_variant_get_boolean")
	})
)

// secrets serves one request at a time, through the session it opened, "" until it opens one.
var secrets struct {
	mu      sync.Mutex
	session string
}

// secretCall calls a method of the Secret Service, waiting as long as the bus lets it, as unlocking may; a service the bus has none of is driver.ErrNoSecretStore.
func secretCall(bus uintptr, path, iface, method string, args ...uintptr) (uintptr, error) {
	var params uintptr
	if len(args) > 0 {
		params = gVariantNewTuple(&args[0], uintptr(len(args)))
	} else {
		params = gVariantNewTuple(nil, 0)
	}
	var gerr uintptr
	reply := gDBusConnectionCallSync(bus, cstr(secretName), cstr(path), cstr(iface), cstr(method), params, 0, 0, -1, 0, &gerr)
	if gerr != 0 {
		defer gErrorFree(gerr)
		msg := goString((*gError)(cPointer(gerr)).message)
		if strings.Contains(msg, "org.freedesktop.DBus.Error.ServiceUnknown") || strings.Contains(msg, "org.freedesktop.DBus.Error.NameHasNoOwner") {
			return 0, fmt.Errorf("secret service: %s: %w", msg, driver.ErrNoSecretStore)
		}
		return 0, fmt.Errorf("secret service: %s.%s: %s", iface, method, msg)
	}
	return reply, nil
}

// secretChildPath is the object path child i of a tuple holds, "" without one.
func secretChildPath(tuple uintptr, i int) string {
	child := gVariantGetChildValue(tuple, uintptr(i))
	defer gVariantUnref(child)
	if t := goString(gVariantGetTypeString(child)); t != "o" {
		return ""
	}
	return goString(gVariantGetString(child, 0))
}

// secretChildPaths are the object paths of the array that child i of a tuple is.
func secretChildPaths(tuple uintptr, i int) []string {
	child := gVariantGetChildValue(tuple, uintptr(i))
	defer gVariantUnref(child)
	n := int(gVariantNChildren(child))
	paths := make([]string, 0, n)
	for j := range n {
		p := gVariantGetChildValue(child, uintptr(j))
		paths = append(paths, goString(gVariantGetString(p, 0)))
		gVariantUnref(p)
	}
	return paths
}

// secretBytes is a new floating ay of b.
func secretBytes(b []byte) uintptr {
	y := gVariantTypeNew(cstr("y"))
	defer gVariantTypeFree(y)
	if len(b) == 0 {
		var none byte
		return gVariantNewFixedArray(y, unsafe.Pointer(&none), 0, 1)
	}
	return gVariantNewFixedArray(y, unsafe.Pointer(&b[0]), uintptr(len(b)), 1)
}

// secretAttributes is a new floating a{ss} of item's attributes: its service and account, and libsecret's schema when written, as a search finds what was written without it too.
func secretAttributes(item driver.SecretItem, written bool) uintptr {
	str := func(s string) uintptr { return gVariantNewString(cstr(s)) }
	entries := []uintptr{
		gVariantNewDictEntry(str("service"), str(item.Service)),
		gVariantNewDictEntry(str("account"), str(item.Account)),
	}
	if written {
		entries = append(entries, gVariantNewDictEntry(str("xdg:schema"), str("org.freedesktop.Secret.Generic")))
	}
	t := gVariantTypeNew(cstr("{ss}"))
	defer gVariantTypeFree(t)
	return gVariantNewArray(t, &entries[0], uintptr(len(entries)))
}

// secretSession returns the plain session, opening it first. The caller holds secrets.mu.
func secretSession(bus uintptr) (string, error) {
	if secrets.session != "" {
		return secrets.session, nil
	}
	reply, err := secretCall(bus, secretPath, secretServiceIface, "OpenSession",
		gVariantNewString(cstr("plain")), gVariantNewVariant(gVariantNewString(cstr(""))))
	if err != nil {
		return "", err
	}
	defer gVariantUnref(reply)
	secrets.session = secretChildPath(reply, 1)
	if secrets.session == "" {
		return "", errors.New("secret service: OpenSession answered no session")
	}
	return secrets.session, nil
}

// secretPrompts are the prompts waiting for their Completed signal, by path, which the main loop delivers; each hears whether the user dismissed it.
var secretPrompts struct {
	mu      sync.Mutex
	waiting map[string]chan bool
}

// secretPrompt shows the Secret Service's prompt at path, unless it is "/", none, and waits for the user's answer.
func secretPrompt(bus uintptr, path string) error {
	if path == "" || path == "/" {
		return nil
	}
	done := make(chan bool, 1)
	secretPrompts.mu.Lock()
	if secretPrompts.waiting == nil {
		secretPrompts.waiting = map[string]chan bool{}
	}
	secretPrompts.waiting[path] = done
	secretPrompts.mu.Unlock()
	sub := gDBusConnectionSignalSubscribe(bus, cstr(secretName), cstr(secretPromptIface), cstr("Completed"), cstr(path), nil, 0, cbSecretPrompt, 0, 0)
	defer func() {
		gDBusConnectionSignalUnsubscribe(bus, sub)
		secretPrompts.mu.Lock()
		delete(secretPrompts.waiting, path)
		secretPrompts.mu.Unlock()
	}()
	reply, err := secretCall(bus, path, secretPromptIface, "Prompt", gVariantNewString(cstr("")))
	if err != nil {
		return err
	}
	gVariantUnref(reply)
	select {
	case dismissed := <-done:
		if dismissed {
			return errors.New("secret service: the user dismissed the prompt")
		}
		return nil
	case <-time.After(secretPromptWait):
		gVariantUnrefNonZero(dbusCall(bus, secretName, path, secretPromptIface, "Dismiss"))
		return errors.New("secret service: the prompt went unanswered")
	}
}

// secretPromptCompleted hears a prompt's Completed(dismissed, result), on the main thread.
func secretPromptCompleted(conn, sender, path, iface, signal, params, data uintptr) {
	dismissed := true
	if params != 0 {
		child := gVariantGetChildValue(params, 0)
		dismissed = gVariantGetBool(child) != 0
		gVariantUnref(child)
	}
	secretPrompts.mu.Lock()
	done := secretPrompts.waiting[goString(path)]
	secretPrompts.mu.Unlock()
	if done != nil {
		select {
		case done <- dismissed:
		default:
		}
	}
}

// secretUnlock unlocks objects, prompting the user where the service asks to.
func secretUnlock(bus uintptr, objects []string) error {
	if len(objects) == 0 {
		return nil
	}
	paths := make([]uintptr, len(objects))
	for i, o := range objects {
		paths[i] = gVariantNewObjectPath(cstr(o))
	}
	t := gVariantTypeNew(cstr("o"))
	array := gVariantNewArray(t, &paths[0], uintptr(len(paths)))
	gVariantTypeFree(t)
	reply, err := secretCall(bus, secretPath, secretServiceIface, "Unlock", array)
	if err != nil {
		return err
	}
	prompt := secretChildPath(reply, 1)
	gVariantUnref(reply)
	return secretPrompt(bus, prompt)
}

// secretFind returns the paths of the items named as item is, unlocking the locked ones.
func secretFind(bus uintptr, item driver.SecretItem) ([]string, error) {
	reply, err := secretCall(bus, secretPath, secretServiceIface, "SearchItems", secretAttributes(item, false))
	if err != nil {
		return nil, err
	}
	unlocked, locked := secretChildPaths(reply, 0), secretChildPaths(reply, 1)
	gVariantUnref(reply)
	if len(locked) > 0 {
		if err := secretUnlock(bus, locked); err != nil {
			return nil, err
		}
		unlocked = append(unlocked, locked...)
	}
	return unlocked, nil
}

// secretRun runs fn with the bus and the session, one request at a time; a failure forgets the session, which a restarted service no longer has.
func secretRun(fn func(bus uintptr, session string) error) error {
	bindSecrets()
	bus := sessionBus()
	if bus == 0 {
		return fmt.Errorf("secret service: no session bus: %w", driver.ErrNoSecretStore)
	}
	secrets.mu.Lock()
	defer secrets.mu.Unlock()
	session, err := secretSession(bus)
	if err == nil {
		err = fn(bus, session)
	}
	if err != nil {
		secrets.session = ""
	}
	return err
}

func (secretService) ReadSecret(item driver.SecretItem) (value string, ok bool, err error) {
	err = secretRun(func(bus uintptr, session string) error {
		items, err := secretFind(bus, item)
		if err != nil || len(items) == 0 {
			return err
		}
		reply, err := secretCall(bus, items[0], secretItemIface, "GetSecret", gVariantNewObjectPath(cstr(session)))
		if err != nil {
			return err
		}
		defer gVariantUnref(reply)
		secret := gVariantGetChildValue(reply, 0)
		defer gVariantUnref(secret)
		data := gVariantGetChildValue(secret, 2)
		defer gVariantUnref(data)
		var n uintptr
		p := gVariantGetFixedArray(data, &n, 1)
		if n > 0 && p != 0 {
			value = string(unsafe.Slice((*byte)(cPointer(p)), n))
		}
		ok = true
		return nil
	})
	return value, ok, err
}

// secretValue is a new floating (oayays) of value in session.
func secretValue(session, value string) uintptr {
	parts := []uintptr{gVariantNewObjectPath(cstr(session)), secretBytes(nil), secretBytes([]byte(value)),
		gVariantNewString(cstr("text/plain"))}
	return gVariantNewTuple(&parts[0], uintptr(len(parts)))
}

func (secretService) WriteSecret(item driver.SecretItem, value string) error {
	return secretRun(func(bus uintptr, session string) error {
		items, err := secretFind(bus, item)
		if err != nil {
			return err
		}
		if len(items) > 0 {
			reply, err := secretCall(bus, items[0], secretItemIface, "SetSecret", secretValue(session, value))
			if err == nil {
				gVariantUnref(reply)
			}
			return err
		}
		if err := secretUnlock(bus, []string{secretDefault}); err != nil {
			return err
		}
		str := func(s string) uintptr { return gVariantNewString(cstr(s)) }
		props := []uintptr{
			gVariantNewDictEntry(str("org.freedesktop.Secret.Item.Label"), gVariantNewVariant(str(item.Service))),
			gVariantNewDictEntry(str("org.freedesktop.Secret.Item.Attributes"), gVariantNewVariant(secretAttributes(item, true))),
		}
		t := gVariantTypeNew(cstr("{sv}"))
		array := gVariantNewArray(t, &props[0], uintptr(len(props)))
		gVariantTypeFree(t)
		reply, err := secretCall(bus, secretDefault, secretCollectionIface, "CreateItem", array, secretValue(session, value), gVariantNewBoolean(1))
		if err != nil {
			return err
		}
		prompt := secretChildPath(reply, 1)
		gVariantUnref(reply)
		return secretPrompt(bus, prompt)
	})
}

func (secretService) DeleteSecret(item driver.SecretItem) error {
	return secretRun(func(bus uintptr, _ string) error {
		items, err := secretFind(bus, item)
		if err != nil {
			return err
		}
		for _, path := range items {
			reply, err := secretCall(bus, path, secretItemIface, "Delete")
			if err != nil {
				return err
			}
			prompt := secretChildPath(reply, 0)
			gVariantUnref(reply)
			if err := secretPrompt(bus, prompt); err != nil {
				return err
			}
		}
		return nil
	})
}
