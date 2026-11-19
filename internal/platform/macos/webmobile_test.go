//go:build darwin && !ios

package macos

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
)

// mobilePage reports what it sees of the device, and then the touches a drag made or the mouse's release after one that made none.
const mobilePage = `<!doctype html><html><head><meta name="viewport" content="width=device-width"></head><body style="margin:0;height:100vh"><script>
var seen = { moves: 0, mouseMoves: 0, ids: [] };
var at = function (e) { var t = e.changedTouches[0]; seen.ids.push(t.identifier); return [Math.round(t.clientX), Math.round(t.clientY), e.touches.length]; };
addEventListener('touchstart', function (e) { seen.start = at(e); }, { passive: false });
addEventListener('touchmove', function (e) { seen.moves++; seen.last = at(e); }, { passive: false });
addEventListener('touchend', function (e) { seen.end = at(e); window.nsgo.postMessage('touches:' + JSON.stringify(seen)); });
addEventListener('mousemove', function (e) { if (seen.start && !seen.end) seen.mouseMoves++; });
addEventListener('mouseup', function () { window.nsgo.postMessage('mouseup:' + JSON.stringify(seen)); });
window.nsgo.postMessage('ready:' + JSON.stringify({ ua: navigator.userAgent, points: navigator.maxTouchPoints,
  coarse: matchMedia('(pointer: coarse)').matches, noHover: matchMedia('(hover: none)').matches, touch: 'ontouchstart' in window }));
</script></body></html>`

type mobileDevice struct {
	UA      string `json:"ua"`
	Points  int    `json:"points"`
	Coarse  bool   `json:"coarse"`
	NoHover bool   `json:"noHover"`
	Touch   bool   `json:"touch"`
}

type mobileTouches struct {
	Start, Last, End []int
	Moves            int
	MouseMoves       int
	IDs              []int
}

// TestWebMobilePresentsAPhone loads a page in a Mobile web view, one with a user agent of its own too and one without Mobile, and drags the mouse up over the first and the last with AppKit's events. The Mobile views' pages see a touch screen without hover, the first iPhone Safari's user agent and the second its own, and the drag reaches the first as a finger's touches, with no mouse moves meanwhile; the last sees the desktop and only the mouse. Run ends a process's app for good, so the test runs in a process of its own.
func TestWebMobilePresentsAPhone(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if os.Getenv("NSGO_TEST_WEB_PROCESS") != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_WEB_PROCESS="+t.Name(), "HOME="+t.TempDir())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	var mu sync.Mutex
	heard := map[string][]string{}
	var once sync.Once
	app := func(cx *core.Scope) core.Node {
		web := func(id string, mobile bool, ua string) core.Node {
			return core.WebView{Source: core.WebHTML(mobilePage, "https://localhost/"), Mobile: mobile, UserAgent: ua,
				Style: core.Style{Width: core.Pt(200), Height: core.Pt(200)},
				OnMessage: func(data string) {
					mu.Lock()
					heard[id] = append(heard[id], data)
					ready := len(heard["mobile"]) > 0 && len(heard["agent"]) > 0 && len(heard["desktop"]) > 0
					done := slicesHavePrefix(heard["mobile"], "touches:") && slicesHavePrefix(heard["desktop"], "mouseup:")
					mu.Unlock()
					if ready {
						once.Do(func() { go dragMobileViews() })
					}
					if done {
						cx.Quit()
					}
				}}
		}
		return core.View{Style: core.Style{Direction: core.Row}, Children: []core.Node{
			web("mobile", true, ""), web("agent", true, "nsgo-test"), web("desktop", false, ""),
		}}
	}
	watchdog := time.AfterFunc(30*time.Second, func() { panic("the web views did not report the drags") })
	defer watchdog.Stop()
	onMain(func() { Run(app, Options{Title: "Web mobile", Width: 600, Height: 200}) })
	mu.Lock()
	defer mu.Unlock()
	t.Logf("heard %q", heard)
	device := func(id string) mobileDevice {
		var d mobileDevice
		if len(heard[id]) == 0 || json.Unmarshal([]byte(strings.TrimPrefix(heard[id][0], "ready:")), &d) != nil {
			t.Fatalf("the %s view heard %q", id, heard[id])
		}
		return d
	}
	phone := mobileDevice{UA: driver.WebMobileUserAgentIPhone, Points: 5, Coarse: true, NoHover: true, Touch: true}
	if d := device("mobile"); d != phone {
		t.Errorf("the Mobile view's page sees %+v, want %+v", d, phone)
	}
	if d, want := device("agent"), (mobileDevice{UA: "nsgo-test", Points: 5, Coarse: true, NoHover: true, Touch: true}); d != want {
		t.Errorf("the Mobile view with a user agent's page sees %+v, want %+v", d, want)
	}
	if d := device("desktop"); d.Points != 0 || d.Coarse || d.NoHover || d.Touch || strings.Contains(d.UA, "iPhone") {
		t.Errorf("the desktop view's page sees %+v", d)
	}
	touches := func(id, prefix string) mobileTouches {
		var got mobileTouches
		for _, m := range heard[id] {
			if rest, ok := strings.CutPrefix(m, prefix); ok && json.Unmarshal([]byte(rest), &got) == nil {
				return got
			}
		}
		t.Fatalf("the %s view heard %q", id, heard[id])
		return got
	}
	checkDrag(t, touches("mobile", "touches:"))
	if got := touches("desktop", "mouseup:"); got.Start != nil || got.Moves != 0 {
		t.Errorf("the desktop view's page saw touches: %+v", got)
	}
}

