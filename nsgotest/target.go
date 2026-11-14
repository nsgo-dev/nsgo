package nsgotest

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/automation"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/testhost"
)

// mainRan is set once Main hands the main thread to the platform, and
// opened once a test opened its app there: a process shows one.
var mainRan, opened bool

// Main runs the package's tests, from TestMain:
//
//	func TestMain(m *testing.M) { nsgotest.Main(m) }
//
// Headless it is m.Run. Under `nsgo test -target` it gives the main
// thread to the platform, as macOS, iOS and Windows need their windows'
// loop there, and runs the tests beside it.
func Main(m *testing.M) {
	if h := testhost.Current(); onTarget() && h != nil {
		mainRan = true
		h.Main(m.Run)
		return
	}
	os.Exit(m.Run())
}

// onTarget reports whether `nsgo test -target` runs the tests.
func onTarget() bool { return os.Getenv("NSGO_TEST_TARGET") != "" }

// Target is the platform `nsgo test -target` runs the tests on, such as
// "macos" or "ios_simulator", or "" headless.
func Target() string { return os.Getenv("NSGO_TEST_TARGET") }

// target drives an app shown on a platform: the model follows the
// batches the app's engine sends and the events the platform reports,
// and steps act with the platform's input where the model says a view
// is.
type target struct {
	a       *App
	surface testhost.Surface
	// live is set for a running app's surface the automation follows (automation.go), whose model also drops the alerts the user answers.
	live bool
	// quiet reports whether the last settle came to quiet before settleWait.
	quiet bool

	mu      sync.Mutex
	batches []*driver.MountBatch
	events  []driver.Event
	// lost says why a live model stopped following its surface: a batch it could not apply. It queues nothing after.
	lost string
}

// observe queues a batch the app's engine sent, which the next drain applies.
func (t *target) observe(b *driver.MountBatch) {
	t.mu.Lock()
	if t.lost == "" {
		t.batches = append(t.batches, b)
	}
	t.mu.Unlock()
}

// event queues what the next drain takes of an event the platform reported: scrolls, the focus and texts, and for a live app alerts answered.
func (t *target) event(ev driver.Event) {
	switch ev.Kind {
	case driver.EventScroll, driver.EventFocus, driver.EventBlur, driver.EventChangeText:
	case driver.EventAlert:
		if !t.live {
			return
		}
	default:
		return
	}
	t.mu.Lock()
	if t.lost == "" {
		t.events = append(t.events, ev)
	}
	t.mu.Unlock()
}

// lostWhy says why a live model stopped following its surface, "" while it follows.
func (t *target) lostWhy() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lost
}

// applyLive applies b to a running app's model. One the model cannot apply leaves it lost, dropping what is queued, and fails the step taking it, rather than ending the app or leaving a model half applied to answer later steps.
func (t *target) applyLive(b *driver.MountBatch) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		why := fmt.Sprint(v)
		if f, ok := v.(liveFailure); ok {
			why = f.msg
		}
		why, _, _ = strings.Cut(strings.TrimPrefix(why, "nsgotest: "), "\n")
		t.mu.Lock()
		t.lost, t.batches, t.events = why, nil, nil
		t.mu.Unlock()
		log.Printf("nsgo: the automation stopped following a surface: %s", why)
		fail(automation.Failed, "the automation stopped following the window: "+why)
	}()
	t.a.apply(b)
}

// quietFor is how long nothing must change for a step to be over, and
// settleWait how long a step waits for that.
const (
	quietFor   = 100 * time.Millisecond
	settleWait = 5 * time.Second
)

