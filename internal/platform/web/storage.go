//go:build js && wasm

package web

import (
	"fmt"
	"log"
	"strings"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// localStorage keeps the app's entries in the browser's localStorage, each
// under its key prefixed with "nsgo:<app id>:", so apps on one origin keep
// apart.
type localStorage struct {
	prefix string
}

func storageAppID(id string) string {
	if id == "" {
		return "default"
	}
	return id
}

func newLocalStorage(id string) *localStorage {
	return &localStorage{prefix: "nsgo:" + storageAppID(id) + ":"}
}

// Value.Get does not catch JavaScript exceptions. In an opaque-origin
// frame, even reading window.localStorage (or indexedDB) can throw.
func storageGlobal(name string) js.Value {
	return js.Global().Get("Reflect").Call("get", js.Global(), name)
}

func storageException(op string, err *error) {
	if r := recover(); r != nil {
		if e, ok := r.(js.Error); ok {
			*err = fmt.Errorf("%s: %s", op, e.Value.Call("toString").String())
			return
		}
		panic(r)
	}
}

func localStorageValue() (js.Value, error) {
	ls := storageGlobal("localStorage")
	if !ls.Truthy() {
		return ls, fmt.Errorf("localStorage is unavailable")
	}
	return ls, nil
}

func (l *localStorage) Load() (entries map[string]string, err error) {
	defer storageException("localStorage load", &err)
	ls, err := localStorageValue()
	if err != nil {
		return nil, err
	}
	entries = map[string]string{}
	for _, k := range l.keys(ls) {
		if v := ls.Call("getItem", k); !v.IsNull() {
			entries[k[len(l.prefix):]] = v.String()
		}
	}
	return entries, nil
}

// keys returns the stored keys with the app's prefix.
func (l *localStorage) keys(ls js.Value) []string {
	var keys []string
	for i, n := 0, ls.Get("length").Int(); i < n; i++ {
		k := ls.Call("key", i).String()
		if strings.HasPrefix(k, l.prefix) {
			keys = append(keys, k)
		}
	}
	return keys
}

func (l *localStorage) Write(c driver.StorageChange) {
	if err := l.WriteErr(c); err != nil {
		log.Printf("nsgo: the app's storage could not be written: %v", err)
	}
}

func (l *localStorage) WriteErr(c driver.StorageChange) (err error) {
	defer storageException("localStorage write", &err)
	ls, err := localStorageValue()
	if err != nil {
		return err
	}
	switch c.Op {
	case driver.StorageSet:
		ls.Call("setItem", l.prefix+c.Key, c.Value)
	case driver.StorageRemove:
		ls.Call("removeItem", l.prefix+c.Key)
	case driver.StorageClear:
		for _, k := range l.keys(ls) {
			ls.Call("removeItem", k)
		}
	}
	return nil
}
