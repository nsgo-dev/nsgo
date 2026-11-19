//go:build !js

package tsx

import (
	"errors"
	goruntime "runtime"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx/internal/js"
)

// limited is a sandboxed runtime with no app whose built-ins hold limit bytes, hardened as a sandbox's are.
func limited(limit int64) *runtime {
	r := &runtime{vm: js.New(), warned: map[string]bool{}, sandbox: &sandboxed{}}
	if limit > 0 {
		r.limitMemory(limit)
	}
	r.harden()
	return r
}

// used is what r's built-ins hold now.
func (r *runtime) used() int64 {
	m := r.sandbox.memory
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.used
}

// TestMemoryStopsWhatWouldNotFit holds each built-in that allocates by a size to the budget before it allocates: none of these, which would take from a megabyte to exabytes at once, allocates, and each stops the runtime with the quota error, which the script cannot catch.
func TestMemoryStopsWhatWouldNotFit(t *testing.T) {
	for _, src := range []string{
		`new ArrayBuffer(512 * 1024 * 1024)`,
		`new ArrayBuffer(2 ** 32)`,
		`new ArrayBuffer(2 ** 52)`,
		`new ArrayBuffer({ valueOf: () => 2 ** 32 })`,
		`new Uint8Array(2 ** 32)`,
		`new Float64Array(2 ** 28)`,
		`new BigUint64Array(2 ** 28)`,
		`new Uint8Array({ length: 2 ** 32 })`,
		`new Int32Array(new Uint8Array(500_000))`,
		`new (class extends Uint16Array {})(2 ** 30)`,
		`Uint8Array.from({ length: 2 ** 32 })`,
		`new ArrayBuffer(8).transfer(2 ** 32)`,
		`new ArrayBuffer(8).transferToFixedLength(2 ** 32)`,
		`const b = new ArrayBuffer(700_000); b.slice()`,
		`const a = new Uint8Array(700_000); a.toReversed()`,
		`const a = new Uint8Array(700_000); a.slice()`,
		`const a = new Uint8Array(700_000); a.map((x) => x)`,
		`"x".repeat(2 ** 32)`,
		`"ab".repeat({ valueOf: () => 2 ** 30 })`,
		`"x".padStart(2 ** 32)`,
		`"x".padEnd(2 ** 32, "yz")`,
		`String.raw({ raw: { length: 2 ** 32 } })`,
		`new Array(2 ** 30).join("x")`,
		`Array.prototype.join.call({ length: 2 ** 32 })`,
		`new Array(1000).fill("x".repeat(100_000)).join("")`,
		`new Array(1000).fill({ toString: () => "x".repeat(100_000) }).join()`,
		`new Array(2 ** 30).fill(0)`,
		`[].fill.call({ length: 2 ** 32 }, 0)`,
		`Array.from({ length: 2 ** 30 })`,
		`Array.from("x".repeat(500_000))`,
		`Array.prototype.sort.call({ length: 2 ** 32 - 1 })`,
		`Array.prototype.toSorted.call({ length: 2 ** 32 - 2 })`,
		`Array.prototype.toReversed.call({ length: 2 ** 32 - 2 })`,
		`Array.prototype.with.call({ length: 2 ** 32 - 2 }, 0, 0)`,
		`Array.prototype.toSpliced.call({ length: 2 ** 32 - 2 })`,
		`Array.prototype.toLocaleString.call({ length: 2 ** 32 })`,
		`new Array(1000).fill("x".repeat(100_000)).toLocaleString()`,
		`new Float64Array(100_000).join()`,
		`new Uint8Array(500_000).toLocaleString()`,
		`Math.max.apply(null, { length: 2 ** 31 })`,
		`Reflect.apply(Math.max, null, { length: 2 ** 31 })`,
		`Reflect.construct(Array, { length: 2 ** 31 })`,
		`JSON.stringify(new Array(1000).fill("x".repeat(100_000)))`,
		`JSON.stringify({ a: 1 }, new Array(2 ** 31))`,
		`const s = "x".repeat(300_000); JSON.stringify(new Array(10).fill(0), (k, v) => k === "" ? v : s)`,
	} {
		r := limited(1 << 20)
		_, err := r.vm.RunString(`let caught = false; try { ` + src + ` } catch (e) { caught = true; }`)
		var interrupted *js.InterruptedError
		if !errors.As(err, &interrupted) || core.ErrorCode(r.stoppedErr()) != core.CodeQuotaExceeded {
			t.Errorf("%s: ran to %v, stopped with %v", src, err, r.stoppedErr())
		}
		if used := r.used(); used > 1<<20 {
			t.Errorf("%s: holds %d bytes", src, used)
		}
	}
}

