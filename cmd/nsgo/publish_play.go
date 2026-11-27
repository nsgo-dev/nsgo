package main

import (
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
)

type playPublishOptions struct {
	track, listings, name, action string
	fraction                      *float64
	priority                      *int
	changesNotSentForReview       bool
	client                        *googlePlayClient
}

func validPlayFraction(f float64) bool { return f > 0 && f < 1 }

func (o playPublishOptions) validate() error {
	if o.fraction != nil && !validPlayFraction(*o.fraction) {
		return errors.New("publish: -rollout must be strictly between 0 and 1 (e.g. 0.1 for 10%); use complete for 100%")
	}
	if o.priority != nil && (*o.priority < 0 || *o.priority > 5) {
		return errors.New("publish: -in-app-update-priority must be between 0 and 5")
	}
	switch o.action {
	case "":
		return nil
	case "increase":
		if o.fraction == nil {
			return errors.New("publish: rollout increase requires -rollout")
		}
	case "halt", "resume", "complete":
		if o.fraction != nil {
			return errors.New("publish: only rollout increase takes -rollout")
		}
	default:
		return fmt.Errorf("publish: unknown -rollout-action %q; use increase, halt, resume or complete", o.action)
	}
	if o.listings != "" || o.name != "" || o.priority != nil {
		return errors.New("publish: -listings, -release-name and -in-app-update-priority apply only to bundle uploads")
	}
	return nil
}

func publishPlay(ctx context.Context, o playPublishOptions, path string) error {
	o.track = strings.TrimSpace(o.track)
	if o.track == "" {
		return errors.New("publish: -target android requires -track")
	}
	if err := o.validate(); err != nil {
		return err
	}
	if o.action != "" {
		p, err := load(path)
		if err != nil {
			return err
		}
		client, err := playPublishClient(ctx, o.client)
		if err != nil {
			return err
		}
		if err := updateGooglePlayRollout(ctx, client, p.ID, o); err != nil {
			return err
		}
		done("committed Google Play rollout %s on track %s for %s", o.action, o.track, p.ID)
		reportPlayReview(o.changesNotSentForReview)
		return nil
	}
	path, project, err := playBundlePath(path)
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
	if project != nil && identity.Package != project.ID {
		return fmt.Errorf("App Bundle package %q differs from the project's ID %q", identity.Package, project.ID)
	}
	notes, err := loadPlayBundleNotes(o.listings, project, identity.Package)
	if err != nil {
		return err
	}
	client, err := playPublishClient(ctx, o.client)
	if err != nil {
		return err
	}
	if err := uploadGooglePlayBundle(ctx, client, identity, bundle, info.Size(), o, notes); err != nil {
		return err
	}
	result := ""
	switch {
	case o.fraction != nil:
		result = fmt.Sprintf(" (staged rollout at %g)", *o.fraction)
	case o.track == "production" || strings.HasSuffix(o.track, ":production"):
		result = " (full rollout, 100%)"
	}
	done("committed %s version code %d to Google Play track %s%s", identity.Package, identity.VersionCode, o.track, result)
	reportPlayReview(o.changesNotSentForReview)
	return nil
}

func playPublishClient(ctx context.Context, client *googlePlayClient) (*googlePlayClient, error) {
	if client != nil {
		return client, nil
	}
	// An App Bundle can take minutes to upload, longer than store listings' 60-second requests.
	client, err := googlePlayClientFromEnv(ctx, &http.Client{
		Timeout: 5 * time.Minute,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	})
	if err != nil {
		return nil, err
	}
	if cred := client.credentialSummary(); cred != "" {
		note("Google Play auth: %s", cred)
	}
	return client, nil
}

func reportPlayReview(changesNotSentForReview bool) {
	if changesNotSentForReview {
		note("send these changes for review from Google Play Console > Publishing overview")
	}
}

func playBundlePath(path string) (string, *project, error) {
	if strings.EqualFold(filepath.Ext(path), ".aab") {
		return path, nil, nil
	}
	if strings.EqualFold(filepath.Ext(path), ".apk") {
		return "", nil, errors.New("Google Play publishing takes an Android App Bundle (.aab)")
	}
	p, err := load(path)
	if err != nil {
		return "", nil, err
	}
	var found string
	for _, target := range []string{"android_arm64", "android_x64", "android_universal"} {
		candidate := filepath.Join(p.releaseDir(), p.releaseName(target, ".aab"))
		info, err := os.Stat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if found != "" {
			return "", nil, fmt.Errorf("multiple App Bundles in %s. Pass the .aab file to publish", rel(p.releaseDir()))
		}
		found = candidate
	}
	if found == "" {
		return "", nil, fmt.Errorf("no App Bundle in %s for %s %s. Run nsgo build -target android -channel store", rel(p.releaseDir()), p.Name, p.Version)
	}
	return found, p, nil
}

