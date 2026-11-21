package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/cmd/nsgo/apk"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jwt"
)

const (
	playCredentials = "NSGO_PLAY_CREDENTIALS"
	playScope       = "https://www.googleapis.com/auth/androidpublisher"
	playTokenURL    = "https://oauth2.googleapis.com/token"
	playEndpoint    = "https://androidpublisher.googleapis.com"
)

func publishPlay(ctx context.Context, track, path string) error {
	track = strings.TrimSpace(track)
	if track == "" {
		return errors.New("publish: -target android requires -track")
	}
	path, projectID, err := playBundlePath(path)
	if err != nil {
		return err
	}
	bundle, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open the App Bundle from nsgo build -target android -channel store: %w", err)
	}
	defer bundle.Close()
	info, err := bundle.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("expected an App Bundle file at %s", rel(path))
	}
	identity, err := apk.ReadBundleIdentity(bundle, info.Size())
	if err != nil {
		return fmt.Errorf("read App Bundle %s: %w", rel(path), err)
	}
	if projectID != "" && identity.Package != projectID {
		return fmt.Errorf("App Bundle package %q differs from the project's ID %q", identity.Package, projectID)
	}
	account, err := readPlayAccount(os.Getenv(playCredentials))
	if err != nil {
		return err
	}
	client := &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	token, err := googlePlayToken(ctx, client, account)
	if err != nil {
		return err
	}
	play := playClient{http: client, endpoint: playEndpoint, token: token}
	if err := play.upload(ctx, track, identity, bundle, info.Size()); err != nil {
		return err
	}
	done("committed %s version code %d to Google Play track %s", identity.Package, identity.VersionCode, track)
	return nil
}

func playBundlePath(path string) (bundle, projectID string, err error) {
	if strings.EqualFold(filepath.Ext(path), ".aab") {
		return path, "", nil
	}
	if strings.EqualFold(filepath.Ext(path), ".apk") {
		return "", "", errors.New("Google Play publishing takes an Android App Bundle (.aab)")
	}
	p, err := load(path)
	if err != nil {
		return "", "", err
	}
	var found string
	for _, target := range []string{"android_arm64", "android_x64", "android_universal"} {
		candidate := filepath.Join(p.releaseDir(), p.releaseName(target, ".aab"))
		info, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if found != "" {
			return "", "", fmt.Errorf("multiple App Bundles in %s. Pass the .aab file to publish", rel(p.releaseDir()))
		}
		found = candidate
	}
	if found == "" {
		return "", "", fmt.Errorf("no App Bundle in %s for %s %s. Run nsgo build -target android -channel store", rel(p.releaseDir()), p.Name, p.Version)
	}
	return found, p.ID, nil
}

func readPlayAccount(value string) (*jwt.Config, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("set %s to the Google Play service account's JSON key or its file path", playCredentials)
	}
	data := []byte(value)
	if !strings.HasPrefix(value, "{") {
		var err error
		data, err = os.ReadFile(value)
		if err != nil {
			// The environment can contain a secret key. Leave its value
			// out of file errors.
			var pathErr *os.PathError
			if errors.As(err, &pathErr) {
				err = pathErr.Err
			}
			return nil, fmt.Errorf("read %s: %w", playCredentials, err)
		}
	}
	var key struct {
		Type       string `json:"type"`
		Email      string `json:"client_email"`
		PrivateKey string `json:"private_key"`
		KeyID      string `json:"private_key_id"`
	}
	if err := json.Unmarshal(data, &key); err != nil || key.Type != "service_account" || key.Email == "" || key.PrivateKey == "" {
		return nil, fmt.Errorf("%s must contain a Google service account JSON key", playCredentials)
	}
	return &jwt.Config{
		Email: key.Email, PrivateKey: []byte(key.PrivateKey), PrivateKeyID: key.KeyID,
		Scopes: []string{playScope}, TokenURL: playTokenURL,
	}, nil
}

func googlePlayToken(ctx context.Context, client *http.Client, account *jwt.Config) (string, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	token, err := account.TokenSource(ctx).Token()
	if err != nil {
		var retrieve *oauth2.RetrieveError
		if errors.As(err, &retrieve) {
			var response struct {
				Code    string `json:"error"`
				Message string `json:"error_description"`
			}
			json.Unmarshal(retrieve.Body, &response)
			err = &playAPIError{StatusCode: retrieve.Response.StatusCode, Code: response.Code, Message: response.Message}
		}
		return "", fmt.Errorf("authenticate the Google Play service account: %w", err)
	}
	if token.AccessToken == "" {
		return "", errors.New("Google OAuth returned no access token")
	}
	return token.AccessToken, nil
}

