//go:build js && wasm

package web

import (
	"fmt"
	"log"
	"sync"
	"syscall/js"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

const (
	indexedStorageQuota = 64 << 20
	indexedEntries      = "entries"
)

// indexedStorage keeps the app's entries in one database per app ID, with
// the core store's partition prefixes left intact in the keys. The engine
// goroutine can wait for the browser; JavaScript callbacks must not wait for
// it while it does. The core Store serializes Load and WriteErr.
type indexedStorage struct {
	name string
	db   js.Value

	mu      sync.Mutex
	pending chan struct{}
}

func newIndexedStorage(id string) *indexedStorage {
	return &indexedStorage{name: "nsgo:" + storageAppID(id), pending: make(chan struct{})}
}

// waiting closes when a storage operation needs the browser event loop.
func (b *indexedStorage) waiting() <-chan struct{} {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.pending
}

func (b *indexedStorage) beginWait() {
	b.mu.Lock()
	close(b.pending)
	b.mu.Unlock()
}

func (b *indexedStorage) endWait() {
	b.mu.Lock()
	b.pending = make(chan struct{})
	b.mu.Unlock()
}

func (b *indexedStorage) Load() (entries map[string]string, err error) {
	defer storageException("IndexedDB load", &err)
	api := storageGlobal("indexedDB")
	if !api.Truthy() {
		return nil, fmt.Errorf("IndexedDB is unavailable")
	}
	request := api.Call("open", b.name)
	b.beginWait()
	defer b.endWait()
	db, err := openIndexedStorage(request)
	if err != nil {
		return nil, err
	}
	loaded := false
	defer func() {
		if !loaded {
			db.Call("close")
		}
	}()

	// Both requests use the same readonly snapshot and enumerate the store
	// in the same key order. Neither needs a round trip per entry.
	tx := db.Call("transaction", indexedEntries, "readonly")
	store := tx.Call("objectStore", indexedEntries)
	keys, values := store.Call("getAllKeys"), store.Call("getAll")
	if err := waitIndexedTransaction(tx); err != nil {
		return nil, err
	}
	ks, vs := keys.Get("result"), values.Get("result")
	n := ks.Length()
	entries = make(map[string]string, n)
	for i := range n {
		entries[ks.Index(i).String()] = vs.Index(i).String()
	}
	b.db = db
	loaded = true
	return entries, nil
}

func openIndexedStorage(request js.Value) (js.Value, error) {
	done := make(chan error, 1)
	var ls listeners
	defer ls.release()
	var upgradeErr error
	ls.on(request, "upgradeneeded", func(js.Value) {
		func() {
			defer storageException("IndexedDB upgrade", &upgradeErr)
			request.Get("result").Call("createObjectStore", indexedEntries)
		}()
		if upgradeErr != nil {
			request.Get("transaction").Call("abort")
		}
	})
	ls.on(request, "success", func(js.Value) { done <- nil })
	ls.on(request, "error", func(js.Value) {
		if upgradeErr != nil {
			done <- upgradeErr
		} else {
			done <- indexedError("open", request.Get("error"))
		}
	})
	if err := <-done; err != nil {
		return js.Undefined(), err
	}
	return request.Get("result"), nil
}

func (b *indexedStorage) Write(c driver.StorageChange) {
	if err := b.WriteErr(c); err != nil {
		log.Printf("nsgo: the app's storage could not be written: %v", err)
	}
}

func (b *indexedStorage) WriteErr(c driver.StorageChange) (err error) {
	defer storageException("IndexedDB write", &err)
	tx := b.db.Call("transaction", indexedEntries, "readwrite", map[string]any{"durability": "strict"})
	store := tx.Call("objectStore", indexedEntries)
	switch c.Op {
	case driver.StorageSet:
		store.Call("put", c.Value, c.Key)
	case driver.StorageRemove:
		store.Call("delete", c.Key)
	case driver.StorageClear:
		store.Call("clear")
	}
	b.beginWait()
	defer b.endWait()
	return waitIndexedTransaction(tx)
}

// A request's success event does not mean the transaction committed. A
// quota error or abort can still roll it back before the complete event.
func waitIndexedTransaction(tx js.Value) error {
	done := make(chan error, 1)
	var ls listeners
	defer ls.release()
	ls.on(tx, "complete", func(js.Value) { done <- nil })
	ls.on(tx, "abort", func(js.Value) { done <- indexedError("transaction aborted", tx.Get("error")) })
	return <-done
}

func indexedError(op string, e js.Value) error {
	if !e.Truthy() {
		return fmt.Errorf("IndexedDB %s", op)
	}
	return fmt.Errorf("IndexedDB %s: %s", op, e.Call("toString").String())
}
