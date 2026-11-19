package pick_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/nsgo-dev/nsgo"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

// pickApp is a button that asks for a photo and shows how many files came back.
func pickApp(cx *core.Scope) core.Node {
	got := core.NewSignal(cx, "none")
	return core.View{Children: []core.Node{
		core.View{Role: core.RoleButton, Label: "Pick", TestID: "pick", Style: core.Style{Width: core.Pt(80), Height: core.Pt(30)},
			OnPress: func(core.PressEvent) {
				cx.PickImages(core.PickOptions{Limit: 1}, func(files []core.PickedFile) {
					got.Set(strings.TrimSpace(fmt.Sprintf("picked %d %s", len(files), names(files))))
				})
			}},
		core.Text{ContentFunc: got.Get},
	}}
}

func names(files []core.PickedFile) string {
	s := ""
	for _, f := range files {
		s += f.Name
	}
	return s
}

func skipOtherTargets(t *testing.T) {
	switch nsgotest.Target() {
	case "", "macos", "windows", "linux":
	default:
		t.Skip("PickImages answers headless and on the desktops; the other targets show the system's picker")
	}
}

// TestPickImagesAnswersTheApp checks that App.PickImages, set after Start as a test sets it, answers the app's pick in place of the system's picker, headless and on a desktop target alike (#866).
func TestPickImagesAnswersTheApp(t *testing.T) {
	skipOtherTargets(t)
	app := nsgotest.Start(t, pickApp)
	called := 0
	app.PickImages = func(core.PickRequest) []core.PickedFile {
		called++
		return []core.PickedFile{{Name: "a.png", MIME: "image/png", Data: []byte("png")}}
	}
	app.Press("id:pick")
	app.WaitFor("text:picked 1 a.png", 5*time.Second)
	if called != 1 || len(app.Picks) != 1 || app.Picks[0].Limit != 1 {
		t.Fatalf("PickImages was asked %d times for %+v, want once for a limit of 1", called, app.Picks)
	}
}

// TestPickImagesWithoutAnAnswerCancels checks that without App.PickImages the user cancels, headless and on a desktop target alike, rather than the system's picker opening over the test.
func TestPickImagesWithoutAnAnswerCancels(t *testing.T) {
	skipOtherTargets(t)
	app := nsgotest.Start(t, pickApp)
	app.Press("id:pick")
	app.WaitFor("text:picked 0", 5*time.Second)
}
