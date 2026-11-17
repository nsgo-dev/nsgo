package pkg

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"
)

func openTestTrust(t *testing.T, k keys, dir string) *Trust {
	t.Helper()
	trust, err := OpenTrust(dir)
	if err != nil {
		t.Fatal(err)
	}
	trust.AddRoot(k.root.Public().(ed25519.PublicKey))
	return trust
}

func TestAcceptSaveFailure(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "first build"
		if existing {
			name = "higher build"
		}
		t.Run(name, func(t *testing.T) {
			k, dir := newKeys(t), trustTestDir(t)
			trust := openTestTrust(t, k, dir)
			oldData, nextData := k.build(t, 2), k.build(t, 5)
			old, err := trust.Verify(oldData)
			if err != nil {
				t.Fatal(err)
			}
			if existing {
				if err := trust.Accept(old); err != nil {
					t.Fatal(err)
				}
			}
			next, err := trust.Verify(nextData)
			if err != nil {
				t.Fatal(err)
			}
			unblock := blockTrustWrites(t, dir)
			for attempt := range 2 {
				if err := trust.Accept(next); !errors.Is(err, ErrState) {
					t.Errorf("attempt %d: got %v, want ErrState", attempt, err)
				}
				for _, current := range []*Trust{trust, openTestTrust(t, k, dir)} {
					build, ok := current.AcceptedBuild(next.App.ID)
					if ok != existing || existing && build != old.App.Build {
						t.Errorf("after failed save: accepted build %d, %v", build, ok)
					}
					if _, err := current.Verify(oldData); err != nil {
						t.Errorf("previous build after failed save: %v", err)
					}
				}
			}
			unblock()
			if err := trust.Accept(next); err != nil {
				t.Fatalf("retry after storage recovers: %v", err)
			}
			for _, current := range []*Trust{trust, openTestTrust(t, k, dir)} {
				if build, ok := current.AcceptedBuild(next.App.ID); !ok || build != next.App.Build {
					t.Errorf("after retry: accepted build %d, %v", build, ok)
				}
				if _, err := current.Verify(oldData); !errors.Is(err, ErrRollback) {
					t.Errorf("previous build after retry: got %v, want ErrRollback", err)
				}
			}
		})
	}
}

func TestRevocationsSaveFailure(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "first list"
		if existing {
			name = "newer list"
		}
		t.Run(name, func(t *testing.T) {
			k, dir := newKeys(t), trustTestDir(t)
			trust := openTestTrust(t, k, dir)
			oldData, nextData := k.build(t, 1), k.build(t, 2)
			old, err := trust.Verify(oldData)
			if err != nil {
				t.Fatal(err)
			}
			next, err := trust.Verify(nextData)
			if err != nil {
				t.Fatal(err)
			}
			if existing {
				list, err := Revoke(k.root, 1, time.Time{}, nil, []string{old.SHA256})
				if err != nil {
					t.Fatal(err)
				}
				if err := trust.UpdateRevocations(list); err != nil {
					t.Fatal(err)
				}
			}
			list, err := Revoke(k.root, 2, time.Time{}, nil, []string{old.SHA256, next.SHA256})
			if err != nil {
				t.Fatal(err)
			}
			unblock := blockTrustWrites(t, dir)
			for attempt := range 2 {
				if err := trust.UpdateRevocations(list); !errors.Is(err, ErrState) {
					t.Errorf("attempt %d: got %v, want ErrState", attempt, err)
				}
				for _, current := range []*Trust{trust, openTestTrust(t, k, dir)} {
					if _, err := current.Verify(nextData); err != nil {
						t.Errorf("new revocation applied after failed save: %v", err)
					}
					_, err := current.Verify(oldData)
					if existing && !errors.Is(err, ErrRevoked) || !existing && err != nil {
						t.Errorf("previous revocation changed after failed save: %v", err)
					}
				}
			}
			unblock()
			if err := trust.UpdateRevocations(list); err != nil {
				t.Fatalf("retry after storage recovers: %v", err)
			}
			for _, current := range []*Trust{trust, openTestTrust(t, k, dir)} {
				if _, err := current.Verify(nextData); !errors.Is(err, ErrRevoked) {
					t.Errorf("new revocation after retry: got %v, want ErrRevoked", err)
				}
				if err := current.UpdateRevocations(list); !errors.Is(err, ErrStale) {
					t.Errorf("same list after retry: got %v, want ErrStale", err)
				}
			}
		})
	}
}
