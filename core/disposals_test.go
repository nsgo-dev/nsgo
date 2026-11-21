//go:build nsgodev || nsgotest || nsgocheck

package core_test

import (
	"runtime"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/devtools"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestCausalChainOwnerDisposalToCleanups(t *testing.T) {
	driver.TraceCauses, devtools.Enabled = true, true
	defer func() {
		driver.TraceCauses, devtools.Enabled = false, false
		for _, s := range devtools.Surfaces() {
			devtools.Unregister(s)
		}
	}()

	clock := core.NewManualClock(time.Unix(0, 0))
	root := core.NewRoot(core.RootOptions{
		CheckCommits: true,
		Post:         func(func()) {},
		Clock:        clock,
		Windows:      &core.WindowOptions{Width: 400, Height: 300},
	})
	defer root.Dispose()
	appTree := core.NewTree(root, core.ApproxTextEngine{})
	appTree.SetEnvironment(core.Environment{Audio: true})

	var (
		shown       *core.Signal[bool]
		winSite     string
		playerSite  string
		cleanupSite string
		manualScope *core.Scope
		manualPlay  *core.AudioPlayer
	)
	appTree.Mount(func(app *core.Scope) core.Node {
		shown = core.NewSignal(app, true)
		manualScope = app.Child()
		manualScope.Run(func() {
			manualPlay = manualScope.NewAudioPlayer(core.AudioSource{URI: "asset:manual.mp3"}, core.AudioPlayerOptions{})
			manualScope.Cleanup(func() {})
		})
		return core.Show{When: shown.Get, Then: func(branch *core.Scope) core.Node {
			branch.OpenWindow(core.WindowOptions{Title: "Settings"}, func(*core.Scope) core.Node {
				return core.Text{Content: "settings"}
			})
			winSite = prevLine(3)
			branch.NewAudioPlayer(core.AudioSource{URI: "asset:bg.mp3"}, core.AudioPlayerOptions{})
			playerSite = prevLine(1)
			_ = core.NewHolder(branch, func(*core.Scope) core.Node { return core.View{} })
			branch.AfterFunc(time.Minute, func() {})
			_ = branch.Context()
			branch.Cleanup(func() {})
			cleanupSite = prevLine(1)
			return core.Text{Content: "branch"}
		}}
	})
	appTree.Commit()

	// Before disposal, Owners().Resources lists the active window, audio player, holder and cleanup.
	surfaces := devtools.Surfaces()
	if len(surfaces) == 0 {
		t.Fatal("no registered DevTools surface")
	}
	surf := surfaces[len(surfaces)-1]
	var resources []devtools.Resource
	var walk func(o devtools.Owner)
	walk = func(o devtools.Owner) {
		resources = append(resources, o.Resources...)
		for _, c := range o.Children {
			walk(c)
		}
	}
	walk(surf.Owners())
	hasRes := func(kind, detail string) bool {
		return slices.ContainsFunc(resources, func(r devtools.Resource) bool {
			return r.Kind == kind && (detail == "" || r.Detail == detail)
		})
	}
	if !hasRes("window", "Settings") || !hasRes("audio-player", "asset:bg.mp3") || !hasRes("holder", "") || !hasRes("cleanup", "") {
		t.Fatalf("active resources = %+v", resources)
	}

	// Flipping shown to false disposes the Show branch and cascades to its window, player, holder, timer, context and cleanup.
	shown.Set(false)
	b := appTree.Commit()
	if b == nil || b.Causes == nil || len(b.Causes.Disposals) == 0 {
		t.Fatalf("expected disposal causes on commit, got %+v", b)
	}
	var showDisp *driver.Disposal
	for i := range b.Causes.Disposals {
		if b.Causes.Disposals[i].Reason == "rerun" {
			showDisp = &b.Causes.Disposals[i]
			break
		}
	}
	if showDisp == nil {
		t.Fatalf("no rerun disposal in %+v", b.Causes.Disposals)
	}
	hasCleanup := func(d *driver.Disposal, kind, detail, site string) bool {
		return slices.ContainsFunc(d.Cleanups, func(cl driver.CleanupEvent) bool {
			return cl.Kind == kind && (detail == "" || cl.Detail == detail) && (site == "" || cl.Site == site)
		})
	}
	if !hasCleanup(showDisp, "window", "Settings", winSite) {
		t.Errorf("missing window Settings cleanup (%s) in %+v", winSite, showDisp.Cleanups)
	}
	if !hasCleanup(showDisp, "audio-player", "asset:bg.mp3", playerSite) {
		t.Errorf("missing audio-player cleanup (%s) in %+v", playerSite, showDisp.Cleanups)
	}
	if !hasCleanup(showDisp, "holder", "", "") {
		t.Errorf("missing holder cleanup in %+v", showDisp.Cleanups)
	}
	if !hasCleanup(showDisp, "timer", "", "") {
		t.Errorf("missing timer cleanup in %+v", showDisp.Cleanups)
	}
	if !hasCleanup(showDisp, "context", "", "") {
		t.Errorf("missing context cleanup in %+v", showDisp.Cleanups)
	}
	if !hasCleanup(showDisp, "cleanup", "", cleanupSite) {
		t.Errorf("missing custom cleanup (%s) in %+v", cleanupSite, showDisp.Cleanups)
	}

	// Manually closing a resource BEFORE its owner disposes removes it from Owners().Resources and does not falsely attribute it on later disposal.
	manualPlay.Close()
	resources = nil
	walk(surf.Owners())
	if hasRes("audio-player", "asset:manual.mp3") {
		t.Fatalf("manually closed audio player still shown as active: %+v", resources)
	}
	manualScope.Dispose()
	disposeLine := prevLine(1)
	history := surf.Disposals()
	if len(history) < 2 {
		t.Fatalf("expected disposal history >= 2, got %+v", history)
	}
	last := history[len(history)-1]
	if last.Reason != "dispose" || last.Site != disposeLine {
		t.Fatalf("last disposal = %+v, want reason=dispose site=%s", last, disposeLine)
	}
	if hasCleanup(&last, "audio-player", "asset:manual.mp3", "") {
		t.Errorf("manually closed player was falsely attributed to scope disposal: %+v", last.Cleanups)
	}
	if !hasCleanup(&last, "cleanup", "", "") {
		t.Errorf("expected remaining custom cleanup in %+v", last.Cleanups)
	}

	surf.ClearDisposals()
	if got := surf.Disposals(); len(got) != 0 {
		t.Fatalf("ClearDisposals left %+v", got)
	}
}

// prevLine returns file:(callerLine - delta), matching DevTools' full pathSite format.
func prevLine(delta int) string {
	_, file, n, _ := runtime.Caller(1)
	return file + ":" + strconv.Itoa(n-delta)
}
