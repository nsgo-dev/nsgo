package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type appStoreKey struct {
	file, id, issuer string
	key              *ecdsa.PrivateKey
}

func readAppStoreKey() (appStoreKey, error) {
	file, id, issuer := os.Getenv(notaryKeyEnv[0]), os.Getenv(notaryKeyEnv[1]), os.Getenv(notaryKeyEnv[2])
	if file == "" || strings.TrimSpace(id) == "" || strings.TrimSpace(issuer) == "" {
		return appStoreKey{}, fmt.Errorf("App Store Connect needs %s, %s and %s", notaryKeyEnv[0], notaryKeyEnv[1], notaryKeyEnv[2])
	}
	file, err := filepath.Abs(file)
	if err != nil {
		return appStoreKey{}, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return appStoreKey{}, fmt.Errorf("read %s: %w", notaryKeyEnv[0], err)
	}
	block, _ := pem.Decode(data)
	if block != nil && block.Type == "PRIVATE KEY" {
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if key, ok := parsed.(*ecdsa.PrivateKey); err == nil && ok && key.Curve == elliptic.P256() {
			return appStoreKey{file: file, id: id, issuer: issuer, key: key}, nil
		}
	}
	return appStoreKey{}, fmt.Errorf("%s must name an App Store Connect P-256 .p8 private key", notaryKeyEnv[0])
}

func (k appStoreKey) token(now time.Time) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "ES256", "kid": k.id, "typ": "JWT"})
	claims, _ := json.Marshal(struct {
		Issuer   string `json:"iss"`
		IssuedAt int64  `json:"iat"`
		Expires  int64  `json:"exp"`
		Audience string `json:"aud"`
	}{k.issuer, now.Unix(), now.Add(2 * time.Minute).Unix(), "appstoreconnect-v1"})
	encode := base64.RawURLEncoding.EncodeToString
	message := encode(header) + "." + encode(claims)
	digest := sha256.Sum256([]byte(message))
	r, s, err := ecdsa.Sign(rand.Reader, k.key, digest[:])
	if err != nil {
		return "", fmt.Errorf("sign App Store Connect token: %w", err)
	}
	var signature [64]byte
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return message + "." + encode(signature[:]), nil
}

type appStoreClient struct {
	origin string
	http   *http.Client
	key    appStoreKey
}

type appStoreList struct {
	Data []struct {
		ID         string `json:"id"`
		Attributes struct {
			BundleID string `json:"bundleId"`
		} `json:"attributes"`
	} `json:"data"`
	Errors []struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
	} `json:"errors"`
}

func (s appStoreClient) list(ctx context.Context, resource string, query url.Values) (appStoreList, error) {
	var result appStoreList
	token, err := s.key.token(time.Now())
	if err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.origin+"/v1/"+resource+"?"+query.Encode(), nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.http.Do(req)
	if err != nil {
		return result, fmt.Errorf("App Store Connect list %s: %w", resource, err)
	}
	defer resp.Body.Close()
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return result, fmt.Errorf("App Store Connect refused the API key (HTTP %d), check the key ID, issuer and app permissions", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		message := ""
		if len(result.Errors) > 0 {
			message = strings.TrimSpace(result.Errors[0].Title + " " + result.Errors[0].Detail)
		}
		if message != "" {
			return result, fmt.Errorf("App Store Connect list %s (HTTP %d): %s", resource, resp.StatusCode, message)
		}
		return result, fmt.Errorf("App Store Connect list %s: HTTP %d", resource, resp.StatusCode)
	}
	if decodeErr != nil {
		return result, fmt.Errorf("read App Store Connect %s: %w", resource, decodeErr)
	}
	if result.Data == nil {
		return result, errors.New("App Store Connect returned no result list")
	}
	return result, nil
}

func (s appStoreClient) checkBuild(ctx context.Context, build appleUpload) error {
	apps, err := s.list(ctx, "apps", url.Values{
		"filter[bundleId]": {build.bundle},
		"fields[apps]":     {"bundleId"},
		"limit":            {"2"},
	})
	if err != nil {
		return err
	}
	if len(apps.Data) != 1 || apps.Data[0].Attributes.BundleID != build.bundle || apps.Data[0].ID == "" {
		return fmt.Errorf("App Store Connect has no matching app for %s visible to the API key", build.bundle)
	}
	query := url.Values{
		"filter[app]":                        {apps.Data[0].ID},
		"filter[version]":                    {build.build},
		"filter[preReleaseVersion.platform]": {build.platform},
		"filter[processingState]":            {"PROCESSING,VALID"},
		"fields[builds]":                     {"version"},
		"limit":                              {"1"},
	}
	// macOS build numbers are shared across all versions of an app. iOS
	// starts a new sequence for each app version.
	if build.platform == "IOS" {
		query.Set("filter[preReleaseVersion.version]", build.version)
	}
	builds, err := s.list(ctx, "builds", query)
	if err != nil {
		return err
	}
	if len(builds.Data) != 0 {
		return fmt.Errorf("App Store Connect already has build %s for %s (%s), increment build_number and build the package again", build.build, build.bundle, build.platform)
	}
	return nil
}
