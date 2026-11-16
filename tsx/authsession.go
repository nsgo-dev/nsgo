package tsx

import (
	"errors"
	"time"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"

	"github.com/nsgo-dev/nsgo/core"
)

// authSession is AuthSession on core's AuthSession: start(url, options) gives a session whose result resolves with the callback URL, or null once the user or cancel() cancels it, and rejects with an Error whose code is unavailable, timeout or failed. The authSession capability grants it.
func (r *runtime) authSession() js.Value {
	o := r.vm.NewObject()
	_ = o.Set("start", r.function(func(call js.FunctionCall) js.Value {
		build := arg(call, 0)
		if _, ok := js.AssertFunction(build); !ok {
			panic(r.vm.NewTypeError("nsgo: AuthSession.start takes a function building the authorization URL"))
		}
		op := r.options(arg(call, 1))
		p, resolve, reject := r.vm.NewPromise()
		session := r.vm.NewObject()
		_ = session.Set("result", p)
		cancel := func() {}
		if err := r.checkAccess(core.AccessRequest{Permission: core.AccessAuthSession}); err != nil {
			_ = reject(r.goError(err))
		} else {
			opts := core.AuthSessionOptions{Scheme: op.str("scheme", ""), Host: op.str("host", ""), Path: op.str("path", ""),
				Ephemeral: op.bool("ephemeral", false), Timeout: time.Duration(op.float("timeout", 0) * float64(time.Millisecond))}
			cancel = r.cx.AuthSession(func(redirect string) string {
				return r.call(build, r.vm.ToValue(redirect)).String()
			}, opts, func(callback string, err error) {
				r.inJS(func() {
					var code core.AuthError
					switch {
					case err == nil:
						_ = resolve(callback)
					case errors.Is(err, core.AuthCancelled):
						_ = resolve(js.Null())
					default:
						e := r.goError(err)
						if errors.As(err, &code) {
							_ = e.Set("code", string(code))
						}
						_ = reject(e)
					}
				})
			})
		}
		_ = session.Set("cancel", r.function(func(js.FunctionCall) js.Value {
			cancel()
			return js.Undefined()
		}))
		return session
	}))
	return o
}
