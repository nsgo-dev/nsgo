//go:build !js

package tsx

import (
	"fmt"
	"math"
	"reflect"
	goruntime "runtime"
	"slices"
	"strconv"
	"sync"
	"weak"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx/internal/js"
)

// A sandbox's memory budget (Limits.Memory). goja allocates from Go's heap, which the host and every other runtime share, so a sample of that heap cannot tell what one bundle holds, and one allocation can take more than the system has before any sample sees it. Instead the built-ins that allocate by a size the bundle gives them, a length, a count or a length's worth of elements, are replaced by ones that check the budget before they call the original, so that what would not fit is never allocated, and charge what the original made until the Go collector frees it. The bundle's other allocations, its objects, closures, the arrays it builds or spreads and the strings it concatenates, are not counted: goja gives no hook for them, and they cost the bundle time as they grow, which Limits.Call bounds per call.

// memory is what the built-ins hold for a sandbox.
type memory struct {
	limit int64
	mu    sync.Mutex
	used  int64
	// held is what each charged allocation holds, by a weak pointer to it: the first byte of a buffer's or a string's memory, or an array's object. A cleanup on it releases what it holds.
	held map[any]int64
	// joining are the arrays Array.prototype.join is joining, outermost first, as goja's own join keeps them, so that an array in itself joins as "".
	joining []*js.Object
}

const (
	// valueSize is what a value in an array or a list of arguments takes: a Go interface.
	valueSize = 16
	// unitSize is the most a string's UTF-16 code unit takes: goja keeps an ASCII string in a byte each.
	unitSize = 2
	// numberLen is the most UTF-16 code units a number takes as a string, a locale's grouping included.
	numberLen = 32
	// heldMin is the least an allocation is charged from. Smaller ones cost no more than the objects goja makes uncounted, and Go may batch tiny ones with others so that they are never freed alone.
	heldMin = 1 << 10
)