func (a *App) startTarget(app func(cx *core.Scope) core.Node) {
	a.t.Helper()
	h := testhost.Current()
	switch {
	case h == nil:
		a.t.Fatal("nsgotest: no test host is linked: test the app's main package, which imports github.com/nsgo-dev/nsgo, with nsgo test -target")
	case !mainRan:
		a.t.Fatal("nsgotest: a platform needs the main thread: add func TestMain(m *testing.M) { nsgotest.Main(m) }")
	case opened && !h.Several():
		a.t.Skip("nsgotest: the test opens a second app, which only a desktop shows beside the first")
	}
	opened = true
	if a.store == nil {
		a.store = core.NewStore(nil)
	}
	t := &target{a: a}
	a.target = t
	// The test's own graph shares the app's store, as another window
	// would, for Storage.
	a.root = core.NewRoot(core.RootOptions{
		Post: func(fn func()) {
			a.mu.Lock()
			a.posts = append(a.posts, fn)
			a.mu.Unlock()
		},
		Clock: a.Clock, Storage: a.store,
	})
	a.t.Cleanup(a.root.Dispose)
	a.anim = driver.NewAnimator(func(driver.Event) {})
	// A nil clock is the system's; a nil *ManualClock would not be.
	var clock core.Clock = a.Clock
	if a.measure {
		clock = nil
		driver.CheckCommits = false
	}
	s, err := h.Open(app, testhost.Options{
		Size: a.size, Environment: a.env, Storage: a.store, Host: a.host, Clock: clock,
		FPS:     os.Getenv("NSGO_TEST_FPS") != "",
		Observe: t.observe,
		Event:   t.event,
	})
	if err != nil {
		a.t.Fatalf("nsgotest: the platform cannot open the app: %v", err)
	}
	t.surface = s
	// The first batch can take longer than a step's quiet time to come,
	// as on Windows, where the window opens first.
	for deadline := time.Now().Add(settleWait); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		t.mu.Lock()
		came := len(t.batches) > 0
		t.mu.Unlock()
		if came {
			break
		}
	}
	t.settle()
}

// drain applies what came since the last call, reporting whether
// anything did.
// drainEvents is drain, reporting only whether events came.
func (t *target) drainEvents() bool {
	t.mu.Lock()
	events := len(t.events)
	t.mu.Unlock()
	t.drain()
	return events > 0
}

func (t *target) drain() bool {
	t.mu.Lock()
	batches, events := t.batches, t.events
	t.batches, t.events = nil, nil
	t.mu.Unlock()
	for _, b := range batches {
		t.a.traceBatch(b, 0)
		if t.live {
			t.applyLive(b)
		} else {
			t.a.apply(b)
		}
	}
	for _, ev := range events {
		if ev.Kind == driver.EventAlert {
			// The user answered it, or the app's own code did.
			t.a.alerts = slices.DeleteFunc(t.a.alerts, func(c driver.AlertConfig) bool { return c.ID == ev.Count })
			continue
		}
		v := t.a.views[ev.ID]
		if v == nil {
			continue
		}
		switch ev.Kind {
		case driver.EventScroll:
			v.Offset, v.Applied = ev.Point, ev.Count
			if ev.Value != 0 {
				v.ZoomScale = ev.Value
			}
		case driver.EventFocus, driver.EventBlur:
			v.Focused = ev.Kind == driver.EventFocus
		case driver.EventChangeText:
			v.Text = ev.Text
		}
	}
	return len(batches)+len(events) > 0
}