type playClient struct {
	http     *http.Client
	endpoint string
	token    string
}

type playTrack struct {
	ID       string        `json:"track"`
	Releases []playRelease `json:"releases"`
}

type playRelease struct {
	VersionCodes []string `json:"versionCodes"`
	Status       string   `json:"status"`
}

func (c playClient) upload(ctx context.Context, track string, app apk.BundleIdentity, bundle io.ReaderAt, size int64) (err error) {
	appPath := "/androidpublisher/v3/applications/" + url.PathEscape(app.Package) + "/edits"
	var edit struct {
		ID string `json:"id"`
	}
	if err := c.json(ctx, http.MethodPost, appPath, struct{}{}, &edit); err != nil {
		return fmt.Errorf("open Google Play edit: %w", err)
	}
	if edit.ID == "" {
		return errors.New("Google Play returned an edit without an ID")
	}
	editPath := appPath + "/" + url.PathEscape(edit.ID)
	defer func() {
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			c.json(cleanup, http.MethodDelete, editPath, nil, nil)
		}
	}()
	current, err := c.readTrack(ctx, editPath, track)
	if err != nil {
		return fmt.Errorf("read Google Play track %q: %w", track, err)
	}
	var highest int64
	for _, release := range current.Releases {
		for _, code := range release.VersionCodes {
			n, err := strconv.ParseInt(code, 10, 64)
			if err != nil || n <= 0 {
				return fmt.Errorf("Google Play track %q returned an invalid version code %q", track, code)
			}
			highest = max(highest, n)
		}
	}
	if app.VersionCode <= highest {
		return fmt.Errorf("bundle version code %d must exceed Google Play track %q's highest version code %d", app.VersionCode, track, highest)
	}
	var uploaded struct {
		VersionCode int64 `json:"versionCode"`
	}
	uploadPath := "/upload" + editPath + "/bundles?uploadType=media"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+uploadPath, io.NewSectionReader(bundle, 0, size))
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	if err := c.do(req, &uploaded); err != nil {
		return fmt.Errorf("upload App Bundle to Google Play: %w", err)
	}
	if uploaded.VersionCode != app.VersionCode {
		return fmt.Errorf("Google Play returned version code %d for bundle version code %d", uploaded.VersionCode, app.VersionCode)
	}
	change := playTrack{ID: current.ID, Releases: []playRelease{{VersionCodes: []string{strconv.FormatInt(uploaded.VersionCode, 10)}, Status: "completed"}}}
	if err := c.json(ctx, http.MethodPut, editPath+"/tracks/"+url.PathEscape(current.ID), change, nil); err != nil {
		return fmt.Errorf("update Google Play track %q: %w", track, err)
	}
	if err := c.json(ctx, http.MethodPost, editPath+":commit", nil, nil); err != nil {
		return fmt.Errorf("commit Google Play edit: %w", err)
	}
	return nil
}

func (c playClient) readTrack(ctx context.Context, editPath, track string) (playTrack, error) {
	var current playTrack
	path := editPath + "/tracks/"
	trackID := track
	err := c.json(ctx, http.MethodGet, path+url.PathEscape(trackID), nil, &current)
	var response *playAPIError
	if track == "internal" && errors.As(err, &response) && response.StatusCode == http.StatusNotFound {
		// Google also uses "qa" for internal testing in its track API.
		trackID = "qa"
		err = c.json(ctx, http.MethodGet, path+trackID, nil, &current)
	}
	if err != nil {
		return playTrack{}, err
	}
	if current.ID == "" {
		current.ID = trackID
	}
	if current.ID != track && (track != "internal" || current.ID != "qa") {
		return playTrack{}, fmt.Errorf("Google Play returned a different track %q", current.ID)
	}
	return current, nil
}

func (c playClient) json(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, body)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, out)
}

func (c playClient) do(req *http.Request, out any) error {
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure struct {
			Error struct {
				Status  string `json:"status"`
				Message string `json:"message"`
				Errors  []struct {
					Reason string `json:"reason"`
				} `json:"errors"`
			} `json:"error"`
		}
		json.Unmarshal(data, &failure)
		code := failure.Error.Status
		if code == "" && len(failure.Error.Errors) > 0 {
			code = failure.Error.Errors[0].Reason
		}
		return &playAPIError{StatusCode: resp.StatusCode, Code: code, Message: failure.Error.Message}
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

type playAPIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *playAPIError) Error() string {
	msg := fmt.Sprintf("HTTP %d", e.StatusCode)
	if e.Code != "" {
		msg += " " + e.Code
	}
	if e.Message != "" {
		msg += ": " + e.Message
	} else if e.Code == "" {
		msg += " " + http.StatusText(e.StatusCode)
	}
	return msg
}