// limitMemory holds the built-ins that allocate by size to limit bytes, before the bundle runs and hardening freezes them.
func (r *runtime) limitMemory(limit int64) {
	r.sandbox.memory = &memory{limit: limit, held: map[any]int64{}}
	g := r.vm.GlobalObject()
	r.wrapConstructor("ArrayBuffer", 0)
	for _, name := range []string{"Uint8Array", "Uint8ClampedArray", "Int8Array", "Uint16Array", "Int16Array", "Uint32Array", "Int32Array", "Float32Array", "Float64Array", "BigInt64Array", "BigUint64Array"} {
		r.wrapConstructor(name, g.Get(name).(*js.Object).Get("BYTES_PER_ELEMENT").ToInteger())
	}

	buffer := object(g, "ArrayBuffer", "prototype")
	r.wrap(buffer, "slice", func(call js.FunctionCall, orig js.Callable) js.Value {
		r.checkMemory(int64(len(r.bytesOf(call.This))))
		return r.holdBytes(r.callOrig(orig, call.This, call.Arguments))
	})
	transfer := func(call js.FunctionCall, orig js.Callable) js.Value {
		args := slices.Clone(call.Arguments)
		grows := false
		if len(args) > 0 && !js.IsUndefined(args[0]) {
			args[0] = args[0].ToNumber()
			n := lengthOf(args[0])
			if grows = n > int64(len(r.bytesOf(call.This))); grows {
				r.checkMemory(n)
			}
		}
		v := r.callOrig(orig, call.This, args)
		if grows {
			r.holdBytes(v)
		}
		return v
	}
	r.wrap(buffer, "transfer", transfer)
	r.wrap(buffer, "transferToFixedLength", transfer)
	typed := object(g, "Uint8Array", "prototype").Prototype()
	for _, name := range []string{"slice", "map", "filter", "toReversed", "toSorted", "with"} {
		r.wrap(typed, name, func(call js.FunctionCall, orig js.Callable) js.Value {
			r.checkMemory(int64(len(r.bytesOf(call.This))))
			return r.holdBytes(r.callOrig(orig, call.This, call.Arguments))
		})
	}
	// A typed array's join and toLocaleString write each number in at most numberLen units.
	for _, name := range []string{"join", "toLocaleString"} {
		r.wrap(typed, name, func(call js.FunctionCall, orig js.Callable) js.Value {
			args := slices.Clone(call.Arguments)
			sepLen := int64(1)
			if name == "join" && len(args) > 0 && !js.IsUndefined(args[0]) {
				args[0] = args[0].ToString()
				sepLen = stringLen(args[0])
			}
			r.checkMemory(mul(r.elementsOf(call.This), numberLen+sepLen, unitSize))
			return r.holdString(r.callOrig(orig, call.This, args))
		})
	}

	str := object(g, "String", "prototype")
	r.wrap(str, "repeat", func(call js.FunctionCall, orig js.Callable) js.Value {
		if js.IsUndefined(call.This) || js.IsNull(call.This) {
			return r.callOrig(orig, call.This, call.Arguments)
		}
		s, count := call.This.ToString(), call.Argument(0).ToNumber()
		if f := count.ToFloat(); f < 0 || math.IsInf(f, 0) {
			// Out of range: the original throws its RangeError.
			return r.callOrig(orig, s, []js.Value{count})
		}
		r.checkMemory(mul(stringLen(s), lengthOf(count), unitSize))
		return r.holdString(r.callOrig(orig, s, []js.Value{count}))
	})
	pad := func(call js.FunctionCall, orig js.Callable) js.Value {
		if js.IsUndefined(call.This) || js.IsNull(call.This) {
			return r.callOrig(orig, call.This, call.Arguments)
		}
		s, n := call.This.ToString(), call.Argument(0).ToNumber()
		args := []js.Value{n}
		filled := true
		if fill := call.Argument(1); !js.IsUndefined(fill) {
			fill = fill.ToString()
			args = append(args, fill)
			filled = stringLen(fill) > 0
		}
		if !filled || lengthOf(n) <= stringLen(s) {
			return r.callOrig(orig, s, args)
		}
		r.checkMemory(mul(lengthOf(n), unitSize))
		return r.holdString(r.callOrig(orig, s, args))
	}
	r.wrap(str, "padStart", pad)
	r.wrap(str, "padEnd", pad)
	r.wrap(object(g, "String"), "raw", func(call js.FunctionCall, orig js.Callable) js.Value {
		if o, ok := call.Argument(0).(*js.Object); ok {
			if raw, ok := o.Get("raw").(*js.Object); ok {
				r.checkMemory(mul(r.lengthOf(raw), valueSize))
			}
		}
		return r.holdString(r.callOrig(orig, call.This, call.Arguments))
	})

	array := object(g, "Array", "prototype")
	join, _ := js.AssertFunction(array.Get("join"))
	r.wrap(array, "join", func(call js.FunctionCall, _ js.Callable) js.Value {
		o, sep := call.This.ToObject(r.vm), call.Argument(0)
		if js.IsUndefined(sep) {
			sep = r.vm.ToValue(",")
		} else {
			sep = sep.ToString()
		}
		return r.join(o, sep, js.Value.ToString, join)
	})
	r.wrap(array, "toLocaleString", func(call js.FunctionCall, _ js.Callable) js.Value {
		return r.join(call.This.ToObject(r.vm), r.vm.ToValue(","), r.localeString, join)
	})
	r.wrap(array, "fill", func(call js.FunctionCall, orig js.Callable) js.Value {
		o := call.This.ToObject(r.vm)
		n := r.lengthOf(o)
		start, end := call.Argument(1).ToNumber(), call.Argument(2)
		if !js.IsUndefined(end) {
			end = end.ToNumber()
		}
		from, to := relIndex(start, n, 0), relIndex(end, n, n)
		if to > from {
			r.checkMemory(mul(to-from, valueSize))
		}
		v := r.callOrig(orig, o, []js.Value{call.Argument(0), start, end})
		hold(r, o, mul(to, valueSize))
		return v
	})
	// sort copies the elements of what is not an array into a list first.
	r.wrap(array, "sort", func(call js.FunctionCall, orig js.Callable) js.Value {
		o := call.This.ToObject(r.vm)
		r.checkMemory(mul(r.lengthOf(o), valueSize))
		return r.callOrig(orig, o, call.Arguments)
	})
	// What makes an array as long as this one, or one longer by its arguments.
	for _, name := range []string{"toSorted", "toReversed", "toSpliced", "with"} {
		r.wrap(array, name, func(call js.FunctionCall, orig js.Callable) js.Value {
			o := call.This.ToObject(r.vm)
			r.checkMemory(mul(r.lengthOf(o)+int64(len(call.Arguments)), valueSize))
			return r.holdArray(r.callOrig(orig, o, call.Arguments))
		})
	}
	r.wrap(object(g, "Array"), "from", func(call js.FunctionCall, orig js.Callable) js.Value {
		if items := call.Argument(0); !js.IsUndefined(items) && !js.IsNull(items) {
			r.checkMemory(mul(r.lengthOf(items.ToObject(r.vm)), valueSize))
		}
		return r.holdArray(r.callOrig(orig, call.This, call.Arguments))
	})

	// What makes a list of arguments of an array-like's length.
	arguments := func(i int) func(call js.FunctionCall, orig js.Callable) js.Value {
		return func(call js.FunctionCall, orig js.Callable) js.Value {
			if o, ok := call.Argument(i).(*js.Object); ok {
				r.checkMemory(mul(r.lengthOf(o), valueSize))
			}
			return r.callOrig(orig, call.This, call.Arguments)
		}
	}
	r.wrap(object(g, "Function", "prototype"), "apply", arguments(1))
	r.wrap(object(g, "Reflect"), "apply", arguments(2))
	r.wrap(object(g, "Reflect"), "construct", arguments(1))

	isArray, _ := js.AssertFunction(object(g, "Array", "isArray"))
	r.wrap(object(g, "JSON"), "stringify", func(call js.FunctionCall, orig js.Callable) js.Value {
		return r.stringify(call, orig, isArray)
	})
}

