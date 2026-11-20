//go:build !js

package tsx

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// nestingCases build data a million levels deep, or an array whose join is its toString, which calls it again (defined, as a hardened Array.prototype keeps assignment from shadowing join), and hand it to each built-in that goes down it in Go. Before limitNesting each either searched every level above for a cycle at every level, for hours, or overflowed the goroutine's stack, which ends the process (#875).
var nestingCases = map[string]string{
	"stringify array":    `JSON.stringify(deepArray())`,
	"stringify object":   `JSON.stringify(deepObject())`,
	"stringify indented": `JSON.stringify(deepObject(), null, 2)`,
	"stringify keys":     `JSON.stringify(deepObject(), ["a"])`,
	"stringify replacer": `const d = deepArray(); JSON.stringify({}, (k, v) => (k === "" ? [v, d] : v))`,
	"stringify toJSON":   `const d = deepArray(); JSON.stringify({ toJSON: () => d })`,
	"stringify getter":   `const d = deepArray(); JSON.stringify({ get a() { return d; } })`,
	"string":             `String(deepArray())`,
	"template":           "`${deepArray()}`",
	"join":               `deepArray().join("-")`,
	"toLocaleString":     `deepArray().toLocaleString()`,
	"flat":               `deepArray().flat(Infinity)`,
	"flat deep":          `deepArray().flat(1e6)`,
	"parse reviver":      `const d = deepArray(); JSON.parse('{"a":1,"b":2}', function (k, v) { if (k === "a") this.b = d; return v; })`,
	"join is toString":   `const a = [1]; Object.defineProperty(a, "join", { value: Array.prototype.toString }); String(a)`,
	"typed toString":     `const a = [1]; Object.defineProperty(a, "join", { value: Object.getPrototypeOf(Uint8Array.prototype).toString }); String(a)`,
	"join in toJSON":     `const d = deepArray(); let n = 0; const o = { toJSON() { return n++ < 900 ? [o] : String(d); } }; JSON.stringify(o)`,
}

const nestingPrelude = `function deepArray() { let a = []; for (let i = 0; i < 1e6; i++) a = [a]; return a; }
function deepObject() { let a = {}; for (let i = 0; i < 1e6; i++) a = { a }; return a; }
`

// TestNestingThrowsARangeError runs each case in a process of its own, as one that overflows its stack ends the process, with and without a memory budget, whose wrappers hold the nesting too: each must end quickly with a RangeError the script catches, and the runtime must keep running.
func TestNestingThrowsARangeError(t *testing.T) {
	if name := os.Getenv("NSGO_TEST_NESTING_CASE"); name != "" {
		for _, limit := range []int64{0, 256 << 20} {
			r := limited(limit)
			v, err := r.vm.RunString(nestingPrelude + `let caught; try { ` + nestingCases[name] + `; caught = "none"; } catch (e) { caught = e.name + ": " + e.message; } caught + "|" + JSON.stringify([[1], { a: [2] }]) + "|" + String([1, [2]])`)
			if err != nil {
				t.Fatalf("memory %d: %v", limit, err)
			}
			if got, want := v.String(), "RangeError: Maximum call stack size exceeded|[[1],{\"a\":[2]}]|1,2"; got != want {
				t.Fatalf("memory %d: %s, want %s", limit, got, want)
			}
		}
		return
	}
	for name := range nestingCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			start := time.Now()
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestNestingThrowsARangeError$", "-test.v")
			cmd.Env = append(os.Environ(), "NSGO_TEST_NESTING_CASE="+name)
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
