//go:build linux && !android

package linux

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego"
	"github.com/nsgo-dev/nsgo/core"
)

// TestAccessibilitySelectedAndExpandedReachATSPI reads the states through the same AT-SPI interface a screen reader uses, on a private session bus so the test sees only its own window.
func TestAccessibilitySelectedAndExpandedReachATSPI(t *testing.T) {
	if want := os.Getenv("NSGO_TEST_ACCESS_WANT"); want != "" {
		probeAccessibilityStates(t, want == "true")
		return
	}
	if os.Getenv("NSGO_TEST_ACCESS_PROCESS") != t.Name() {
		bus, err := exec.LookPath("dbus-run-session")
		if err != nil {
			t.Skip("needs dbus-run-session")
		}
		cmd := exec.Command(bus, "--", os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v", "-test.timeout=45s")
		cmd.Env = append(os.Environ(), "NSGO_TEST_ACCESS_PROCESS="+t.Name(), "GTK_A11Y=atspi")
		runAccessibilityProcess(t, cmd)
		return
	}
	var update func(bool)
	w := startTestWindow(t, func(cx *core.Scope) core.Node {
		on := core.NewSignal(cx, true)
		update = func(v bool) { cx.Post(func() { on.Set(v) }) }
		cx.Effect(func() { cx.SetTitle(fmt.Sprintf("access-states|%v", on.Get())) })
		return core.View{Role: core.RoleTreeItem, Label: accessTestLabel, Selected: on.Get, Expanded: on.Get,
			Style: core.Style{Width: core.Pt(100), Height: core.Pt(100)}}
	}, Options{Title: "access-states", Width: 150, Height: 150})
	waitForTitle(t, w, "access-states|true")
	for _, want := range []bool{true, false, true} {
		update(want)
		waitForTitle(t, w, fmt.Sprintf("access-states|%v", want))
		// AT-SPI's synchronous calls iterate GLib's default context. Keep the client in another process so it cannot dispatch GTK callbacks on the test goroutine while the window's thread uses that context.
		cmd := exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$", "-test.v", "-test.timeout=15s")
		cmd.Env = append(os.Environ(), fmt.Sprintf("NSGO_TEST_ACCESS_WANT=%v", want))
		runAccessibilityProcess(t, cmd)
	}
}

func runAccessibilityProcess(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if strings.Contains(string(out), "g_value_get_int:") {
		t.Fatalf("GTK rejected an accessibility state's type:\n%s", out)
	}
	if strings.Contains(string(out), "--- SKIP:") {
		t.Skip(string(out))
	}
}

const accessTestLabel = "NSGo selected and expanded regression"

func probeAccessibilityStates(t *testing.T, want bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	lib, err := purego.Dlopen("libatspi.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Skip("needs libatspi: ", err)
	}
	var init func() int32
	var desktop func(int32) uintptr
	var name func(uintptr, *uintptr) uintptr
	var count func(uintptr, *uintptr) int32
	var child func(uintptr, int32, *uintptr) uintptr
	var states func(uintptr) uintptr
	var contains func(uintptr, int32) int32
	var clear func(uintptr)
	var retain func(uintptr) uintptr
	purego.RegisterLibFunc(&init, lib, "atspi_init")
	purego.RegisterLibFunc(&desktop, lib, "atspi_get_desktop")
	purego.RegisterLibFunc(&name, lib, "atspi_accessible_get_name")
	purego.RegisterLibFunc(&count, lib, "atspi_accessible_get_child_count")
	purego.RegisterLibFunc(&child, lib, "atspi_accessible_get_child_at_index")
	purego.RegisterLibFunc(&states, lib, "atspi_accessible_get_state_set")
	purego.RegisterLibFunc(&contains, lib, "atspi_state_set_contains")
	purego.RegisterLibFunc(&clear, lib, "atspi_accessible_clear_cache")
	purego.RegisterLibFunc(&retain, gobjectLib, "g_object_ref")
	if init() != 0 {
		t.Fatal("AT-SPI could not connect to the private bus")
	}
	// GTK registers with the accessibility bus asynchronously.
	var find func(uintptr, int) uintptr
	find = func(node uintptr, depth int) uintptr {
		if node == 0 || depth > 10 {
			return 0
		}
		clear(node)
		s := name(node, nil)
		got := goString(s)
		gFree(s)
		if got == accessTestLabel {
			retain(node)
			return node
		}
		for i, n := int32(0), count(node, nil); i < n; i++ {
			c := child(node, i, nil)
			found := find(c, depth+1)
			if c != 0 {
				gObjectUnref(c)
			}
			if found != 0 {
				return found
			}
		}
		return 0
	}
	var node uintptr
	for deadline := time.Now().Add(10 * time.Second); node == 0; time.Sleep(20 * time.Millisecond) {
		root := desktop(0)
		node = find(root, 0)
		if root != 0 {
			gObjectUnref(root)
		}
		if time.Now().After(deadline) {
			t.Fatal("the labelled view did not appear on the AT-SPI bus")
		}
	}
	defer gObjectUnref(node)
	// AtspiStateType's stable protocol values.
	const expandable, expanded, selected = 9, 10, 23
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		clear(node)
		set := states(node)
		e, x, s := contains(set, expandable) != 0, contains(set, expanded) != 0, contains(set, selected) != 0
		gObjectUnref(set)
		// A collapsed view must stay expandable: its expanded state is false, not unset.
		if e && x == want && s == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("AT-SPI expandable %v, expanded %v, selected %v; want expandable and both %v", e, x, s, want)
		}
	}
}