func slicesHavePrefix(messages []string, prefix string) bool {
	for _, m := range messages {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}
	return false
}

// checkDrag holds the touches a drag up from (100, 150) to (100, 40) made to a finger's: one that starts and ends where the drag did, moves up between, is down until it ends and keeps its identifier, with no mouse moves meanwhile.
func checkDrag(t *testing.T, got mobileTouches) {
	t.Helper()
	near := func(p []int, x, y, down int) bool {
		return len(p) == 3 && p[0]-x <= 2 && x-p[0] <= 2 && p[1]-y <= 2 && y-p[1] <= 2 && p[2] == down
	}
	same := len(got.IDs) > 0
	for _, id := range got.IDs {
		same = same && id == got.IDs[0]
	}
	if !near(got.Start, 100, 150, 1) || !near(got.Last, 100, 40, 1) || !near(got.End, 100, 40, 0) || got.Moves < 3 || !same || got.MouseMoves != 0 {
		t.Errorf("the drag made %+v", got)
	}
}

// dragMobileViews drags the mouse up from (100, 150) to (100, 40) over the Mobile web view with iPhone Safari's user agent and over the one without Mobile, as AppKit's events handed to each WKWebView, a frame apart.
func dragMobileViews() {
	webs := make(chan []objc.ID, 1)
	apple.OnMain(func() {
		var found []objc.ID
		for _, want := range []string{driver.WebMobileUserAgentIPhone, ""} {
			for id, v := range views {
				if v.web != nil && v.web.View() == id && apple.GoString(id.Send(sel("customUserAgent"))) == want {
					found = append(found, id)
				}
			}
		}
		webs <- found
	})
	for _, web := range <-webs {
		const steps = 10
		for i := 0; i <= steps+1; i++ {
			done := make(chan struct{})
			apple.OnMain(func() {
				defer close(done)
				kind, name, y := uint(evLeftDragged), "mouseDragged:", 150-110*float64(min(i, steps))/steps
				switch i {
				case 0:
					kind, name = evLeftDown, "mouseDown:"
				case steps + 1:
					kind, name = evLeftUp, "mouseUp:"
				}
				at := objc.Send[apple.CGPoint](web, sel("convertPoint:toView:"), apple.CGPoint{X: 100, Y: y}, objc.ID(0))
				window := web.Send(sel("window"))
				ev := objc.ID(apple.Class("NSEvent")).Send(
					sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
					kind, at, uint(0), uptime(), objc.Send[int](window, sel("windowNumber")), objc.ID(0), 0, 1, float32(1))
				web.Send(sel(name), ev)
			})
			<-done
			time.Sleep(16 * time.Millisecond)
		}
	}
}
