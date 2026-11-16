package core_test

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

// authURL builds a provider's authorization URL for redirect, as an app does with its PKCE challenge and state.
func authURL(redirect string) string {
	q := url.Values{"client_id": {"app"}, "redirect_uri": {redirect}, "state": {"s1"}, "code_challenge": {"c1"}, "code_challenge_method": {"S256"}}
	return "https://provider.example/authorize?" + q.Encode()
}

// authResult is what an auth session's done heard.
type authResult struct {
	callback string
	err      error
	calls    int
}

func startAuth(t *testing.T, opts core.AuthSessionOptions, answer func(nsgotest.AuthSession) (string, error)) (*nsgotest.App, *authResult, func()) {
	t.Helper()
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	})
	app.AuthAnswer = answer
	r := &authResult{}
	cancel := scope.AuthSession(authURL, opts, func(callback string, err error) {
		r.callback, r.err = callback, err
		r.calls++
	})
	app.Settle()
	return app, r, cancel
}

func TestAuthSessionAnswersTheCallback(t *testing.T) {
	app, r, _ := startAuth(t, core.AuthSessionOptions{Scheme: "com.example.app", Path: "/oauth", Ephemeral: true}, func(s nsgotest.AuthSession) (string, error) {
		return s.Redirect + "?code=abc&state=s1", nil
	})
	if r.calls != 1 || r.err != nil || r.callback != "com.example.app:/oauth?code=abc&state=s1" {
		t.Fatalf("done heard %q, %v (%d calls)", r.callback, r.err, r.calls)
	}
	if len(app.AuthSessions) != 1 {
		t.Fatalf("sessions %v", app.AuthSessions)
	}
	s := app.AuthSessions[0]
	if s.Redirect != "com.example.app:/oauth" || !s.Ephemeral || s.URL != authURL(s.Redirect) {
		t.Fatalf("session %+v", s)
	}
}

func TestAuthSessionOnLoopback(t *testing.T) {
	app, r, _ := startAuth(t, core.AuthSessionOptions{}, func(s nsgotest.AuthSession) (string, error) {
		return s.Redirect + "?code=abc&state=s1", nil
	})
	if r.err != nil || r.callback != nsgotest.AuthLoopback+"/?code=abc&state=s1" {
		t.Fatalf("done heard %q, %v", r.callback, r.err)
	}
	if u, _ := url.Parse(app.AuthSessions[0].URL); u.Query().Get("redirect_uri") != nsgotest.AuthLoopback+"/" {
		t.Fatalf("the URL names the redirect %q", u.Query().Get("redirect_uri"))
	}
}

func TestAuthSessionCancelledByTheUser(t *testing.T) {
	_, r, _ := startAuth(t, core.AuthSessionOptions{Scheme: "app"}, func(nsgotest.AuthSession) (string, error) {
		return "", core.AuthCancelled
	})
	if r.calls != 1 || !errors.Is(r.err, core.AuthCancelled) || errors.Is(r.err, core.AuthFailed) || r.callback != "" {
		t.Fatalf("done heard %q, %v (%d calls)", r.callback, r.err, r.calls)
	}
}

func TestAuthSessionFails(t *testing.T) {
	_, r, _ := startAuth(t, core.AuthSessionOptions{Scheme: "app"}, func(nsgotest.AuthSession) (string, error) {
		return "", errors.New("the network is down")
	})
	if !errors.Is(r.err, core.AuthFailed) || errors.Is(r.err, core.AuthCancelled) || !strings.Contains(r.err.Error(), "the network is down") {
		t.Fatalf("done heard %v", r.err)
	}
}

func TestAuthSessionCancelledByTheApp(t *testing.T) {
	app, r, cancel := startAuth(t, core.AuthSessionOptions{Scheme: "app"}, nil)
	if r.calls != 0 || len(app.AuthSessions) != 1 || app.AuthSessions[0].URL == "" {
		t.Fatalf("the session did not open and wait: %d calls, %+v", r.calls, app.AuthSessions)
	}
	cancel()
	app.Settle()
	cancel()
	app.Settle()
	if r.calls != 1 || !errors.Is(r.err, core.AuthCancelled) || !app.AuthSessions[0].Cancelled {
		t.Fatalf("cancel: %v (%d calls), platform cancelled %v", r.err, r.calls, app.AuthSessions[0].Cancelled)
	}
}

func TestAuthSessionTimesOut(t *testing.T) {
	app, r, _ := startAuth(t, core.AuthSessionOptions{Timeout: time.Minute}, nil)
	app.Advance(59 * time.Second)
	if r.calls != 0 {
		t.Fatalf("ended before its timeout: %v", r.err)
	}
	app.Advance(time.Second)
	if r.calls != 1 || !errors.Is(r.err, core.AuthTimeout) || !app.AuthSessions[0].Cancelled {
		t.Fatalf("timeout: %v (%d calls)", r.err, r.calls)
	}
}

func TestAuthSessionRefusesBadOptions(t *testing.T) {
	for _, opts := range []core.AuthSessionOptions{
		{Scheme: "1app"}, {Scheme: "http"}, {Scheme: "https"}, {Scheme: "app", Host: "example.com"},
		{Host: "example.com"}, {Path: "oauth"}, {Path: "/a?b"},
	} {
		app, r, _ := startAuth(t, opts, nil)
		if r.calls != 1 || !errors.Is(r.err, core.AuthFailed) || len(app.AuthSessions) != 0 {
			t.Errorf("%+v: %v (%d calls), %d sessions", opts, r.err, r.calls, len(app.AuthSessions))
		}
	}
}

func TestAuthSessionRefusesAURLThatIsNoWebAddress(t *testing.T) {
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	})
	var err error
	scope.AuthSession(func(string) string { return "javascript:alert(1)" }, core.AuthSessionOptions{Scheme: "app"}, func(_ string, e error) { err = e })
	app.Settle()
	if !errors.Is(err, core.AuthFailed) || len(app.AuthSessions) != 1 || app.AuthSessions[0].URL != "" || !app.AuthSessions[0].Cancelled {
		t.Fatalf("%v, %+v", err, app.AuthSessions)
	}
}

func TestAuthSessionHeldToAccess(t *testing.T) {
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		cx.ProvideAccess(denyAll{})
		return core.View{}
	})
	var err error
	scope.AuthSession(authURL, core.AuthSessionOptions{Scheme: "app"}, func(_ string, e error) { err = e })
	app.Settle()
	if !errors.Is(err, core.AuthUnavailable) || len(app.AuthSessions) != 0 {
		t.Fatalf("refused: %v, %d sessions", err, len(app.AuthSessions))
	}
}
