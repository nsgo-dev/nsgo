package tsx

import (
	"errors"

	"github.com/nsgo-dev/nsgo/tsx/internal/js"

	"github.com/nsgo-dev/nsgo/core"
)

// signInWithApple is nsgo's SignInWithApple, NSGo's own, on core's: available(), signIn({ fullName, email, nonce }), resolving with the credential or null when the user cancels, and credentialState(user), resolving with the state's name. Both but available are held to the signInWithApple capability, rejecting without it.
func (r *runtime) signInWithApple() js.Value {
	o := r.vm.NewObject()
	_ = o.Set("available", r.function(func(js.FunctionCall) js.Value {
		p, resolve, _ := r.newPromise()
		_ = resolve(core.Untracked(r.cx, r.cx.Env().CanSignInWithApple))
		return r.vm.ToValue(p)
	}))
	_ = o.Set("signIn", r.function(func(call js.FunctionCall) js.Value {
		if p := r.refused(core.AccessSignInWithApple); p != nil {
			return p
		}
		op := r.options(arg(call, 0))
		opts := core.AppleSignInOptions{FullName: op.bool("fullName", false), Email: op.bool("email", false), Nonce: op.str("nonce", "")}
		p, resolve, reject := r.newPromise()
		r.cx.SignInWithApple(opts, func(c core.AppleCredential, err error) {
			r.inJS(func() {
				switch {
				case err == nil:
					_ = resolve(r.appleCredential(c))
				case errors.Is(err, core.AppleSignInCancelled):
					_ = resolve(js.Null())
				default:
					_ = reject(r.goError(appleSignInFailure(err)))
				}
			})
		})
		return r.vm.ToValue(p)
	}))
	_ = o.Set("credentialState", r.function(func(call js.FunctionCall) js.Value {
		if p := r.refused(core.AccessSignInWithApple); p != nil {
			return p
		}
		user := ""
		if v := arg(call, 0); defined(v) {
			user = v.String()
		}
		p, resolve, reject := r.newPromise()
		r.cx.AppleCredentialState(user, func(s core.AppleCredentialState, err error) {
			r.inJS(func() {
				if err != nil {
					_ = reject(r.goError(appleSignInFailure(err)))
					return
				}
				_ = resolve(s.String())
			})
		})
		return r.vm.ToValue(p)
	}))
	return o
}

// appleCredential is c as JS sees it, its name's parts named as Foundation's NSPersonNameComponents names them.
func (r *runtime) appleCredential(c core.AppleCredential) js.Value {
	o := r.vm.NewObject()
	_ = o.Set("user", c.User)
	_ = o.Set("identityToken", c.IdentityToken)
	_ = o.Set("authorizationCode", c.AuthorizationCode)
	_ = o.Set("email", c.Email)
	name := r.vm.NewObject()
	for _, part := range []struct{ key, value string }{
		{"givenName", c.GivenName}, {"familyName", c.FamilyName}, {"middleName", c.MiddleName},
		{"namePrefix", c.NamePrefix}, {"nameSuffix", c.NameSuffix}, {"nickname", c.Nickname},
	} {
		_ = name.Set(part.key, part.value)
	}
	_ = o.Set("fullName", name)
	return o
}

// appleSignInFailure is err with the stable code JS reads: unavailable is ERR_FEATURE_UNAVAILABLE_ON_PLATFORM.
func appleSignInFailure(err error) error {
	if errors.Is(err, core.AppleSignInUnavailable) {
		return core.Unavailable(err.Error())
	}
	return err
}