// object is the object at path from o.
func object(o *js.Object, path ...string) *js.Object {
	for _, name := range path {
		o = o.Get(name).(*js.Object)
	}
	return o
}

// adoptProgram replaces the global constructor name with a wrapper that constructs through construct, taking the original's statics, prototype and place in the prototype chain, so that instanceof, subclasses, species and the statics find it. The wrapper is JS, as only JS tells a call from new (goja's Go constructors see neither new.target nor new).
var adoptProgram = js.MustCompile("memory.js", `(function (g, name, construct) {
	"use strict";
	const original = g[name], apply = Reflect.apply;
	const wrapper = function (...args) {
		return new.target === undefined ? apply(original, this, args) : construct(new.target, args);
	};
	for (const key of Reflect.ownKeys(original)) {
		if (key !== "prototype") Object.defineProperty(wrapper, key, Object.getOwnPropertyDescriptor(original, key));
	}
	Object.defineProperty(wrapper, "prototype", { value: original.prototype, writable: false });
	Object.defineProperty(original.prototype, "constructor", { value: wrapper });
	Object.setPrototypeOf(wrapper, Object.getPrototypeOf(original));
	Object.defineProperty(g, name, { value: wrapper });
})`, true)

// wrapConstructor replaces the global constructor name, of a typed array of elements of size bytes or, size 0, of ArrayBuffer, with one that checks what it would allocate first and charges what it made.
func (r *runtime) wrapConstructor(name string, size int64) {
	construct, _ := js.AssertConstructor(r.vm.Get(name))
	checked := r.function(func(call js.FunctionCall) js.Value {
		newTarget := call.Argument(0).(*js.Object)
		list := call.Argument(1).(*js.Object)
		args := make([]js.Value, r.lengthOf(list))
		for i := range args {
			args[i] = list.Get(strconv.Itoa(i))
		}
		allocates := true
		if len(args) > 0 {
			switch a, ok := args[0].(*js.Object); {
			case size == 0:
				// ArrayBuffer's length, read once.
				args[0] = args[0].ToNumber()
				r.checkMemory(lengthOf(args[0]))
			case ok:
				if a.ExportType() == arrayBufferType {
					// A view of a buffer allocates nothing.
					allocates = false
				} else {
					// An array-like, an iterable or another typed array, as long as its length.
					r.checkMemory(mul(r.lengthOf(a), size))
				}
			default:
				args[0] = args[0].ToNumber()
				r.checkMemory(mul(lengthOf(args[0]), size))
			}
		}
		o, err := construct(newTarget, args...)
		if err != nil {
			panic(err)
		}
		if allocates {
			r.holdBytes(o)
		}
		return o
	})
	adopt, _ := js.AssertFunction(r.run(adoptProgram))
	if _, err := adopt(js.Undefined(), r.vm.GlobalObject(), r.vm.ToValue(name), checked); err != nil {
		panic(err)
	}
}

var arrayBufferType = reflect.TypeFor[js.ArrayBuffer]()

// run runs a program of the runtime's own.
func (r *runtime) run(p *js.Program) js.Value {
	v, err := r.vm.RunProgram(p)
	if err != nil {
		panic(err)
	}
	return v
}