func loadPlayBundleNotes(dir string, p *project, pkg string) ([]playReleaseNote, error) {
	if dir == "" && p == nil {
		return nil, nil // An explicit AAB has no associated project or listings directory.
	}
	optional := dir == "" && p.Store.Dir == ""
	dir = resolveStoreListingsDir(dir, ".", p)
	listings, err := loadStoreListings(dir, p)
	if err != nil {
		if optional && errors.Is(err, os.ErrNotExist) {
			var pathErr *os.PathError
			if errors.As(err, &pathErr) && pathErr.Path == filepath.Join(dir, "listing.yaml") {
				return nil, nil
			}
		}
		return nil, err
	}
	if id := listings.Manifest.GooglePlay.PackageName; id != "" && id != pkg {
		return nil, fmt.Errorf("Google Play listings package %q differs from the App Bundle package %q", id, pkg)
	}
	notes := listings.playReleaseNotes()
	seen := make(map[string]bool, len(notes))
	var errs []string
	for _, note := range notes {
		if !validGooglePlayLocales[note.Language] {
			return nil, fmt.Errorf("unsupported Google Play release notes locale %q", note.Language)
		}
		if seen[note.Language] {
			return nil, fmt.Errorf("duplicate Google Play release notes locale %q", note.Language)
		}
		seen[note.Language] = true
		validateOptionalLen(&errs, note.Language+"/listing.yaml", "google_play release_notes", note.Text, googlePlayReleaseNotesMax)
	}
	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n"))
	}
	return notes, nil
}

type playTrack struct {
	ID       string        `json:"track"`
	Releases []playRelease `json:"releases"`
}

type playRelease struct {
	VersionCodes        []string          `json:"versionCodes,omitempty"`
	Status              string            `json:"status"`
	Name                string            `json:"name,omitempty"`
	UserFraction        *float64          `json:"userFraction,omitempty"`
	ReleaseNotes        []playReleaseNote `json:"releaseNotes,omitempty"`
	InAppUpdatePriority *int              `json:"inAppUpdatePriority,omitempty"`
	CountryTargeting    json.RawMessage   `json:"countryTargeting,omitempty"`
}

type playReleaseNote struct {
	Language string `json:"language"`
	Text     string `json:"text"`
}

func (r playRelease) staged() bool {
	// An already completed release can also be halted, but has no fraction.
	return r.Status == "inProgress" || (r.Status == "halted" && r.UserFraction != nil)
}

// uploadGooglePlayBundle checks the track before uploading, then commits the
// bundle and its release notes in the same edit.
func uploadGooglePlayBundle(ctx context.Context, c *googlePlayClient, app apk.BundleIdentity, bundle io.ReaderAt, size int64, o playPublishOptions, notes []playReleaseNote) (err error) {
	if err := o.validate(); err != nil {
		return err
	}
	editID, editPath, err := c.openEdit(ctx, app.Package)
	if err != nil {
		return err
	}
	defer c.discardFailedEdit(ctx, editPath, &err)
	current, err := readGooglePlayTrack(ctx, c, editPath, o.track)
	if err != nil {
		return err
	}
	var highest int64
	for _, release := range current.Releases {
		if o.fraction != nil && release.staged() {
			return fmt.Errorf("Google Play track %q already has a staged release; finish it before starting another", o.track)
		}
		for _, code := range release.VersionCodes {
			n, err := strconv.ParseInt(code, 10, 64)
			if err != nil || n <= 0 {
				return fmt.Errorf("Google Play track %q returned an invalid version code %q", o.track, code)
			}
			highest = max(highest, n)
		}
	}
	if app.VersionCode <= highest {
		return fmt.Errorf("bundle version code %d must exceed Google Play track %q's highest version code %d", app.VersionCode, o.track, highest)
	}
	uploaded, err := c.uploadBundle(ctx, app.Package, editID, bundle, size)
	if err != nil {
		return fmt.Errorf("upload App Bundle to Google Play: %w", err)
	}
	if uploaded != app.VersionCode {
		return fmt.Errorf("Google Play returned version code %d for bundle version code %d", uploaded, app.VersionCode)
	}
	release := playRelease{
		VersionCodes:        []string{strconv.FormatInt(uploaded, 10)},
		Status:              "completed",
		Name:                o.name,
		ReleaseNotes:        notes,
		InAppUpdatePriority: o.priority,
	}
	if o.fraction != nil {
		release.Status = "inProgress"
		release.UserFraction = o.fraction
	} else {
		current.Releases = current.Releases[:0]
	}
	current.Releases = append(current.Releases, release)
	if err := c.writeTrack(ctx, editPath, current); err != nil {
		return err
	}
	return c.commitEdit(ctx, editPath, o.changesNotSentForReview)
}

