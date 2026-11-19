package tsx

import (
	"errors"
	"fmt"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"
	"github.com/nsgo-dev/nsgo/tsx/pkg"

	"github.com/nsgo-dev/nsgo/core"
)

// View runs the app of a package a pkg.Trust verified where it is placed in a Go app, sandboxed: the app's own screens delivered anew, or a plugin. The package's capabilities hold it, narrowed by Grant; it keeps a partition of the app's storage of its app ID's, and its host module reaches Host alone. A package that throws as it loads, or stops at a quota, panics, as any component does; place it under a core.Catch to show something else. Export names a component exported by the package's entry module; when set, Props are passed to it and the entry-mounted root is not selected. The web cannot isolate a package from the page, so there it panics with ErrUnisolated unless Sandbox.WebUnisolated.
//
//	p, err := trust.Verify(data) // off the engine's goroutine
//	...
//	core.Catch{Try: tsx.View{Package: p}.Render, Fallback: failed}
type View struct {
	Package *pkg.Package
	// Export selects a named component exported by the package's entry module.
	// An empty name keeps the entry-mounted root.
	Export string
	// Props are passed to Export as its props object.
	Props map[string]any
	// Sandbox's Capabilities are the package's and its Storage its app
	// ID when unset.
	Sandbox
}

// Render implements core.Node.
func (v View) Render(cx *core.Scope) core.Node {
	if v.Package == nil {
		panic("tsx: a View needs a package")
	}
	return Sandboxed(v.Package.App.ID, v.Package.Source(), v.Sandbox.of(v.Package), ViewOptions{Export: v.Export, Props: v.Props})(cx)
}

// of is s for package p.
func (s Sandbox) of(p *pkg.Package) Sandbox {
	if s.Capabilities == nil {
		s.Capabilities = &p.Capabilities
	}
	if s.Storage == "" {
		s.Storage = p.App.ID
	}
	return s
}

// Plugin is a package that gives the Go app functions rather than a view:
// what its entry module exports, which Call calls. It runs sandboxed as
// View does, under the scope that loaded it, until Close or that scope's
// end.
//
//	// search.tsx
//	export async function search(query: string) { ... }
//
//	plugin, err := tsx.Load(cx, p, tsx.Sandbox{})
//	plugin.Call("search", []any{"query"}, func(result any, err error) { ... })
type Plugin struct {
	r       *runtime
	cx      *core.Scope
	exports *js.Object
}

// ErrNoExport is the failure of a call of a name the plugin does not
// export as a function.
var ErrNoExport = errors.New("tsx: the plugin exports no such function")

// ErrNoViewExport is the failure of a View.Export name the package does not
// export.
var ErrNoViewExport = errors.New("tsx: the view exports no such component")

// ErrViewExportNotComponent is the failure of a View.Export value that is not
// callable as a component.
var ErrViewExportNotComponent = errors.New("tsx: the view export is not a component")

// Load runs package p as a plugin under cx, within s as View runs it, and returns it once its entry module ran, or a nil Plugin and what that threw, which goes to no Catch above cx and not to the app's report, or ErrUnisolated on the web as View panics with it. Call it on the engine's goroutine.
func Load(cx *core.Scope, p *pkg.Package, s Sandbox) (plugin *Plugin, err error) {
	s = s.of(p)
	// Refused before the scope runs it, which would send the panic to a Catch rather than to the caller.
	if err := s.isolation(js.Browser); err != nil {
		return nil, err
	}
	inner := cx.Child()
	var r *runtime
	inner.Run(func() {
		// Recovered here, inside the scope's run: past it, core would send what the entry module threw to a Catch above cx or to the app's report, and Load would return nothing of it. Disposed before the run ends, so nothing the module set up runs after it failed.
		defer func() {
			if x := recover(); x != nil {
				inner.Dispose()
				if e, ok := x.(error); ok {
					err = e
				} else {
					err = fmt.Errorf("tsx: the plugin failed as it loaded: %v", x)
				}
			}
		}()
		r = s.start(inner, p.App.ID, p.Source())
	})
	if err != nil {
		return nil, err
	}
	plugin = &Plugin{r: r, cx: inner}
	plugin.exports, _ = r.vm.Get(exportsGlobal).(*js.Object)
	return plugin, nil
}

// exportsGlobal is where a bundle leaves its entry module's exports
// (bundler.Compile).
const exportsGlobal = "__nsgo_exports"

// Exports are the names of the functions the plugin exports.
func (p *Plugin) Exports() []string {
	if p.exports == nil {
		return nil
	}
	var out []string
	for _, k := range p.exports.Keys() {
		if isFunction(p.exports.Get(k)) {
			out = append(out, k)
		}
	}
	return out
}

// Call calls the plugin's export name with args, values JSON encodes, and has done hear what it returns, or what its promise resolves with, as JSON decodes it, on the engine's goroutine; err is what it threw or rejected with, a *core.Error when that carries a code, ErrNoExport, or why the plugin stopped. Call it on the engine's goroutine.
func (p *Plugin) Call(name string, args []any, done func(result any, err error)) {
	r := p.r
	if err := r.stoppedErr(); err != nil || p.cx.Disposed() {
		if err == nil {
			err = errors.New("tsx: the plugin was closed")
		}
		done(nil, err)
		return
	}
	var fn js.Value
	if p.exports != nil {
		fn = p.exports.Get(name)
	}
	if !isFunction(fn) {
		done(nil, fmt.Errorf("%w: %q", ErrNoExport, name))
		return
	}
	var once bool
	finish := func(v any, err error) {
		if !once {
			once = true
			done(v, err)
		}
	}
	p.cx.Run(func() {
		defer func() {
			if x := recover(); x != nil {
				var callErr error
				if ex, ok := x.(*js.Exception); ok {
					callErr = r.rejection(ex.Value())
				} else if e, ok := x.(error); ok {
					callErr = e
				} else {
					callErr = fmt.Errorf("%v", x)
				}
				finish(nil, callErr)
			}
		}()
		in := make([]js.Value, len(args))
		for i, a := range args {
			in[i] = r.vm.ToValue(a)
		}
		v := r.call(fn, in...)
		if _, ok := v.Export().(*js.Promise); !ok {
			finish(exportJSON(v), nil)
			return
		}
		then, _ := js.AssertFunction(v.(*js.Object).Get("then"))
		onResolve := r.function(func(call js.FunctionCall) js.Value {
			finish(exportJSON(arg(call, 0)), nil)
			return js.Undefined()
		})
		onReject := r.function(func(call js.FunctionCall) js.Value {
			finish(nil, r.rejection(arg(call, 0)))
			return js.Undefined()
		})
		r.within(r.limits.Call, func() {
			if _, err := then(v, onResolve, onReject); err != nil {
				panic(err)
			}
		})
	})
}

// Close stops the plugin and frees what it holds.
func (p *Plugin) Close() { p.cx.Dispose() }

// rejection is what JS threw as a Go error: its message, with its code.
func (r *runtime) rejection(v js.Value) error {
	o, ok := v.(*js.Object)
	if !ok {
		return fmt.Errorf("%v", exportJSON(v))
	}
	msg := o.Get("message")
	if !defined(msg) {
		return fmt.Errorf("%v", v.String())
	}
	if code := o.Get("code"); defined(code) {
		return &core.Error{Code: code.String(), Message: msg.String()}
	}
	return errors.New(msg.String())
}
