//go:build darwin

package apple

import (
	"log"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Sign in with Apple for macOS, iOS and tvOS through AuthenticationServices: ASAuthorizationAppleIDProvider's request, which an ASAuthorizationController shows over the window of the surface that asked, its delegate NSGo's. The system answers only an app signed with the com.apple.developer.applesignin entitlement, which nsgo build claims when nsgo.yaml's apple.sign_in_with_apple is set; others fail with ASAuthorizationErrorUnknown.

// ASAuthorizationError's codes, from AuthenticationServices' ASAuthorizationError.h.
const (
	asAuthorizationErrorUnknown  = 1000
	asAuthorizationErrorCanceled = 1001
)

// AppleSignInAvailable is Environment.SignInWithApple: every macOS, iOS and tvOS NSGo runs on has AuthenticationServices, which loads only once a request starts.
func AppleSignInAvailable() bool { return true }

// loadAppleSignIn loads AuthenticationServices (authenticationServicesPath, which auth sessions share) and reports whether it has Sign in with Apple's classes.
var loadAppleSignIn = sync.OnceValue(func() bool {
	if _, err := purego.Dlopen(authenticationServicesPath, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		return false
	}
	return Class("ASAuthorizationAppleIDProvider") != 0 && Class("ASAuthorizationController") != 0
})

// appleSignIn is a sign-in the sheet shows, its objects retained until it answers.
type appleSignIn struct {
	id                   uint32
	provider, controller objc.ID
	delegate             objc.ID
	anchor               objc.ID
	dispatch             func(driver.Event)
}

// appleSignIns are the sign-ins showing, by their controller.
var appleSignIns struct {
	mu   sync.Mutex
	byID map[objc.ID]*appleSignIn
}

// appleSignInDelegate is the class of the controllers' delegate and presentation context provider, which answers the sign-in its controller shows.
var appleSignInDelegate = sync.OnceValue(func() objc.Class {
	var protocols []*objc.Protocol
	for _, name := range []string{"ASAuthorizationControllerDelegate", "ASAuthorizationControllerPresentationContextProviding"} {
		if p := objc.GetProtocol(name); p != nil {
			protocols = append(protocols, p)
		}
	}
	return RegisterClass("NsgoAppleSignInDelegate", Class("NSObject"), protocols, []objc.MethodDef{
		{Cmd: sel("authorizationController:didCompleteWithAuthorization:"), Fn: func(_ objc.ID, _ objc.SEL, controller, authorization objc.ID) {
			ev := driver.Event{Kind: driver.EventSignInWithApple, Text: driver.AppleSignInFailed}
			if c := authorization.Send(sel("credential")); c != 0 && objc.Send[bool](c, sel("isKindOfClass:"), Class("ASAuthorizationAppleIDCredential")) {
				ev.Text, ev.Data = "", appleCredential(c)
			}
			finishAppleSignIn(controller, ev)
		}},
		{Cmd: sel("authorizationController:didCompleteWithError:"), Fn: func(_ objc.ID, _ objc.SEL, controller, err objc.ID) {
			finishAppleSignIn(controller, driver.Event{Kind: driver.EventSignInWithApple, Text: appleSignInError(objc.Send[int](err, sel("code")))})
		}},
		{Cmd: sel("presentationAnchorForAuthorizationController:"), Fn: func(_ objc.ID, _ objc.SEL, controller objc.ID) objc.ID {
			appleSignIns.mu.Lock()
			defer appleSignIns.mu.Unlock()
			if s := appleSignIns.byID[controller]; s != nil {
				return s.anchor
			}
			return 0
		}},
	})
})

// HandleAppleSignIn carries out an ItemSignInWithApple, its sheet shown over anchor, the window of the surface that asked, or an ItemAppleCredentialState, on the main thread, answering through dispatch.
func HandleAppleSignIn(it *driver.MountItem, anchor objc.ID, dispatch func(driver.Event)) {
	id := it.Count
	if !loadAppleSignIn() {
		dispatch(driver.Event{Kind: driver.EventSignInWithApple, Count: id, Text: driver.AppleSignInUnavailable})
		return
	}
	provider := objc.ID(Class("ASAuthorizationAppleIDProvider")).Send(sel("new"))
	if it.Kind == driver.ItemAppleCredentialState {
		block := objc.NewBlock(func(_ objc.Block, state int, err objc.ID) {
			ev := driver.Event{Kind: driver.EventSignInWithApple, Count: id}
			if err != 0 {
				ev.Text = driver.AppleSignInFailed
			} else if ev.Index = appleCredentialState(state); ev.Index == 0 {
				ev.Text = driver.AppleSignInFailed
			}
			OnMain(func() { provider.Send(selRelease) })
			dispatch(ev)
		})
		provider.Send(sel("getCredentialStateForUserID:completion:"), NSString(it.Value), block)
		block.Release()
		return
	}
	r := it.AppleSignIn
	request := provider.Send(sel("createRequest"))
	scopes := objc.ID(Class("NSMutableArray")).Send(sel("array"))
	if r.FullName {
		if s := Global(authenticationServicesPath, "ASAuthorizationScopeFullName"); s != 0 {
			scopes.Send(sel("addObject:"), s)
		}
	}
	if r.Email {
		if s := Global(authenticationServicesPath, "ASAuthorizationScopeEmail"); s != 0 {
			scopes.Send(sel("addObject:"), s)
		}
	}
	request.Send(sel("setRequestedScopes:"), scopes)
	if r.Nonce != "" {
		request.Send(sel("setNonce:"), NSString(r.Nonce))
	}
	requests := objc.ID(Class("NSArray")).Send(sel("arrayWithObject:"), request)
	controller := objc.ID(Class("ASAuthorizationController")).Send(sel("alloc")).Send(sel("initWithAuthorizationRequests:"), requests)
	delegate := objc.ID(appleSignInDelegate()).Send(sel("new"))
	appleSignIns.mu.Lock()
	if appleSignIns.byID == nil {
		appleSignIns.byID = map[objc.ID]*appleSignIn{}
	}
	appleSignIns.byID[controller] = &appleSignIn{id: id, provider: provider, controller: controller, delegate: delegate, anchor: anchor, dispatch: dispatch}
	appleSignIns.mu.Unlock()
	// The controller holds its delegate and provider weakly; the entry keeps them until it answers.
	controller.Send(sel("setDelegate:"), delegate)
	controller.Send(sel("setPresentationContextProvider:"), delegate)
	controller.Send(sel("performRequests"))
}

// finishAppleSignIn answers the sign-in controller shows with ev, once, and lets its objects go.
func finishAppleSignIn(controller objc.ID, ev driver.Event) {
	appleSignIns.mu.Lock()
	s := appleSignIns.byID[controller]
	delete(appleSignIns.byID, controller)
	appleSignIns.mu.Unlock()
	if s == nil {
		return
	}
	ev.Count = s.id
	s.dispatch(ev)
	// The controller is still calling its delegate: let both go after it returns.
	OnMain(func() {
		s.controller.Send(selRelease)
		s.delegate.Send(selRelease)
		s.provider.Send(selRelease)
	})
}

// appleCredential reads an ASAuthorizationAppleIDCredential; the token and the code are UTF-8 NSData, and the email and the name are there only on a first sign-in.
func appleCredential(c objc.ID) *driver.AppleCredential {
	out := &driver.AppleCredential{
		User:              GoString(c.Send(sel("user"))),
		IdentityToken:     string(Bytes(c.Send(sel("identityToken")))),
		AuthorizationCode: string(Bytes(c.Send(sel("authorizationCode")))),
		Email:             GoString(c.Send(sel("email"))),
	}
	if name := c.Send(sel("fullName")); name != 0 {
		part := func(key string) string { return GoString(name.Send(sel(key))) }
		out.GivenName, out.FamilyName, out.MiddleName = part("givenName"), part("familyName"), part("middleName")
		out.NamePrefix, out.NameSuffix, out.Nickname = part("namePrefix"), part("nameSuffix"), part("nickname")
	}
	return out
}

// appleSignInError answers a failed request by its ASAuthorizationError code: a cancel is cancelled, anything else failed, an unknown error explained, as it is what an app without the entitlement or a device without an Apple Account gets.
func appleSignInError(code int) string {
	switch code {
	case asAuthorizationErrorCanceled:
		return driver.AppleSignInCancelled
	case asAuthorizationErrorUnknown:
		log.Printf("nsgo: Sign in with Apple failed (ASAuthorizationErrorUnknown): the app needs the com.apple.developer.applesignin entitlement (nsgo.yaml's apple.sign_in_with_apple) and the device an Apple Account")
	}
	return driver.AppleSignInFailed
}

// appleCredentialState is driver's state of ASAuthorizationAppleIDProviderCredentialState's, 0 for one it does not know.
func appleCredentialState(state int) int {
	switch state {
	case 0: // ASAuthorizationAppleIDProviderCredentialRevoked
		return driver.AppleCredentialRevoked
	case 1: // ASAuthorizationAppleIDProviderCredentialAuthorized
		return driver.AppleCredentialAuthorized
	case 2: // ASAuthorizationAppleIDProviderCredentialNotFound
		return driver.AppleCredentialNotFound
	case 3: // ASAuthorizationAppleIDProviderCredentialTransferred
		return driver.AppleCredentialTransferred
	}
	return 0
}
