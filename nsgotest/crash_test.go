package nsgotest_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/crash"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestCrashReportsUI(t *testing.T) {
	t.Parallel()

	var uploaded []crash.CrashReport
	root := func(cx *core.Scope) core.Node {
		initial, _ := crash.Pending(cx)
		pending := core.NewSignalFunc(cx, initial, nil)
		return core.View{Children: []core.Node{
			core.Show{
				When: func() bool { return len(pending.Get()) > 0 },
				Then: func(cx *core.Scope) core.Node {
					return core.View{Children: []core.Node{
						core.Text{ContentFunc: func() string {
							list := pending.Get()
							if len(list) == 0 {
								return ""
							}
							return fmt.Sprintf("Crash detected (%d): %s", len(list), list[0].Kind)
						}},
						core.View{
							Role: core.RoleButton,
							OnPress: func(core.PressEvent) {
								for _, r := range pending.Get() {
									uploaded = append(uploaded, r)
									_ = crash.AcknowledgeIn(cx, r.ID)
								}
								next, _ := crash.Pending(cx)
								pending.Set(next)
							},
							Children: []core.Node{core.Text{Content: "Send Report"}},
						},
					}}
				},
				Else: func(cx *core.Scope) core.Node {
					return core.Text{Content: "All clear"}
				},
			},
		}}
	}

	app := nsgotest.Start(t, root, nsgotest.CrashReports(
		crash.CrashReport{
			ID:          "crash-1",
			Kind:        crash.KindGoFatal,
			AppVersion:  "1.2.3",
			BuildNumber: "42",
			OS:          "darwin",
			Arch:        "arm64",
			Timestamp:   time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC),
			Trace:       "fatal error: concurrent map writes",
		},
		crash.CrashReport{
			ID:          "crash-2",
			Kind:        crash.KindNativeException,
			AppVersion:  "1.2.3",
			BuildNumber: "42",
			OS:          "darwin",
			Arch:        "arm64",
			Timestamp:   time.Date(2026, 11, 20, 13, 0, 0, 0, time.UTC),
			Trace:       "NSException: NSRangeException",
		},
	))

	app.Find("text:Crash detected (2): go_fatal")
	if got := app.PendingCrashReports(); len(got) != 2 {
		t.Fatalf("PendingCrashReports = %d, want 2", len(got))
	}

	app.Press("role:button Send Report")
	app.Find("text:All clear")

	if len(uploaded) != 2 || uploaded[0].ID != "crash-1" || uploaded[1].ID != "crash-2" {
		t.Fatalf("uploaded = %+v, want [crash-1, crash-2]", uploaded)
	}
	if got := app.PendingCrashReports(); len(got) != 0 {
		t.Fatalf("PendingCrashReports after acknowledge = %d, want 0", len(got))
	}

	// Inject a new synthetic report mid-test.
	added := app.AddCrashReport(crash.CrashReport{
		Kind:  crash.KindPanic,
		Trace: "panic: injected",
	})
	if added.ID == "" || added.Timestamp.IsZero() {
		t.Fatalf("AddCrashReport did not populate defaults: %+v", added)
	}
	if got := app.PendingCrashReports(); len(got) != 1 || got[0].ID != added.ID {
		t.Fatalf("PendingCrashReports after AddCrashReport = %+v, want [%s]", got, added.ID)
	}
}
