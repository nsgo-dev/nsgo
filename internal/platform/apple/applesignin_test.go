//go:build darwin

package apple

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Only ASAuthorizationErrorCanceled is a cancel: an unknown error, which an app without the entitlement gets, is a failure.
func TestAppleSignInErrorReadsASAuthorizationErrorCodes(t *testing.T) {
	for code, want := range map[int]string{
		1000: driver.AppleSignInFailed,    // ASAuthorizationErrorUnknown
		1001: driver.AppleSignInCancelled, // ASAuthorizationErrorCanceled
		1002: driver.AppleSignInFailed,    // ASAuthorizationErrorInvalidResponse
		1003: driver.AppleSignInFailed,    // ASAuthorizationErrorNotHandled
		1004: driver.AppleSignInFailed,    // ASAuthorizationErrorFailed
		1005: driver.AppleSignInFailed,    // ASAuthorizationErrorNotInteractive
	} {
		if got := appleSignInError(code); got != want {
			t.Errorf("appleSignInError(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestAppleCredentialStateReadsTheProvidersStates(t *testing.T) {
	for state, want := range map[int]int{
		0: driver.AppleCredentialRevoked,
		1: driver.AppleCredentialAuthorized,
		2: driver.AppleCredentialNotFound,
		3: driver.AppleCredentialTransferred,
		4: 0,
	} {
		if got := appleCredentialState(state); got != want {
			t.Errorf("appleCredentialState(%d) = %d, want %d", state, got, want)
		}
	}
}

// AuthenticationServices loads on every Mac NSGo runs on, with the classes a sign-in sends to and the delegate's protocols.
func TestAuthenticationServicesLoads(t *testing.T) {
	if !loadAppleSignIn() {
		t.Fatal("AuthenticationServices did not load")
	}
	if appleSignInDelegate() == 0 {
		t.Fatal("no delegate class")
	}
	for _, name := range []string{"ASAuthorizationScopeFullName", "ASAuthorizationScopeEmail"} {
		if Global(authenticationServicesPath, name) == 0 {
			t.Errorf("no %s", name)
		}
	}
}