// wrap replaces o's method name with fn, which calls the original, keeping its name and length.
func (r *runtime) wrap(o *js.Object, name string, fn func(call js.FunctionCall, orig js.Callable) js.Value) {
	v, _ := o.Get(name).(*js.Object)
	orig, ok := js.AssertFunction(v)
	if !ok {
		return
	}
	f := r.function(func(call js.FunctionCall) js.Value { return fn(call, orig) })
	for _, key := range []string{"name", "length"} {
		_ = f.DefineDataProperty(key, v.Get(key), js.FLAG_FALSE, js.FLAG_TRUE, js.FLAG_FALSE)
	}
	_ = o.DefineDataProperty(name, f, js.FLAG_TRUE, js.FLAG_TRUE, js.FLAG_FALSE)
}

// callOrig calls a wrapped built-in, throwing what it throws.
func (r *runtime) callOrig(orig js.Callable, this js.Value, args []js.Value) js.Value {
	v, err := orig(this, args...)
	if err != nil {
		panic(err)
	}
	return v
}

// join joins the elements of o, an array or an array-like, with sep through orig, the original Array.prototype.join, as Array.prototype.join does when part is ToString and toLocaleString when it calls each element's: it makes each a string first, so that what they add up to is checked before the original allocates the whole. An array within itself joins as "", as in goja's own.
func (r *runtime) join(o *js.Object, sep js.Value, part func(e js.Value) js.Value, orig js.Callable) js.Value {
	m := r.sandbox.memory
	if slices.Contains(m.joining, o) {
		return r.vm.ToValue("")
	}
	m.joining = append(m.joining, o)
	defer func() { m.joining = m.joining[:len(m.joining)-1] }()
	n := r.lengthOf(o)
	total := sum(mul(n, valueSize), mul(n, stringLen(sep), unitSize))
	r.checkMemory(total)
	parts := make([]any, n)
	for i := range parts {
		e := o.Get(strconv.Itoa(i))
		if e == nil || js.IsUndefined(e) || js.IsNull(e) {
			parts[i] = ""
			continue
		}
		s := part(e)
		parts[i] = s
		total = sum(total, mul(stringLen(s), unitSize))
		r.checkMemory(total)
	}
	return r.holdString(r.callOrig(orig, r.vm.NewArray(parts...), []js.Value{sep}))
}

// localeString is e.toLocaleString() as a string, as Array.prototype.toLocaleString makes each element.
func (r *runtime) localeString(e js.Value) js.Value {
	f, ok := js.AssertFunction(e.ToObject(r.vm).Get("toLocaleString"))
	if !ok {
		panic(r.vm.NewTypeError("Property 'toLocaleString' of object %s is not a function", e))
	}
	return r.callOrig(f, e, nil).ToString()
}

// stringify is JSON.stringify, which checks what the strings and keys it writes add up to as it reaches them, through a replacer of its own around the bundle's, before it writes them.
func (r *runtime) stringify(call js.FunctionCall, orig, isArray js.Callable) js.Value {
	args := slices.Clone(call.Arguments)
	if rep, ok := call.Argument(1).(*js.Object); ok && r.callOrig(isArray, js.Undefined(), []js.Value{rep}).ToBoolean() {
		// A list of the properties to write: only its own length is checked.
		r.checkMemory(mul(r.lengthOf(rep), valueSize))
		return r.holdString(r.callOrig(orig, call.This, args))
	}
	user, replaces := js.AssertFunction(call.Argument(1))
	var total int64
	replacer := r.function(func(c js.FunctionCall) js.Value {
		key, v := c.Argument(0), c.Argument(1)
		if replaces {
			v = r.callOrig(user, c.This, []js.Value{key, v})
		}
		total = sum(total, valueSize, mul(stringLen(key), unitSize))
		if _, ok := v.(js.String); ok {
			total = sum(total, mul(stringLen(v), unitSize))
		}
		r.checkMemory(total)
		return v
	})
	for len(args) < 2 {
		args = append(args, js.Undefined())
	}
	args[1] = replacer
	return r.holdString(r.callOrig(orig, call.This, args))
}

// checkMemory stops the runtime when n more bytes would not fit its budget.
func (r *runtime) checkMemory(n int64) {
	m := r.sandbox.memory
	m.mu.Lock()
	over := n > m.limit-m.used
	m.mu.Unlock()
	if over {
		r.exceedMemory()
	}
}