// updateGooglePlayRollout changes the only staged release in place. Other
// releases on the track, including the serving fallback, are left untouched.
func updateGooglePlayRollout(ctx context.Context, c *googlePlayClient, pkg string, o playPublishOptions) (err error) {
	if err := o.validate(); err != nil {
		return err
	}
	_, editPath, err := c.openEdit(ctx, pkg)
	if err != nil {
		return err
	}
	defer c.discardFailedEdit(ctx, editPath, &err)
	current, err := readGooglePlayTrack(ctx, c, editPath, o.track)
	if err != nil {
		return err
	}
	var staged *playRelease
	for i := range current.Releases {
		if !current.Releases[i].staged() {
			continue
		}
		if staged != nil {
			return fmt.Errorf("Google Play track %q has multiple staged releases", o.track)
		}
		staged = &current.Releases[i]
	}
	if staged == nil {
		return fmt.Errorf("Google Play track %q has no staged release", o.track)
	}
	if staged.UserFraction == nil || !validPlayFraction(*staged.UserFraction) {
		return fmt.Errorf("Google Play track %q returned a staged release without a valid user fraction", o.track)
	}
	if o.action == "resume" {
		if staged.Status != "halted" {
			return errors.New("only a halted staged rollout can be resumed")
		}
		staged.Status = "inProgress"
	} else {
		if staged.Status != "inProgress" {
			return fmt.Errorf("cannot %s a halted rollout; resume it first", o.action)
		}
		switch o.action {
		case "increase":
			if *o.fraction <= *staged.UserFraction {
				return fmt.Errorf("rollout fraction %g must exceed the current fraction %g", *o.fraction, *staged.UserFraction)
			}
			staged.UserFraction = o.fraction
		case "halt":
			staged.Status = "halted"
		case "complete":
			staged.Status = "completed"
			staged.UserFraction = nil
			staged.CountryTargeting = nil
		}
	}
	if err := c.writeTrack(ctx, editPath, current); err != nil {
		return err
	}
	return c.commitEdit(ctx, editPath, o.changesNotSentForReview)
}

func (c *googlePlayClient) openEdit(ctx context.Context, pkg string) (id, path string, err error) {
	appPath := "/androidpublisher/v3/applications/" + url.PathEscape(pkg) + "/edits"
	var edit struct {
		ID string `json:"id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, appPath, struct{}{}, &edit); err != nil {
		return "", "", fmt.Errorf("open Google Play edit: %w", err)
	}
	if edit.ID == "" {
		return "", "", errors.New("Google Play returned an edit without an ID")
	}
	return edit.ID, appPath + "/" + url.PathEscape(edit.ID), nil
}

func (c *googlePlayClient) discardFailedEdit(ctx context.Context, editPath string, err *error) {
	if *err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = c.doJSON(cleanup, http.MethodDelete, editPath, nil, nil)
	}
}

func (c *googlePlayClient) commitEdit(ctx context.Context, editPath string, changesNotSentForReview bool) error {
	path := editPath + ":commit"
	if changesNotSentForReview {
		path += "?changesNotSentForReview=true"
	}
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil); err != nil {
		return fmt.Errorf("commit Google Play edit: %w", err)
	}
	return nil
}

func (c *googlePlayClient) writeTrack(ctx context.Context, editPath string, track playTrack) error {
	if err := c.doJSON(ctx, http.MethodPut, editPath+"/tracks/"+url.PathEscape(track.ID), track, nil); err != nil {
		return fmt.Errorf("update Google Play track %q: %w", track.ID, err)
	}
	return nil
}

func readGooglePlayTrack(ctx context.Context, c *googlePlayClient, editPath, track string) (playTrack, error) {
	var current playTrack
	path := editPath + "/tracks/"
	trackID := track
	err := c.doJSON(ctx, http.MethodGet, path+url.PathEscape(trackID), nil, &current)
	var response *googlePlayError
	if track == "internal" && errors.As(err, &response) && response.StatusCode == http.StatusNotFound {
		// Google also uses "qa" for internal testing in its track API.
		trackID = "qa"
		err = c.doJSON(ctx, http.MethodGet, path+trackID, nil, &current)
	}
	if err != nil {
		return playTrack{}, fmt.Errorf("read Google Play track %q: %w", track, err)
	}
	if current.ID == "" {
		current.ID = trackID
	}
	if current.ID != track && (track != "internal" || current.ID != "qa") {
		return playTrack{}, fmt.Errorf("Google Play returned a different track %q", current.ID)
	}
	return current, nil
}
