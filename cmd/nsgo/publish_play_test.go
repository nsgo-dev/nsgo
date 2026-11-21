package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nsgo-dev/nsgo/cmd/nsgo/apk"
)

const (
	testPlayApp    = "dev.example.play"
	testPlayEdits  = "/androidpublisher/v3/applications/" + testPlayApp + "/edits"
	testPlayEdit   = testPlayEdits + "/edit1"
	testPlayUpload = "/upload" + testPlayEdit + "/bundles?uploadType=media"
	testPlayTrack  = testPlayEdit + "/tracks/internal"
)

func TestPlayPublishCommitsTheRequestedTrack(t *testing.T) {
	for _, tc := range []struct{ flag, id string }{
		{"internal", "internal"},
		{"internal", "qa"},
		{"closed-testers", "closed-testers"},
		{"production", "production"},
	} {
		t.Run(tc.flag+"/"+tc.id, func(t *testing.T) {
			const bundle = "example aab data"
			track := testPlayEdit + "/tracks/" + tc.id
			steps := []playExchange{{http.MethodPost, testPlayEdits, `{}`, `{"id":"edit1"}`, 0}}
			if tc.flag != tc.id {
				steps = append(steps, playExchange{http.MethodGet, testPlayTrack, "", `{"error":{"message":"track not found"}}`, http.StatusNotFound})
			}
			steps = append(steps,
				playExchange{http.MethodGet, track, "", `{"track":"` + tc.id + `","releases":[{"versionCodes":["18","9"]},{"versionCodes":["17"]}]}`, 0},
				playExchange{http.MethodPost, testPlayUpload, bundle, `{"versionCode":19}`, 0},
				playExchange{http.MethodPut, track, `{"track":"` + tc.id + `","releases":[{"versionCodes":["19"],"status":"completed"}]}`, `{"track":"` + tc.id + `"}`, 0},
				playExchange{http.MethodPost, testPlayEdit + ":commit", "", `{"id":"edit1"}`, 0},
			)
			c := mockPlay(t, steps)
			app := apk.BundleIdentity{Package: testPlayApp, VersionCode: 19}
			if err := c.upload(t.Context(), tc.flag, app, strings.NewReader(bundle), int64(len(bundle))); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPlayPublishDiscardsFailedEdits(t *testing.T) {
	const bundle = "bundle"
	const noReleases = `{"track":"internal"}`
	for _, tc := range []struct {
		name, want string
		steps      []playExchange
	}{
		{"equal version", "highest version code 20", []playExchange{{http.MethodGet, testPlayTrack, "", `{"releases":[{"versionCodes":["12","20"]},{"versionCodes":["15"]}]}`, 0}}},
		{"older version", "highest version code 21", []playExchange{{http.MethodGet, testPlayTrack, "", `{"releases":[{"versionCodes":["21"]}]}`, 0}}},
		{"wrong track", `different track "production"`, []playExchange{{http.MethodGet, testPlayTrack, "", `{"track":"production"}`, 0}}},
		{"track API error", "PERMISSION_DENIED: app access denied", []playExchange{{http.MethodGet, testPlayTrack, "", `{"error":{"status":"PERMISSION_DENIED","message":"app access denied"}}`, 403}}},
		{"upload API error", "versionCodeAlreadyUsed: version exists", []playExchange{
			{http.MethodGet, testPlayTrack, "", noReleases, 0},
			{http.MethodPost, testPlayUpload, bundle, `{"error":{"errors":[{"reason":"versionCodeAlreadyUsed"}],"message":"version exists"}}`, 403},
		}},
		{"wrong uploaded version", "returned version code 31 for bundle version code 20", []playExchange{
			{http.MethodGet, testPlayTrack, "", noReleases, 0},
			{http.MethodPost, testPlayUpload, bundle, `{"versionCode":31}`, 0},
		}},
		{"commit API error", "commit Google Play edit: HTTP 409 ABORTED: edit changed", []playExchange{
			{http.MethodGet, testPlayTrack, "", noReleases, 0},
			{http.MethodPost, testPlayUpload, bundle, `{"versionCode":20}`, 0},
			{http.MethodPut, testPlayTrack, `{"track":"internal","releases":[{"versionCodes":["20"],"status":"completed"}]}`, noReleases, 0},
			{http.MethodPost, testPlayEdit + ":commit", "", `{"error":{"status":"ABORTED","message":"edit changed"}}`, 409},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := append([]playExchange{{http.MethodPost, testPlayEdits, `{}`, `{"id":"edit1"}`, 0}}, tc.steps...)
			steps = append(steps, playExchange{http.MethodDelete, testPlayEdit, "", "", http.StatusNoContent})
			c := mockPlay(t, steps)
			app := apk.BundleIdentity{Package: testPlayApp, VersionCode: 20}
			err := c.upload(t.Context(), "internal", app, strings.NewReader(bundle), int64(len(bundle)))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("upload returned %v, want %q", err, tc.want)
			}
		})
	}
}

type playExchange struct {
	method, path, body, response string
	status                       int
}

func mockPlay(t *testing.T, steps []playExchange) playClient {
	t.Helper()
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(count.Add(1)) - 1
		if i >= len(steps) {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		want := steps[i]
		if r.Method != want.method || r.URL.RequestURI() != want.path || r.Header.Get("Authorization") != "Bearer test-access-token" {
			t.Errorf("request %d = %s %s, want authorized %s %s", i, r.Method, r.URL, want.method, want.path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != want.body {
			t.Errorf("request %d body = %q, %v; want %q", i, body, err, want.body)
		}
		if want.path == testPlayUpload && (r.ContentLength != int64(len(want.body)) || r.Header.Get("Content-Type") != "application/octet-stream") {
			t.Errorf("bundle upload did not send its binary length and content type")
		}
		if want.status != 0 {
			w.WriteHeader(want.status)
		}
		io.WriteString(w, want.response)
	}))
	t.Cleanup(func() {
		server.Close()
		if got := int(count.Load()); got != len(steps) {
			t.Errorf("Google Play received %d requests, want %d", got, len(steps))
		}
	})
	return playClient{http: server.Client(), endpoint: server.URL, token: "test-access-token"}
}

func TestPlayCredentialsSignAServiceAccountAssertion(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	const email = "test@example.iam.gserviceaccount.com"
	data, err := json.Marshal(map[string]string{
		"type": "service_account", "client_email": email, "private_key_id": "test-key-id",
		"private_key": string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKey})),
		"token_uri":   "https://untrusted.example/token",
	})
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(t.TempDir(), "account.json")
	if err := os.WriteFile(filename, data, 0o600); err != nil {
		t.Fatal(err)
	}
	account, err := readPlayAccount(filename)
	if err != nil {
		t.Fatal(err)
	}
	inline, err := readPlayAccount(string(data))
	if err != nil || !reflect.DeepEqual(inline, account) || account.TokenURL != playTokenURL {
		t.Fatal("inline and file credentials must use the same Google token endpoint")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.ParseForm() != nil || r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Error("expected an OAuth JWT bearer token request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		parts := strings.Split(r.Form.Get("assertion"), ".")
		if len(parts) != 3 {
			t.Error("invalid signed JWT")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		decode := base64.RawURLEncoding.DecodeString
		header, _ := decode(parts[0])
		payload, _ := decode(parts[1])
		signature, _ := decode(parts[2])
		var claims, tokenHeader map[string]any
		json.Unmarshal(header, &tokenHeader)
		json.Unmarshal(payload, &claims)
		if claims["iss"] != email || claims["scope"] != playScope || claims["aud"] != "http://"+r.Host || tokenHeader["kid"] != "test-key-id" || tokenHeader["alg"] != "RS256" {
			t.Errorf("unexpected JWT header or claims: %v %v", tokenHeader, claims)
		}
		digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
		if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], signature); err != nil {
			t.Error(err)
		}
		io.WriteString(w, `{"access_token":"test-access-token","token_type":"Bearer","expires_in":3600}`)
	}))
	defer server.Close()
	account.TokenURL = server.URL
	token, err := googlePlayToken(context.Background(), server.Client(), account)
	if err != nil || token != "test-access-token" {
		t.Fatalf("token exchange returned %q, %v", token, err)
	}
}

