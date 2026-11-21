//go:build js && wasm

package crash

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

const webStoragePrefix = "nsgo.crash.reports"

var (
	webMu          sync.Mutex
	webMemStores   = map[string][]CrashReport{}
	webInstalled   bool
	webErrorFn     js.Func
	webRejectFn    js.Func
	webActiveStore *Store
)

func installPlatform(opts Options) error {
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn().UTC()
	meta := resolveSessionMeta(opts, now)
	s := &Store{
		dir:  opts.Dir,
		meta: meta,
		now:  nowFn,
	}

	webMu.Lock()
	webActiveStore = s
	if !webInstalled {
		installWebListenersLocked()
		webInstalled = true
	}
	webMu.Unlock()

	setDefaultStore(s)
	return nil
}

// Promote is a no-op on the web because unhandled browser errors are saved
// directly as pending reports when they fire.
func (s *Store) Promote() error { return nil }

// PendingReports returns all unacknowledged crash reports in s sorted in
// chronological order (oldest first).
func (s *Store) PendingReports() ([]CrashReport, error) {
	if s == nil {
		return nil, nil
	}
	webMu.Lock()
	defer webMu.Unlock()

	reports := loadWebReportsLocked(s.storageKey())
	sortReports(reports)
	return reports, nil
}

// WriteReport persists r into s (localStorage when available, else in-memory).
func (s *Store) WriteReport(r CrashReport) (CrashReport, error) {
	if s == nil {
		return CrashReport{}, nil
	}
	norm, err := normalizeReport(r, s.meta, s.nowTime())
	if err != nil {
		return CrashReport{}, err
	}

	webMu.Lock()
	defer webMu.Unlock()

	key := s.storageKey()
	reports := loadWebReportsLocked(key)
	replaced := false
	for i := range reports {
		if reports[i].ID == norm.ID {
			reports[i] = norm
			replaced = true
			break
		}
	}
	if !replaced {
		reports = append(reports, norm)
	}
	sortReports(reports)
	if len(reports) > maxPendingReports {
		reports = reports[len(reports)-maxPendingReports:]
	}
	saveWebReportsLocked(key, reports)
	return norm, nil
}

// Acknowledge deletes the pending crash report with id from s.
func (s *Store) Acknowledge(id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	if s == nil {
		return nil
	}
	webMu.Lock()
	defer webMu.Unlock()

	key := s.storageKey()
	reports := loadWebReportsLocked(key)
	out := reports[:0]
	for _, r := range reports {
		if r.ID != id {
			out = append(out, r)
		}
	}
	saveWebReportsLocked(key, out)
	return nil
}

// AcknowledgeAll deletes every pending crash report from s.
func (s *Store) AcknowledgeAll() error {
	if s == nil {
		return nil
	}
	webMu.Lock()
	defer webMu.Unlock()
	saveWebReportsLocked(s.storageKey(), nil)
	return nil
}

func (s *Store) storageKey() string {
	if s != nil && s.dir != "" {
		return webStoragePrefix + ":" + s.dir
	}
	if id := core.App().ID; id != "" {
		return webStoragePrefix + ":" + id
	}
	return webStoragePrefix
}

func installWebListenersLocked() {
	global := js.Global()
	if global.IsUndefined() || global.IsNull() {
		return
	}
	add := global.Get("addEventListener")
	if add.Type() != js.TypeFunction {
		return
	}
	webErrorFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		recordWebErrorEvent(args[0])
		return nil
	})
	webRejectFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		recordWebRejectionEvent(args[0])
		return nil
	})
	global.Call("addEventListener", "error", webErrorFn)
	global.Call("addEventListener", "unhandledrejection", webRejectFn)
}

func recordWebErrorEvent(ev js.Value) {
	webMu.Lock()
	s := webActiveStore
	webMu.Unlock()
	if s == nil {
		return
	}
	var b strings.Builder
	msg := safeJSString(ev, "message")
	if msg == "" {
		msg = "Unhandled error"
	}
	fmt.Fprintf(&b, "Error: %s", msg)
	file := safeJSString(ev, "filename")
	if file != "" {
		lineno := safeJSInt(ev, "lineno")
		colno := safeJSInt(ev, "colno")
		fmt.Fprintf(&b, "\nLocation: %s:%d:%d", file, lineno, colno)
	}
	if errObj := ev.Get("error"); !errObj.IsUndefined() && !errObj.IsNull() {
		if stack := safeJSString(errObj, "stack"); stack != "" {
			fmt.Fprintf(&b, "\n%s", stack)
		}
	}
	_, _ = s.WriteReport(CrashReport{
		Kind:  KindNativeException,
		Trace: b.String(),
	})
}

func recordWebRejectionEvent(ev js.Value) {
	webMu.Lock()
	s := webActiveStore
	webMu.Unlock()
	if s == nil {
		return
	}
	reason := ev.Get("reason")
	var trace string
	if !reason.IsUndefined() && !reason.IsNull() {
		if stack := safeJSString(reason, "stack"); stack != "" {
			trace = "Unhandled Promise Rejection:\n" + stack
		} else {
			trace = "Unhandled Promise Rejection: " + reason.String()
		}
	} else {
		trace = "Unhandled Promise Rejection"
	}
	_, _ = s.WriteReport(CrashReport{
		Kind:  KindNativeException,
		Trace: trace,
	})
}

func safeJSString(v js.Value, prop string) string {
	if v.IsUndefined() || v.IsNull() {
		return ""
	}
	f := v.Get(prop)
	if f.Type() == js.TypeString {
		return f.String()
	}
	return ""
}

func safeJSInt(v js.Value, prop string) int {
	if v.IsUndefined() || v.IsNull() {
		return 0
	}
	f := v.Get(prop)
	if f.Type() == js.TypeNumber {
		return f.Int()
	}
	return 0
}

func localStorage() (js.Value, bool) {
	global := js.Global()
	if global.IsUndefined() || global.IsNull() {
		return js.Undefined(), false
	}
	var storage js.Value
	ok := false
	func() {
		defer func() { _ = recover() }()
		s := global.Get("localStorage")
		if !s.IsUndefined() && !s.IsNull() && s.Get("getItem").Type() == js.TypeFunction {
			storage = s
			ok = true
		}
	}()
	return storage, ok
}

func loadWebReportsLocked(key string) []CrashReport {
	if ls, ok := localStorage(); ok {
		var raw string
		var valid bool
		func() {
			defer func() { _ = recover() }()
			v := ls.Call("getItem", key)
			if v.Type() == js.TypeString {
				raw = v.String()
				valid = true
			}
		}()
		if valid && raw != "" {
			var reports []CrashReport
			if err := json.Unmarshal([]byte(raw), &reports); err == nil {
				return reports
			}
		}
		return nil
	}
	src := webMemStores[key]
	if len(src) == 0 {
		return nil
	}
	out := make([]CrashReport, len(src))
	copy(out, src)
	return out
}

func saveWebReportsLocked(key string, reports []CrashReport) {
	if ls, ok := localStorage(); ok {
		saved := false
		func() {
			defer func() { _ = recover() }()
			if len(reports) == 0 {
				ls.Call("removeItem", key)
			} else if data, err := json.Marshal(reports); err == nil {
				ls.Call("setItem", key, string(data))
			}
			saved = true
		}()
		if saved {
			return
		}
	}
	if len(reports) == 0 {
		delete(webMemStores, key)
		return
	}
	cp := make([]CrashReport, len(reports))
	copy(cp, reports)
	webMemStores[key] = cp
}
