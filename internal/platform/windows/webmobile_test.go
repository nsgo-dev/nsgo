//go:build windows

package windows

import (
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
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
// Two frames once the page shows: WebView2 takes input once the page draws.
var shown = function () {
  if (document.visibilityState !== 'visible') return;
  document.removeEventListener('visibilitychange', shown);
  requestAnimationFrame(function () { requestAnimationFrame(function () { window.nsgo.postMessage('painted'); }); });
};
document.addEventListener('visibilitychange', shown);
shown();
window.nsgo.postMessage('ready:' + JSON.stringify({ ua: navigator.userAgent, points: navigator.maxTouchPoints,
  coarse: matchMedia('(pointer: coarse)').matches, noHover: matchMedia('(hover: none)').matches, touch: 'ontouchstart' in window,
  mobile: !!(navigator.userAgentData && navigator.userAgentData.mobile), platform: navigator.userAgentData ? navigator.userAgentData.platform : '' }));
</script></body></html>`

var setCursorPos = user32.NewProc("SetCursorPos")

type mobileDevice struct {
	UA       string `json:"ua"`
	Points   int    `json:"points"`
	Coarse   bool   `json:"coarse"`
	NoHover  bool   `json:"noHover"`
	Touch    bool   `json:"touch"`
	Mobile   bool   `json:"mobile"`
	Platform string `json:"platform"`
}

type mobileTouches struct {
	Start, Last, End []int
	Moves            int
	MouseMoves       int
	IDs              []int
}

// TestWebMobilePresentsAPhone loads a page in a Mobile web view, one with a user agent of its own too and one without Mobile, and drags the mouse up over the first and the last with the window's mouse messages, which the backend forwards to WebView2. The Mobile views' pages see a touch screen without hover and Android's client hints, the first Android Chrome's user agent and the second its own, and WebView2's emulation turns the drag over the first into a finger's touches, with no mouse moves meanwhile; the last sees the desktop and only the mouse. The first view's next page, of another site, is a phone's too. It runs in a process of its own, with a profile of its own.
func TestWebMobilePresentsAPhone(t *testing.T) {
	if testing.Short() {
		t.Skip("opens a window")
	}
	if _, err := findWebView2(); err != nil {
		t.Skip(err)
	}
	if os.Getenv("NSGO_TEST_WEBVIEW2_PROCESS") != t.Name() {
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v")
		// WebView2's processes may hold the profile a while after the test's process ends.
		dir, err := os.MkdirTemp("", "nsgo-mobile-webview2-")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(100 * time.Millisecond) {
				if err := os.RemoveAll(dir); err == nil {
					return
				} else if time.Now().After(deadline) {
					t.Logf("WebView2 profile cleanup: %v", err)
					return
				}
			}
		}()
		cmd.Env = append(os.Environ(), "NSGO_TEST_WEBVIEW2_PROCESS="+t.Name(), "APPDATA="+dir, "LOCALAPPDATA="+dir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		if strings.Contains(string(out), "--- SKIP") {
			t.Skipf("%s", out)
		}
		return
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
	title := "nsgo-web-mobile"
	app := func(cx *core.Scope) core.Node {
		web := func(id string, mobile bool, ua string) core.Node {
			var ref core.Ref
			return core.WebView{Source: core.WebHTML(mobilePage, "https://localhost/"), Mobile: mobile, UserAgent: ua, Ref: &ref,
				MessageOrigins: []string{"https://localhost", "https://next.localhost"},
				Style:          core.Style{Width: driver.Pt(200), Height: driver.Pt(200)},
				OnMessage: func(data string) {
					mu.Lock()
					heard[id] = append(heard[id], data)
					mu.Unlock()
					// The page of another site the view goes to next is a phone's too.
					if id == "mobile" && strings.HasPrefix(data, "touches:") {
						ref.Load(core.WebHTML(mobilePage, "https://next.localhost/"))
					}
				}}
		}
		return core.View{Style: core.Style{Width: driver.Pt(600), Height: driver.Pt(200), Direction: core.Row, Background: driver.RGB(0xffffff)},
			Children: []core.Node{web("mobile", true, ""), web("agent", true, "nsgo-test"), web("desktop", false, "")}}
	}
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer close(done)
		Run(app, Options{Title: title, Width: 600, Height: 200})
	}()
	hwnd := waitForWindow(t, title, done)
	if hwnd == 0 {
		return
	}
	t.Cleanup(func() {
		postMessageW.Call(hwnd, wmClose, 0, 0)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("the window did not close")
		}
	})
	wait := func(what string, ok func() bool) {
		t.Helper()
		// The first start of the browser process can take a while.
		for deadline := time.Now().Add(30 * time.Second); !ok(); time.Sleep(20 * time.Millisecond) {
			if time.Now().After(deadline) {
				mu.Lock()
				defer mu.Unlock()
				t.Fatalf("%s: heard %q", what, heard)
			}
		}
	}
	wait("the pages did not load", func() bool { return has("mobile", "ready:") && has("agent", "ready:") && has("desktop", "ready:") })
	wait("the pages did not draw", func() bool { return has("mobile", "painted") && has("desktop", "painted") })
	// A drag up from (100, 150) to (100, 40) in a view x points from the window's left, as the mouse's messages, with the window in front and the cursor where they say: posted to a window behind others, away from the cursor, they reached the page only some of the time.
	drag := func(x driver.Points) {
		var at [12]uintptr
		var screen [12]point
		onMain(t, hwnd, func(w *window) {
			for i := range at {
				y := 150 - driver.Points(11*min(i, 10))
				at[i] = uintptr(uint32(uint16(w.px(y)))<<16 | uint32(uint16(w.px(x))))
				screen[i] = point{w.px(x), w.px(y)}
				clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&screen[i])))
			}
		})
		setForegroundWindow.Call(hwnd)
		send := func(i int, message, wParam uintptr) {
			setCursorPos.Call(uintptr(screen[i].X), uintptr(screen[i].Y))
			postMessageW.Call(hwnd, message, wParam, at[i])
		}
		send(0, wmMouseMove, 0)
		send(0, wmLButtonDown, mkLButton)
		time.Sleep(100 * time.Millisecond)
		for i := 1; i <= 10; i++ {
			send(i, wmMouseMove, mkLButton)
			time.Sleep(16 * time.Millisecond)
		}
		send(11, wmLButtonUp, 0)
	}
	// WebView2 at times drops the input a page gets soon after it shows: drag again until one reaches the page.
	dragUntil := func(x driver.Points, what string, ok func() bool) {
		t.Helper()
		for try := 1; ; try++ {
			drag(x)
			for deadline := time.Now().Add(2 * time.Second); !ok() && time.Now().Before(deadline); {
				time.Sleep(20 * time.Millisecond)
			}
			if ok() {
				t.Logf("%s on try %d", what, try)
				return
			}
			if try == 10 {
				mu.Lock()
				defer mu.Unlock()
				t.Fatalf("no %s: heard %q", what, heard)
			}
		}
	}
	dragUntil(100, "touches from the drag over the Mobile view", func() bool { return has("mobile", "touches:") })
	dragUntil(500, "mouse release from the drag over the desktop view", func() bool { return has("desktop", "mouseup:") })
	wait("the next page did not load", func() bool {
		mu.Lock()
		defer mu.Unlock()
		n := 0
		for _, m := range heard["mobile"] {
			if strings.HasPrefix(m, "ready:") {
				n++
			}
		}
		return n == 2
	})
	mu.Lock()
	defer mu.Unlock()
	t.Logf("heard %q", heard)
	device := func(id string) mobileDevice {
		var d mobileDevice
		ready := ""
		for _, m := range heard[id] {
			if rest, ok := strings.CutPrefix(m, "ready:"); ok {
				ready = rest
			}
		}
		if json.Unmarshal([]byte(ready), &d) != nil {
			t.Fatalf("the %s view heard %q", id, heard[id])
		}
		return d
	}
	d := device("mobile")
	if head, tail, _ := strings.Cut(driver.WebMobileUserAgentAndroid, "%s"); !strings.HasPrefix(d.UA, head) || !strings.HasSuffix(d.UA, tail) {
		t.Errorf("the Mobile view's user agent is %q", d.UA)
	}
	phone := mobileDevice{UA: d.UA, Points: 5, Coarse: true, NoHover: true, Touch: true, Mobile: true, Platform: "Android"}
	if d != phone {
		t.Errorf("the Mobile view's page sees %+v, want %+v", d, phone)
	}
	if d, want := device("agent"), (mobileDevice{UA: "nsgo-test", Points: 5, Coarse: true, NoHover: true, Touch: true, Mobile: true, Platform: "Android"}); d != want {
		t.Errorf("the Mobile view with a user agent's page sees %+v, want %+v", d, want)
	}
	if d := device("desktop"); d.Coarse || d.NoHover || d.Mobile || d.Platform != "Windows" || strings.Contains(d.UA, "Android") {
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
	// Chromium may fold the last moves into the end.
	movedUp := len(got.Last) == 3 && got.Last[1] < 100 && got.Last[2] == 1
	if !near(got.Start, 100, 150, 1) || !movedUp || !near(got.End, 100, 40, 0) || got.Moves < 3 || !same || got.MouseMoves != 0 {
		t.Errorf("the drag made %+v", got)
	}
	if got := touches("desktop", "mouseup:"); got.Start != nil || got.Moves != 0 {
		t.Errorf("the desktop view's page saw touches: %+v", got)
	}
}