func TestPlayFindsTheProjectsStoreBundle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nsgo.yaml"), []byte("name: Example App\nid: dev.example.play\nversion: 1.2.3\nbuild:\n  output: artifacts\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := playBundlePath(dir); err == nil || !strings.Contains(err.Error(), "no App Bundle") {
		t.Fatalf("missing bundle returned %v", err)
	}
	releases := filepath.Join(dir, "artifacts", "releases")
	if err := os.MkdirAll(releases, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(releases, "example-app-1.2.3-android-universal.aab")
	if err := os.WriteFile(want, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	path, id, err := playBundlePath(dir)
	if err != nil || path != want || id != testPlayApp {
		t.Fatalf("bundle = %q, app = %q, err = %v; want %q", path, id, err, want)
	}
	if _, _, err := playBundlePath(filepath.Join(dir, "example.apk")); err == nil {
		t.Fatal("publish accepted an APK")
	}
	if err := os.WriteFile(filepath.Join(releases, "example-app-1.2.3-android-arm64.aab"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := playBundlePath(dir); err == nil || !strings.Contains(err.Error(), "multiple App Bundles") {
		t.Fatalf("ambiguous bundle lookup returned %v", err)
	}
}

func TestPlayPublishCommand(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-target", "android", "--track", "internal", "missing.aab"}, "open the App Bundle"},
		{[]string{"-target", "android"}, "requires -track"},
		{[]string{"-track", "internal"}, "-track requires -target android"},
		{[]string{"-target", "android", "-track", "internal", "-cert", "package.cert"}, "TSX signing flags cannot be used"},
		{nil, "-cert is required"},
	} {
		cmd := exec.CommandContext(t.Context(), exe, append([]string{"-test.run=^TestCLIProcess$", "--", "publish"}, tc.args...)...)
		cmd.Dir = t.TempDir()
		cmd.Env = append(os.Environ(), "NSGO_CLI_PROCESS=1", playCredentials+"=")
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), tc.want) {
			t.Errorf("publish %v exited with %v and %s, want %q", tc.args, err, out, tc.want)
		}
	}
}
