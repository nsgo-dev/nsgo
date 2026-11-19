//go:build linux && !android

package linux

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// mobilePage reports what it sees of the device, and then the touches a drag made or the mouse's release after one that made none.
const mobilePage = `<!doctype html><html><head><meta name="viewport" content="width=device-width"></head><body style="margin:0;height:100vh"><script>
var seen = { moves: 0, ids: [] };
var at = function (e) { var t = e.changedTouches[0]; seen.ids.push(t.identifier); return [Math.round(t.clientX), Math.round(t.clientY), e.touches.length]; };
addEventListener('touchstart', function (e) { seen.start = at(e); }, { passive: false });
addEventListener('touchmove', function (e) { seen.moves++; seen.last = at(e); }, { passive: false });
addEventListener('touchend', function (e) { seen.end = at(e); window.nsgo.postMessage('touches:' + JSON.stringify(seen)); });
addEventListener('mouseup', function () { window.nsgo.postMessage('mouseup:' + JSON.stringify(seen)); });
window.nsgo.postMessage('ready:' + JSON.stringify({ ua: navigator.userAgent, points: navigator.maxTouchPoints,
  coarse: matchMedia('(pointer: coarse)').matches, noHover: matchMedia('(hover: none)').matches, touch: 'ontouchstart' in window }));
</script></body></html>`

// mobileDrag drags the mouse up from (100, 150) to (100, 40) in the page, as the pointer and mouse events WebKit would dispatch for one: GTK cannot make the input a user's.
const mobileDrag = `(function () {
  var el = document.elementFromPoint(100, 150) || document.body;
  var send = function (kind, y, buttons) {
    var init = { bubbles: true, cancelable: true, composed: true, view: window, clientX: 100, clientY: y, button: 0, buttons: buttons, pointerId: 1, pointerType: 'mouse', isPrimary: true };
    el.dispatchEvent(new PointerEvent('pointer' + kind, init));
    el.dispatchEvent(new MouseEvent('mouse' + kind, init));
  };
  send('down', 150, 1);
  for (var i = 1; i <= 10; i++) send('move', 150 - 11 * i, 1);
  send('up', 40, 0);
})()`

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
	IDs              []int
}

// TestWebMobilePresentsAPhone loads a page in a Mobile web view, one with a user agent of its own too and one without Mobile, and drags the mouse up over the first and the last. The Mobile views' pages see a touch screen without hover, the first Android Chrome's user agent and the second its own, and the drag reaches the first as a finger's touches; the last sees the desktop and only the mouse. It skips where WebKitGTK 6.0 is missing, in a process of its own as WebKit takes the first thread that uses it.
func TestWebMobilePresentsAPhone(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if os.Getenv("NSGO_TEST_WEBKIT_PROCESS") != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		cmd.Env = append(os.Environ(), "NSGO_TEST_WEBKIT_PROCESS="+t.Name())
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		if strings.Contains(string(out), "--- SKIP") {
			t.Skipf("%s", out)
		}
		return
	}
	if err := openWebKit(); err != nil {
		t.Skip(err)
	}
	var mu sync.Mutex
	heard := map[string][]string{}
	has := func(id, prefix string) bool {
		mu.Lock()
		defer mu.Unlock()
		for _, m := range heard[id] {
			if strings.HasPrefix(m, prefix) {
				return true
			}
		}
		return false
	}
	app := func(cx *core.Scope) core.Node {
		web := func(id string, mobile bool, ua string) core.Node {
			return core.WebView{Source: core.WebHTML(mobilePage, "https://localhost/"), Mobile: mobile, UserAgent: ua, TestID: id,
				Style: core.Style{Width: driver.Pt(200), Height: driver.Pt(200)},
				OnMessage: func(data string) {
					mu.Lock()
					heard[id] = append(heard[id], data)
					mu.Unlock()
				}}
		}
		return core.View{Style: core.Style{Width: driver.Pt(600), Height: driver.Pt(200), Direction: core.Row},
			Children: []core.Node{web("mobile", true, ""), web("agent", true, "nsgo-test"), web("desktop", false, "")}}
	}
	w := startTestWindow(t, app, Options{Title: "nsgo-native-" + t.Name(), Width: 600, Height: 200})
	wait := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(20 * time.Second); !ok(); time.Sleep(20 * time.Millisecond) {
			if time.Now().After(deadline) {
				mu.Lock()
				defer mu.Unlock()
				t.Fatalf("%s: heard %q", what, heard)
			}
		}
	}
	wait("the pages did not load", func() bool { return has("mobile", "ready:") && has("agent", "ready:") && has("desktop", "ready:") })
	w.onLoop(t, func() {
		for _, v := range w.views {
			if v != nil && v.web != nil && v.web.widget != 0 && v.web.config.UserAgent == "" {
				evaluate(v.web.widget, mobileDrag)
			}
		}
	})
	wait("the drags made nothing", func() bool { return has("mobile", "touches:") && has("desktop", "mouseup:") })
	mu.Lock()
	defer mu.Unlock()
	t.Logf("heard %q", heard)
	device := func(id string) mobileDevice {
		var d mobileDevice
		if json.Unmarshal([]byte(strings.TrimPrefix(heard[id][0], "ready:")), &d) != nil {
			t.Fatalf("the %s view heard %q", id, heard[id])
		}
		return d
	}
	phone := mobileDevice{UA: mobileUserAgent, Points: 5, Coarse: true, NoHover: true, Touch: true}
	if d := device("mobile"); d != phone {
		t.Errorf("the Mobile view's page sees %+v, want %+v", d, phone)
	}
	if d, want := device("agent"), (mobileDevice{UA: "nsgo-test", Points: 5, Coarse: true, NoHover: true, Touch: true}); d != want {
		t.Errorf("the Mobile view with a user agent's page sees %+v, want %+v", d, want)
	}
	if d := device("desktop"); d.Points != 0 || d.Coarse || d.NoHover || strings.Contains(d.UA, "Android") {
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
	got := touches("mobile", "touches:")
	near := func(p []int, x, y, down int) bool {
		return len(p) == 3 && p[0]-x <= 2 && x-p[0] <= 2 && p[1]-y <= 2 && y-p[1] <= 2 && p[2] == down
	}
	same := len(got.IDs) > 0
	for _, id := range got.IDs {
		same = same && id == got.IDs[0]
	}
	if !near(got.Start, 100, 150, 1) || !near(got.Last, 100, 40, 1) || !near(got.End, 100, 40, 0) || got.Moves < 3 || !same {
		t.Errorf("the drag made %+v", got)
	}
	if got := touches("desktop", "mouseup:"); got.Start != nil || got.Moves != 0 {
		t.Errorf("the desktop view's page saw touches: %+v", got)
	}
}
