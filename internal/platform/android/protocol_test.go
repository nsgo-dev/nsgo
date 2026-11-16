package android

import (
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// javaNames are the driver's names of the kinds Java names otherwise.
var javaNames = map[string]string{"EventHoverIn": "EventMouseEnter", "EventHoverOut": "EventMouseLeave", "EventHoverMove": "EventMouseMove"}

// TestJavaProtocolMatchesTheDriver holds Protocol.java's item and event kinds, roles and key uses to the driver's of the same names, which the Java runtime reads the batches and writes the events by: a kind the driver gains shifts the ones after it.
func TestJavaProtocolMatchesTheDriver(t *testing.T) {
	fset := token.NewFileSet()
	dir := filepath.Join("..", "..", "driver")
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, f := range pkgs["driver"].Files {
		files = append(files, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("github.com/nsgo-dev/nsgo/internal/driver", fset, files, nil)
	if err != nil {
		t.Fatal(err)
	}
	java, err := os.ReadFile(filepath.Join("java", "dev", "oneo", "nsgo", "Protocol.java"))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, m := range regexp.MustCompile(`static final int (ITEM|EVENT|ROLE|KEY_USE)_(\w+) = (\d+);`).FindAllStringSubmatch(string(java), -1) {
		var name string
		for _, word := range strings.Split(m[1]+"_"+m[2], "_") {
			if word == "URL" {
				name += word
				continue
			}
			name += word[:1] + strings.ToLower(word[1:])
		}
		if n, ok := javaNames[name]; ok {
			name = n
		}
		c, ok := pkg.Scope().Lookup(name).(*types.Const)
		if !ok {
			t.Errorf("Protocol.java's %s_%s is no driver constant %s", m[1], m[2], name)
			continue
		}
		want, _ := constant.Int64Val(c.Val())
		if got, _ := strconv.Atoi(m[3]); int64(got) != want {
			t.Errorf("Protocol.java's %s_%s is %d, the driver's %s %d", m[1], m[2], got, name, want)
		}
		checked++
	}
	if checked < 50 {
		t.Fatalf("checked only %d constants", checked)
	}
}

// TestOverlayProtocolMatchesJava holds Protocol.java's overlay constants to Go's: the window commands writeWindow sends, what a SURFACE message reports, and the driver's permissions and ends.
func TestOverlayProtocolMatchesJava(t *testing.T) {
	java, err := os.ReadFile(filepath.Join("java", "dev", "oneo", "nsgo", "Protocol.java"))
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]int{}
	for _, m := range regexp.MustCompile(`static final int (\w+) = (\d+);`).FindAllStringSubmatch(string(java), -1) {
		values[m[1]], _ = strconv.Atoi(m[2])
	}
	for name, want := range map[string]int{
		"WINDOW_NONE": windowNone, "WINDOW_FRAME": windowFrame, "WINDOW_SHOW": windowShow, "WINDOW_HIDE": windowHide,
		"WINDOW_FRONT": windowFront, "WINDOW_DESTROY": windowDestroy,
		"SURFACE_STATE": surfaceState, "SURFACE_ENDED": surfaceEnded,
		"OVERLAY_DENIED": driver.OverlayDenied, "OVERLAY_GRANTED": driver.OverlayGranted, "OVERLAY_UNAVAILABLE": driver.OverlayUnavailable,
		"OVERLAY_END_DENIED": driver.OverlayEndDenied, "OVERLAY_END_UNAVAILABLE": driver.OverlayEndUnavailable,
		"OVERLAY_END_REVOKED": driver.OverlayEndRevoked, "OVERLAY_END_REMOVED": driver.OverlayEndRemoved,
		"OVERLAY_END_FAILED": driver.OverlayEndFailed,
	} {
		if got, ok := values[name]; !ok || got != want {
			t.Errorf("Protocol.java's %s is %d (found %v), Go's %d", name, got, ok, want)
		}
	}
}

// TestOverlayItemsEncode checks the fields ItemOpenSurface and ItemWindow send, in the order Overlays and NsgoView.apply read them.
func TestOverlayItemsEncode(t *testing.T) {
	var w writer
	writeSurface(&w, &driver.SurfaceConfig{ID: 3, Kind: driver.SurfaceOverlay, Width: 200, Height: 100,
		Frame: driver.WindowFrame{X: 10, Y: 20, Move: true}, Focusable: true, Accessible: true})
	r := reader{w.buf}
	if id, width, height, x, y, flags := r.u32(), r.points(), r.points(), r.points(), r.points(), r.byte(); id != 3 ||
		width != 200 || height != 100 || x != 10 || y != 20 || flags != 1|2|8|32 || len(r.buf) != 0 {
		t.Errorf("surface %d %v %v %v %v %b, %d left", id, width, height, x, y, flags, len(r.buf))
	}
	for _, c := range []struct {
		cmd   driver.WindowCommand
		op    byte
		flags byte
	}{
		{driver.WindowCommand{Action: driver.WindowSetFrame, Frame: driver.WindowFrame{X: 5, Y: 6, Move: true}}, windowFrame, 1},
		{driver.WindowCommand{Action: driver.WindowSetFrame, Frame: driver.WindowFrame{Width: 50, Height: 60}}, windowFrame, 2},
		{driver.WindowCommand{Action: driver.WindowShow}, windowShow, 0},
		{driver.WindowCommand{Action: driver.WindowHide}, windowHide, 0},
		{driver.WindowCommand{Action: driver.WindowOrderFront}, windowFront, 0},
		{driver.WindowCommand{Action: driver.WindowDestroy}, windowDestroy, 0},
		{driver.WindowCommand{Action: driver.WindowMinimize}, windowNone, 0},
	} {
		var w writer
		writeWindow(&w, &c.cmd)
		r := reader{w.buf}
		op := r.byte()
		x, y, width, height := r.points(), r.points(), r.points(), r.points()
		flags := r.byte()
		f := c.cmd.Frame
		if op != c.op || flags != c.flags || x != f.X || y != f.Y || width != f.Width || height != f.Height || len(r.buf) != 0 {
			t.Errorf("%v: op %d flags %b frame %v %v %v %v", c.cmd.Action, op, flags, x, y, width, height)
		}
	}
}

// TestJavaMessagesMatchTheBackend holds Protocol.java's message types for package microphone's recordings and the web views' cookies to android.go's, which carry them both ways.
func TestJavaMessagesMatchTheBackend(t *testing.T) {
	read := func(path ...string) string {
		data, err := os.ReadFile(filepath.Join(path...))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	goSource, java := read("android.go"), read("java", "dev", "oneo", "nsgo", "Protocol.java")
	for goName, javaName := range map[string]string{"msgRecording": "RECORDING", "msgRecordingReport": "RECORDING_REPORT", "msgCookies": "COOKIES", "msgCookiesSet": "COOKIES_SET"} {
		g := regexp.MustCompile(`\b` + goName + ` += (\d+)`).FindStringSubmatch(goSource)
		j := regexp.MustCompile(`static final int ` + javaName + ` = (\d+);`).FindStringSubmatch(java)
		if g == nil || j == nil || g[1] != j[1] {
			t.Errorf("android.go's %s is %v, Protocol.java's %s %v", goName, g, javaName, j)
		}
	}
}

// TestWebCommandsMatchJava holds Protocol.java's web commands to the driver's, which WebNode.command switches on: WebSendInput, which Android does without, sits between them.
func TestWebCommandsMatchJava(t *testing.T) {
	java, err := os.ReadFile(filepath.Join("java", "dev", "oneo", "nsgo", "Protocol.java"))
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]int{}
	for _, m := range regexp.MustCompile(`static final int (WEB_\w+) = (\d+);`).FindAllStringSubmatch(string(java), -1) {
		values[m[1]], _ = strconv.Atoi(m[2])
	}
	for name, want := range map[string]driver.WebCommand{
		"WEB_RELOAD": driver.WebReload, "WEB_GO_BACK": driver.WebGoBack, "WEB_GO_FORWARD": driver.WebGoForward,
		"WEB_STOP_LOADING": driver.WebStopLoading, "WEB_POST_MESSAGE": driver.WebPostMessage,
		"WEB_INJECT_JAVASCRIPT": driver.WebInjectJavaScript, "WEB_LOAD": driver.WebLoad,
	} {
		if got, ok := values[name]; !ok || got != int(want) {
			t.Errorf("Protocol.java's %s is %d (found %v), the driver's %d", name, got, ok, want)
		}
	}
}