// settle waits until no batch came and the platform animated nothing for
// quietFor, settleWait at most, applying what came.
func (t *target) settle() {
	deadline := time.Now().Add(settleWait)
	if t.a.measure {
		// An app measured may commit as long as it runs, as a ticking
		// table does, and asking the platform what animates walks its
		// views: a step waits quietFor for what it set off to land and
		// for the platform's events to stop, then only for what it moved
		// to rest, looking seldom, so the readings are the app's.
		quiet := time.Now()
		for {
			t.runPosts()
			if t.drainEvents() {
				quiet = time.Now()
			}
			if time.Since(quiet) >= quietFor && !t.surface.Animating() || time.Now().After(deadline) {
				t.quiet = time.Now().Before(deadline)
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	quiet := time.Now()
	for {
		t.runPosts()
		if t.drain() || t.surface.Animating() {
			quiet = time.Now()
		}
		if time.Since(quiet) >= quietFor || time.Now().After(deadline) {
			t.quiet = time.Since(quiet) >= quietFor
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// runPosts runs what the test's own graph posted.
func (t *target) runPosts() {
	t.a.mu.Lock()
	posts := t.a.posts
	t.a.posts = nil
	t.a.mu.Unlock()
	for _, fn := range posts {
		fn()
	}
}

func (t *target) check(err error) {
	t.a.t.Helper()
	if err != nil {
		t.a.t.Fatalf("nsgotest: %s: %v", t.a.stepName, err)
	}
}

// reveal scrolls v into view, as a user does before acting on it: each
// scroll view around it that hides its middle moves it to the middle, a
// gesture at a time. It returns where v's middle is then.
func (t *target) reveal(v *View) driver.Point {
	t.a.t.Helper()
	for range 30 {
		_, at := v.center()
		var hider *View
		var box driver.Bounds
		for p := v.Parent; p != nil; p = p.Parent {
			if p.Kind == driver.KindScroll && !p.Scroll.Behavior.NoScroll {
				b := p.surfaceFrame()
				if at.X < b.Origin.X || at.Y < b.Origin.Y || at.X >= b.Origin.X+b.Size.Width || at.Y >= b.Origin.Y+b.Size.Height {
					hider, box = p, b
					break
				}
			}
		}
		if hider == nil {
			return at
		}
		mid := driver.Point{X: box.Origin.X + box.Size.Width/2, Y: box.Origin.Y + box.Size.Height/2}
		d := driver.Point{X: at.X - mid.X, Y: at.Y - mid.Y}
		limit := func(v, size driver.Points) driver.Points { return min(max(v, -size*0.8), size*0.8) }
		if hider.Scroll.Horizontal {
			d = driver.Point{X: limit(d.X, box.Size.Width)}
		} else {
			d = driver.Point{Y: limit(d.Y, box.Size.Height)}
		}
		t.check(t.surface.Scroll(mid, d))
		t.settle()
	}
	_, at := v.center()
	return at
}

func (t *target) storage() core.Storage { return t.a.root.Storage() }

// menu takes a menu step with the platform's own menus: a platform that
// shows none skips the test.
func (t *target) menu(step func(testhost.Surface) error) {
	t.a.t.Helper()
	if err := step(t.surface); errors.Is(err, testhost.ErrNoMenu) {
		t.a.t.Skipf("nsgotest: %s: %v", t.a.stepName, err)
	} else {
		t.check(err)
	}
	t.settle()
}

// advance moves the test's clock, which the app's timers run on, by d:
// a frame at a time, settling between, when run, else at once. The
// platform's own animations run in real time.
func (t *target) advance(d time.Duration, run bool) {
	if t.a.measure {
		time.Sleep(d)
		t.runPosts()
		t.drain()
		return
	}
	if !run {
		t.a.Clock.Jump(d)
		t.settle()
		return
	}
	for d > 0 {
		step := min(d, driver.FrameInterval)
		t.a.Clock.Advance(step)
		d -= step
		// What the step's timers posted runs before the next step, so
		// the timers it sets (an interval's next run) fall due in time.
		t.surface.Sync()
		t.drain()
	}
	t.settle()
}

func (t *target) setEnvironment(env driver.Environment) {
	t.check(t.surface.SetEnvironment(env))
	t.settle()
}

func (t *target) resize(size driver.Size) {
	t.check(t.surface.Resize(size))
	t.settle()
}

func (t *target) answer(alert driver.AlertConfig, index int) {
	t.check(t.surface.Answer(alert.ID, index))
	t.settle()
}

func (t *target) press(at driver.Point, hold time.Duration) { t.check(t.surface.Press(at, hold)) }

// pressWhile holds a press for hold, running during once the finger is
// down.
func (t *target) pressWhile(at driver.Point, hold time.Duration, during func()) {
	done := make(chan error, 1)
	go func() { done <- t.surface.Press(at, hold) }()
	time.Sleep(hold / 2)
	during()
	t.check(<-done)
}

func (t *target) rightClick(at driver.Point) { t.check(t.surface.RightClick(at)) }
func (t *target) hover(at driver.Point)      { t.check(t.surface.Hover(at)) }
func (t *target) back()                      { t.check(t.surface.Back()) }

func (t *target) typeText(v *View, text string) { t.check(t.surface.Type(text)) }
func (t *target) compose(text string)           { t.ime(t.surface.Compose(text)) }
func (t *target) commit(text string)            { t.ime(t.surface.Commit(text)) }

// ime checks a composition step: a machine without an input method skips
// the test.
func (t *target) ime(err error) {
	t.a.t.Helper()
	if errors.Is(err, testhost.ErrNoInputMethod) {
		t.a.t.Skipf("nsgotest: %s: %v", t.a.stepName, err)
	}
	t.check(err)
}

// clear selects the field's text and deletes it, as a keyboard does.
func (t *target) clear(v *View) {
	t.check(t.surface.Key("mod+a"))
	t.check(t.surface.Key("Backspace"))
}

func (t *target) keys(v *View, key string) { t.check(t.surface.Key(key)) }

func (t *target) drag(start driver.Point, path []driver.Point) {
	t.check(t.surface.Drag(append([]driver.Point{start}, path...)))
}

// scroll drags v's content by d along its axis, starting from its middle.
func (t *target) scroll(v *View, d driver.Points) {
	_, at := v.center()
	delta := driver.Point{Y: d}
	if v.Scroll.Horizontal {
		delta = driver.Point{X: d}
	}
	t.check(t.surface.Scroll(at, delta))
}

// CapturePNG captures the app as the platform shows it and returns its PNG.
// Headless runs skip the test because they have no native pixels.
func (a *App) CapturePNG() []byte {
	a.t.Helper()
	if a.target == nil {
		a.t.Skip("nsgotest: no pixels headless; run under nsgo test -target")
	}
	png, err := a.target.screenshotPNG()
	if err != nil {
		a.t.Fatalf("nsgotest: capture: %v", err)
	}
	return append([]byte(nil), png...)
}

// Capture captures the app as the platform shows it and decodes its PNG.
// Headless runs skip the test because they have no native pixels.
func (a *App) Capture() image.Image {
	pngData := a.CapturePNG()
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		a.t.Fatalf("nsgotest: decode capture: %v", err)
	}
	return img
}

// Screenshot captures the app as the platform shows it. name is a relative
// path under the directory `nsgo test -target` saves screenshots to; its
// extension is optional and .png is added when absent. Headless runs have no
// pixels and keep the historical log-only behavior.
func (a *App) Screenshot(name string) {
	a.t.Helper()
	if name == "" || filepath.Clean(name) == "." || filepath.IsAbs(name) || pathHasParent(name) {
		a.t.Fatalf("nsgotest: screenshot name must be a relative file path without ..: %q", name)
	}
	if a.target == nil {
		a.t.Logf("nsgotest: no screenshot %q headless; nsgo test -target takes it", name)
		return
	}
	png := a.CapturePNG()
	encoded := base64.RawURLEncoding.EncodeToString([]byte(name))
	// The tool saves it: the app may run on a phone, or in a browser. A
	// device's log keeps lines of a few kilobytes, so it goes in chunks.
	data := base64.StdEncoding.EncodeToString(png)
	const chunk = 3000
	n := (len(data) + chunk - 1) / chunk
	for i := range n {
		fmt.Printf("nsgo-screenshot ~%s %d/%d %s\n", encoded, i+1, n, data[i*chunk:min((i+1)*chunk, len(data))])
	}
}

// screenshotPNG settles and captures the app as the platform shows it.
func (t *target) screenshotPNG() ([]byte, error) {
	t.settle()
	return t.surface.Screenshot()
}

func pathHasParent(name string) bool {
	for _, part := range strings.FieldsFunc(filepath.ToSlash(name), func(r rune) bool { return r == '/' }) {
		if part == ".." {
			return true
		}
	}
	return false
}