// TestMemoryKeepsWhatFits runs what fits the budget as the originals do, holding what it keeps.
func TestMemoryKeepsWhatFits(t *testing.T) {
	r := limited(64 << 20)
	if _, err := r.vm.RunString(`globalThis.kept = [new ArrayBuffer(1 << 20), new Float64Array(1 << 17), "x".repeat(1 << 20), new Array(1 << 16).fill(0)]`); err != nil {
		t.Fatal(err)
	}
	// 1 MiB, 1 MiB, 1 MiB of ASCII and 64 Ki values.
	if used, want := r.used(), int64(3<<20+1<<16*valueSize); used < want {
		t.Errorf("holds %d bytes, want at least %d", used, want)
	}
}

// TestMemoryKeepsTheBuiltinsBehavior runs scripts in a runtime whose built-ins are held and one whose are not, which must agree.
func TestMemoryKeepsTheBuiltinsBehavior(t *testing.T) {
	for _, src := range []string{
		`const b = new ArrayBuffer(16); [b.byteLength, b instanceof ArrayBuffer, b.constructor === ArrayBuffer, ArrayBuffer.isView(new Uint8Array(b)), ArrayBuffer.name, ArrayBuffer.length, Object.getPrototypeOf(b) === ArrayBuffer.prototype]`,
		`const a = new Uint8Array([1, 2, 300]); [a.join(), a instanceof Uint8Array, a.constructor === Uint8Array, Uint8Array.BYTES_PER_ELEMENT, Uint8Array.name, Uint8Array.length, Object.getPrototypeOf(Uint8Array) === Object.getPrototypeOf(Int8Array)]`,
		`const a = new Float64Array(new Set([1.5, 2.5])); [a.length, a[1], Float64Array.from([1, 2], (x) => x * 2).join(), Int16Array.of(1, 2, 3).join()]`,
		`const b = new ArrayBuffer(8); const v = new Uint16Array(b, 2, 2); v[0] = 7; [v.length, new Uint8Array(b)[2], v.buffer === b]`,
		`class Bytes extends Uint8Array { sum() { return this.reduce((a, b) => a + b, 0); } } const x = new Bytes([1, 2, 3]); [x.sum(), x instanceof Bytes, x.slice(1) instanceof Bytes, x.map((v) => v * 2).sum(), x.subarray(1).length]`,
		`[new BigInt64Array([1n, 2n]).join(), (() => { try { new Uint8Array(new BigInt64Array(1)); } catch (e) { return e.name; } })()]`,
		`[(() => { try { ArrayBuffer(8); } catch (e) { return e.name; } })(), (() => { try { Uint8Array(8); } catch (e) { return e.name; } })(), (() => { try { new ArrayBuffer(-1); } catch (e) { return e.name; } })(), (() => { try { new Uint8Array(-1); } catch (e) { return e.name; } })()]`,
		`const b = new ArrayBuffer(8); new Uint8Array(b)[0] = 9; const t = b.transfer(16); [b.detached, t.byteLength, new Uint8Array(t)[0], new ArrayBuffer(4).transferToFixedLength().byteLength, new ArrayBuffer(10).slice(2, -2).byteLength]`,
		`const a = new Int8Array([3, 1, 2]); [a.toSorted().join(), a.toReversed().join(), a.with(0, 9).join(), a.filter((x) => x > 1).join(), a.join()]`,
		`["ab".repeat(3), "".repeat(5), "x".repeat(0), (() => { try { "x".repeat(-1); } catch (e) { return e.name; } })(), (() => { try { "x".repeat(Infinity); } catch (e) { return e.name; } })(), (() => { try { String.prototype.repeat.call(null, 1); } catch (e) { return e.name; } })(), String.prototype.repeat.call(12, 2), "é".repeat(2)]`,
		`["5".padStart(3, "0"), "5".padEnd(4, "ab"), "abc".padStart(2), "x".padStart(4), "x".padStart(5, ""), "é".padEnd(3, "ü"), String.prototype.padStart.name, String.prototype.padStart.length]`,
		`[String.raw` + "`a\\n${1}b${2}`" + `, String.raw({ raw: ["x", "y"] }, 1, 2)]`,
		`const a = [1, [2, [3]], null, undefined, "x", { toString: () => "o" }]; a.push(a); [a.join(), a.join("-"), [].join(), [,,].join(), Array.prototype.join.call({ length: 2, 0: "a", 1: "b" }, "+"), String(a), Array.prototype.join.name, Array.prototype.join.length]`,
		`[[1, 2, 3, 4].fill(0, 1, -1).join(), new Array(3).fill("z").join(), [1, 2, 3].fill(9, -2).join(), Array.prototype.fill.call({ length: 2 }, 1)[1], [1, 2].fill(5, NaN, undefined).join()]`,
		`const a = [1.5, null, "x", [2, 3], new Date(0)]; a.push(a); [a.toLocaleString(), [].toLocaleString(), (() => { try { [Object.create(null)].toLocaleString(); } catch (e) { return e.name; } })(), new Float64Array([1.5, -2]).join(";"), new Uint8Array([1, 2]).join(), new Int16Array([1000, -3]).toLocaleString(), new Uint8Array(0).join()]`,
		`[[3, 1, 2].sort().join(), [3, 1, 2].sort((a, b) => b - a).join(), [3, 1, 2].toSorted().join(), [1, 2, 3].toReversed().join(), [1, 2, 3].with(-1, 0).join(), [1, 2, 3].toSpliced(1, 1, "a", "b").join(), [1, 2].toLocaleString()]`,
		`[Array.from("abc").join(), Array.from({ length: 3 }, (_, i) => i * 2).join(), Array.from(new Set([1, 1, 2])).join(), Array.from(new Map([[1, 2]])).join(";"), (class extends Array {}).from([1]).constructor.name]`,
		`function f() { return [this.x, ...arguments].join(); } [f.apply({ x: 1 }, [2, 3]), f.apply({ x: 1 }, { length: 2, 0: "a" }), Reflect.apply(f, { x: 0 }, [1]), Reflect.construct(Array, [3]).length, Reflect.construct(Date, [0], Object).constructor === Date, Math.max.apply(null, [1, 5, 2])]`,
		`[JSON.stringify({ a: [1, "x", null], b: { toJSON: () => "j" }, c: undefined }), JSON.stringify({ a: 1, b: 2, c: { a: 3 } }, ["a", "c"]), JSON.stringify({ a: 1, b: "s" }, (k, v) => typeof v === "number" ? v * 10 : v, 2), JSON.stringify("é\n"), JSON.stringify(undefined), JSON.stringify([new Date(0)]), JSON.stringify({ k: 1 }, function (k, v) { return k === "" ? v : this === undefined; })]`,
		`[JSON.stringify.name, JSON.stringify.length, (function () {}).apply.name, Object.isFrozen(ArrayBuffer), Object.isFrozen(Uint8Array.prototype), typeof Uint8Array.fromBase64]`,
	} {
		want, err := limited(0).vm.RunString(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		got, err := limited(64 << 20).vm.RunString(src)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if g, w := got.String(), want.String(); g != w {
			t.Errorf("%s:\n got %s\nwant %s", src, g, w)
		}
	}
}

// TestMemoryReleasesWhatIsCollected frees what an allocation held once the collector frees it, so that a bundle that keeps nothing never runs out however much it allocates over time.
func TestMemoryReleasesWhatIsCollected(t *testing.T) {
	r := limited(32 << 20)
	for i := range 40 {
		if _, err := r.vm.RunString(`new ArrayBuffer(8 << 20).byteLength + "x".repeat(4 << 20).length`); err != nil {
			t.Fatalf("allocation %d: %v", i, err)
		}
		for deadline := time.Now().Add(5 * time.Second); r.used() > 12<<20; {
			if time.Now().After(deadline) {
				t.Fatalf("allocation %d: still holds %d bytes", i, r.used())
			}
			goruntime.GC()
			time.Sleep(time.Millisecond)
		}
	}
}