// exceedMemory stops the runtime at its memory quota, unwinding the built-in that would go past it.
func (r *runtime) exceedMemory() {
	err := &core.Error{Code: core.CodeQuotaExceeded, Resource: "memory",
		Message: fmt.Sprintf("tsx: the sandbox's quota of %d bytes of memory is used up", r.sandbox.memory.limit)}
	r.stop(err)
	panic(r.goError(err))
}

// hold charges n bytes for what p points to, or what n is more than it was charged before, until the collector frees it, and stops the runtime past its budget.
func hold[T any](r *runtime, p *T, n int64) {
	if p == nil || n < heldMin {
		return
	}
	m := r.sandbox.memory
	key := weak.Make(p)
	m.mu.Lock()
	had, ok := m.held[key]
	if n > had {
		m.used += n - had
		m.held[key] = n
	}
	over := m.used > m.limit
	m.mu.Unlock()
	if !ok {
		goruntime.AddCleanup(p, m.release, any(key))
	}
	if over {
		r.exceedMemory()
	}
}

// release frees what the allocation key held, once it is collected.
func (m *memory) release(key any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.used -= m.held[key]
	delete(m.held, key)
}

// holdBytes charges the bytes of v, an ArrayBuffer or a typed array a built-in made, and returns it.
func (r *runtime) holdBytes(v js.Value) js.Value {
	if b := r.bytesOf(v); len(b) > 0 {
		hold(r, &b[0], int64(len(b)))
	}
	return v
}

// bytesOf is the memory of v, an ArrayBuffer or a typed array, nil for anything else.
func (r *runtime) bytesOf(v js.Value) []byte {
	o, ok := v.(*js.Object)
	if !ok {
		return nil
	}
	if t := o.ExportType(); t != arrayBufferType && (t == nil || t.Kind() != reflect.Slice || t.Elem().Kind() == reflect.Interface) {
		return nil
	}
	var b []byte
	if r.vm.ExportTo(o, &b) != nil {
		return nil
	}
	return b
}

// elementsOf is how many elements v, a typed array, has, 0 for anything else.
func (r *runtime) elementsOf(v js.Value) int64 {
	if b := r.bytesOf(v); len(b) > 0 {
		if t := v.(*js.Object).ExportType(); t != arrayBufferType {
			return int64(len(b)) / int64(t.Elem().Size())
		}
	}
	return 0
}

// holdString charges the memory of v, a string a built-in made, and returns it.
func (r *runtime) holdString(v js.Value) js.Value {
	if p, n := js.StringData(v); p != nil {
		hold(r, p, int64(n))
	}
	return v
}

// holdArray charges the values of v, an array a built-in made, and returns it.
func (r *runtime) holdArray(v js.Value) js.Value {
	if o, ok := v.(*js.Object); ok {
		hold(r, o, mul(r.lengthOf(o), valueSize))
	}
	return v
}

// lengthOf is o's length as JS's ToLength reads it.
func (r *runtime) lengthOf(o *js.Object) int64 {
	if l := o.Get("length"); l != nil {
		return lengthOf(l.ToNumber())
	}
	return 0
}

// lengthOf is a number as a length, from 0 to 2⁵³-1.
func lengthOf(v js.Value) int64 {
	switch f := v.ToFloat(); {
	case math.IsNaN(f) || f <= 0:
		return 0
	case f >= 1<<53-1:
		return 1<<53 - 1
	default:
		return int64(f)
	}
}

// relIndex is v, a number or undefined for def, as an index into n elements counted from the end when negative, as Array.prototype.fill reads it.
func relIndex(v js.Value, n, def int64) int64 {
	if js.IsUndefined(v) {
		return def
	}
	f := math.Trunc(v.ToFloat())
	switch {
	case math.IsNaN(f):
		return 0
	case f < 0:
		return int64(max(float64(n)+f, 0))
	default:
		return int64(min(f, float64(n)))
	}
}

// stringLen is the length of v, a string, in UTF-16 code units.
func stringLen(v js.Value) int64 {
	if s, ok := v.(js.String); ok {
		return int64(s.Length())
	}
	return 0
}

// mul multiplies sizes, saturating rather than overflowing.
func mul(n ...int64) int64 {
	p := int64(1)
	for _, x := range n {
		if x <= 0 {
			return 0
		}
		if p > math.MaxInt64/x {
			return math.MaxInt64
		}
		p *= x
	}
	return p
}

// sum adds sizes, saturating rather than overflowing.
func sum(n ...int64) int64 {
	s := int64(0)
	for _, x := range n {
		if s > math.MaxInt64-x {
			return math.MaxInt64
		}
		s += x
	}
	return s
}
