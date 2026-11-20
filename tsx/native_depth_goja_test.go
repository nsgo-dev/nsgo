//go:build !js

package tsx

import (
	"fmt"
	"os"
	"os/exec"
	"runtime/metrics"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"
)

// nativeDepthCases have native functions call each other, which goja did without counting them against Limits.Stack, so each recursed in Go until the goroutine's stack overflowed, ending the process whatever the sandbox's limits (#928). Methods are defined rather than assigned, as hardening freezes the prototypes whose methods assignment would otherwise shadow. A case starting with "export:" builds a value the host then exports, as a plugin call's result is.
var nativeDepthCases = map[string]string{
	"toLocaleString":    `const o = {}; Object.defineProperty(o, "toString", { value: Object.prototype.toLocaleString }); o.toLocaleString()`,
	"valueOf":           `const o = {}; Object.defineProperties(o, { valueOf: { value: Object.prototype.toLocaleString }, toString: { value: Object.prototype.toLocaleString } }); +o`,
	"regexp test":       `const r = /x/; Object.defineProperty(r, "exec", { value: RegExp.prototype.test }); r.test("x")`,
	"date toJSON":       `const o = {}; Object.defineProperty(o, "toISOString", { value: Date.prototype.toJSON }); o.toISOString()`,
	"getter":            `const o = {}; Object.defineProperty(o, "toString", { get: Object.prototype.toLocaleString }); String(o)`,
	"setter":            `const o = {}; Object.defineProperty(o, "x", { set: Reflect.set.bind(null, o, "x", 1, o) }); o.x = 1`,
	"symbol match":      `const r = /x/; Object.defineProperty(r, "exec", { value: RegExp.prototype[Symbol.match] }); r.test("x")`,
	"symbol replace":    `const r = /x/; Object.defineProperty(r, "exec", { value: RegExp.prototype[Symbol.replace] }); "x".replace(r, "y")`,
	"reflect apply":     `let a = [() => 1, null, []]; for (let i = 0; i < 1e6; i++) a = [Reflect.apply, null, a]; Reflect.apply(...a)`,
	"reflect construct": `let C = function () {}; for (let i = 0; i < 1e5; i++) C = C.bind(null); Reflect.construct(C, [])`,
	"bound chain":       `let f = () => 1; for (let i = 0; i < 1e6; i++) f = f.bind(null); f()`,
	"bound new":         `let C = function () {}; for (let i = 0; i < 1e5; i++) C = C.bind(null); new C()`,
	"proxy call":        `let f = function () {}; for (let i = 0; i < 1e5; i++) f = new Proxy(f, {}); f()`,
	"export deep":       `export:deep(1e6)`,
	"export getter":     `export:(() => { const d = deep(1e6); let n = 0; return { get a() { return n++ ? d : 1; } }; })()`,
}

// TestNativeDepthThrowsARangeError runs each case in a process of its own, as one that overflows its stack ends the process, with and without a memory budget, at the sandbox's default stack depth: each must end with a RangeError, which a script catches, and the runtime must keep running. The process fails past 2 GiB, as a chain of a million bound functions used past 20 GB.
func TestNativeDepthThrowsARangeError(t *testing.T) {
	if name := os.Getenv("NSGO_TEST_NATIVE_DEPTH_CASE"); name != "" {
		go func() {
			s := []metrics.Sample{{Name: "/memory/classes/total:bytes"}}
			for range time.Tick(20 * time.Millisecond) {
				if metrics.Read(s); s[0].Value.Uint64() > 2<<30 {
					fmt.Println("memory past 2 GiB")
					os.Exit(3)
				}
			}
		}()
		for _, limit := range []int64{0, 256 << 20} {
			r := limited(limit)
			r.vm.SetMaxCallStackSize(DefaultLimits.Stack)
			src := nativeDepthCases[name]
			if value, ok := strings.CutPrefix(src, "export:"); ok {
				v, err := r.vm.RunString(`function deep(n) { let a = {}; for (let i = 0; i < n; i++) a = { a }; return a; } ` + value)
				if err != nil {
					t.Fatalf("memory %d: %v", limit, err)
				}
				if name == "export getter" {
					exportJSON(v)
				}
				if got := exportPanic(v); !strings.HasPrefix(got, "RangeError") {
					t.Fatalf("memory %d: exporting panicked with %q, want a RangeError", limit, got)
				}
			} else {
				v, err := r.vm.RunString(`let caught; try { ` + src + `; caught = "none"; } catch (e) { caught = e instanceof RangeError ? "RangeError" : String(e); } caught`)
				if err != nil {
					t.Fatalf("memory %d: %v", limit, err)
				}
				if v.String() != "RangeError" {
					t.Fatalf("memory %d: caught %s, want RangeError", limit, v)
				}
			}
			if v, err := r.vm.RunString(`JSON.stringify([[1], { a: [2] }])`); err != nil || v.String() != `[[1],{"a":[2]}]` {
				t.Fatalf("memory %d: after it, %v, %v", limit, v, err)
			}
		}
		return
	}
	for name := range nativeDepthCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			start := time.Now()
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestNativeDepthThrowsARangeError$", "-test.v")
			cmd.Env = append(os.Environ(), "NSGO_TEST_NATIVE_DEPTH_CASE="+name)
			cmd.WaitDelay = time.Second
			timer := time.AfterFunc(2*time.Minute, func() { _ = cmd.Process.Kill() })
			defer timer.Stop()
			out, err := cmd.CombinedOutput()
			if err != nil {
				text := string(out)
				if i := strings.Index(text, "\ngoroutine "); i >= 0 {
					text = text[:i]
				}
				t.Fatalf("after %v: %v\n%s", time.Since(start).Round(time.Millisecond), err, text)
			}
		})
	}
}

// exportPanic is what exporting v panics with, "" when it does not.
func exportPanic(v js.Value) (got string) {
	defer func() {
		switch x := recover().(type) {
		case nil:
		case *js.Exception:
			got = x.Value().String()
		default:
			got = fmt.Sprint(x)
		}
	}()
	exportJSON(v)
	return ""
}
