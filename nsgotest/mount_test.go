package nsgotest

import (
	"fmt"
	"image"
	"slices"
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestRefFocusInUnplacedHolder(t *testing.T) {
	ref, scroll := &core.Ref{}, &core.Ref{}
	var shown, content *core.Signal[bool]
	app := Start(t, func(cx *core.Scope) core.Node {
		shown, content = core.NewSignal(cx, true), core.NewSignal(cx, false)
		held := core.NewHolder(cx, func(cx *core.Scope) core.Node {
			return core.Show{When: content.Get, Then: func(*core.Scope) core.Node {
				return core.Scroll{Ref: scroll, Style: core.Style{Height: core.Pt(100)}, Children: []core.Node{
					core.Input{Ref: ref, TestID: "field"}, core.View{Style: core.Style{Height: core.Pt(500)}},
				}}
			}}
		})
		return core.Show{When: shown.Get, Then: func(*core.Scope) core.Node { return held.Place() }}
	})
	app.Post(func() { shown.Set(false) })
	app.Settle()
	app.Post(func() { content.Set(true) })
	app.Settle()
	app.Post(func() { ref.Focus(); ref.Blur(); scroll.ScrollToEnd(false) })
	app.Settle()
	app.Post(func() { shown.Set(true) })
	app.Settle()
	if app.Find("field").Focused {
		t.Fatal("detached focus was replayed")
	}
	app.Post(func() { ref.Focus(); scroll.ScrollTo(core.Point{Y: 20}, false) })
	app.Settle()
	if !app.Find("field").Focused {
		t.Fatal("placed input did not take focus")
	}
	// A scroll queued just before detaching is dropped; it must not leave an unacknowledged scroll count that offsets later reports forever.
	app.Post(func() { scroll.ScrollToEnd(false); shown.Set(false) })
	app.Settle()
	app.Post(func() { shown.Set(true); scroll.ScrollTo(core.Point{Y: 30}, false) })
	app.Settle()
	if got := app.Find("field").Parent.Offset.Y; got != 30 {
		t.Fatalf("new scroll offset = %v, want 30", got)
	}
}

// Every target pulls a resting scroll view back when content shrinks under its end or the viewport grows past it, and the app hears where it went.
func TestScrollPastItsEndComesBackToIt(t *testing.T) {
	scroll := &core.Ref{}
	var content, viewport *core.Signal[core.Points]
	var heard core.Points
	app := Start(t, func(cx *core.Scope) core.Node {
		content, viewport = core.NewSignal(cx, core.Points(500)), core.NewSignal(cx, core.Points(100))
		return core.Scroll{
			TestID: "scroll", Ref: scroll,
			OnScroll:  func(e core.ScrollEvent) { heard = e.Offset.Y },
			StyleFunc: func() core.Style { return core.Style{Height: core.Pt(viewport.Get())} },
			Children:  []core.Node{core.View{StyleFunc: func() core.Style { return core.Style{Height: core.Pt(content.Get())} }}},
		}
	})
	app.Post(func() { scroll.ScrollToEnd(false) })
	app.Settle()
	if heard != 400 {
		t.Fatalf("the app heard offset %v before the resize, want 400", heard)
	}
	for _, step := range []struct {
		name    string
		change  func()
		wantEnd core.Points
	}{
		{"content shrank", func() { content.Set(300) }, 200},
		{"viewport grew", func() { viewport.Set(150) }, 150},
	} {
		app.Post(step.change)
		app.Settle()
		if got := app.Find("id:scroll").Offset.Y; got != step.wantEnd || heard != step.wantEnd {
			t.Errorf("%s: offset %v, the app heard %v, want the new end %v", step.name, got, heard, step.wantEnd)
		}
	}
}

func TestListPastItsEndComesBackToIt(t *testing.T) {
	var list core.Ref
	var shortened *core.Signal[bool]
	var heard core.Points
	app := Start(t, func(cx *core.Scope) core.Node {
		shortened = core.NewSignal(cx, false)
		return core.List{
			TestID: "list", Ref: &list,
			Count: func() int {
				if shortened.Get() {
					return 40
				}
				return 50
			},
			EstimatedSize: 10,
			Behavior:      core.ScrollBehavior{ContentInset: core.Edges{Bottom: 10}},
			Style:         core.Style{Height: core.Pt(100)},
			OnScroll:      func(e core.ScrollEvent) { heard = e.Offset.Y },
			Row: func(_ *core.Scope, row core.ListRow) core.Node {
				return core.View{StyleFunc: func() core.Style {
					if row.Index() == 30 && shortened.Get() {
						return core.Style{Height: core.Pt(60)}
					}
					return core.Style{Height: core.Pt(10)}
				}}
			},
		}
	})
	app.Post(func() { list.ScrollTo(core.Point{Y: 380}, false) })
	app.Settle()
	if heard != 380 {
		t.Fatalf("the app heard offset %v before removing rows, want 380", heard)
	}
	// Also grow a row above the viewport so the commit includes a relative scroll adjustment before the backend settles on the new end.
	app.Post(func() { shortened.Set(true) })
	app.Settle()
	if got := app.Find("id:list").Offset.Y; got != 360 || heard != 360 {
		t.Fatalf("after removing rows: offset %v, the app heard %v, want 360", got, heard)
	}
}

// A scroll view records the scrolls the app asks it for: a Ref's scroll to a point, and the scroll by a distance a
// List sends to keep its visible rows in place as rows land above them.
func TestScrollRecordsTheScrollsTheAppAsksFor(t *testing.T) {
	ids := func(n, from int) []int {
		out := make([]int, n)
		for i := range out {
			out[i] = from + i
		}
		return out
	}
	ref := &core.Ref{}
	var rows *core.Signal[[]int]
	app := Start(t, func(cx *core.Scope) core.Node {
		rows = core.NewSignalFunc(cx, ids(100, 0), nil)
		return core.List{TestID: "list", Ref: ref, Style: core.Style{Height: core.Pt(400)},
			Count: func() int { return len(rows.Get()) }, Key: func(i int) any { return rows.Get()[i] },
			EstimatedSize: 40, Row: func(*core.Scope, core.ListRow) core.Node {
				return core.View{Style: core.Style{Height: core.Pt(40)}}
			}}
	})
	app.Post(func() { ref.ScrollTo(core.Point{Y: 1000}, true) })
	app.Settle()
	app.Post(func() { rows.Set(append(ids(5, 1000), rows.Get()...)) })
	app.Settle()
	want := []ScrollCommand{{Point: driver.Point{Y: 1000}, Animated: true}, {Point: driver.Point{Y: 200}, By: true}}
	if got := app.Find("id:list").Scrolls; !slices.Equal(got, want) {
		t.Errorf("scrolls %+v, want %+v", got, want)
	}
}

func TestMountFailsWithItemAndMissingView(t *testing.T) {
	for _, item := range []struct {
		kind driver.ItemKind
		name string
	}{
		{driver.ItemFocus, "ItemFocus"}, {driver.ItemBlur, "ItemBlur"},
		{driver.ItemScrollTo, "ItemScrollTo"}, {driver.ItemScrollBy, "ItemScrollBy"},
		{driver.ItemFrame, "ItemFrame"}, {driver.ItemInsert, "ItemInsert"},
	} {
		for _, id := range []driver.NodeID{0, 1 << 30} {
			t.Run(fmt.Sprintf("%s/%d", item.name, id), func(t *testing.T) {
				msg := fails(t, func(tb testing.TB) {
					app := Start(tb, func(*core.Scope) core.Node { return core.View{} })
					app.apply(&driver.MountBatch{Items: []driver.MountItem{{Kind: item.kind, ID: id}}})
				})
				if !strings.Contains(msg, item.name) || !strings.Contains(msg, fmt.Sprintf("unknown view %d", id)) {
					t.Fatalf("missing-view diagnostic = %q", msg)
				}
			})
		}
	}
	msg := fails(t, func(tb testing.TB) {
		app := Start(tb, func(*core.Scope) core.Node { return core.View{TestID: "child", Style: core.Style{Height: core.Pt(20)}} })
		app.apply(&driver.MountBatch{Items: []driver.MountItem{{Kind: driver.ItemInsert, ID: app.Find("child").ID, Parent: 1 << 30}}})
	})
	if !strings.Contains(msg, "ItemInsert") || !strings.Contains(msg, "unknown parent 1073741824") {
		t.Fatalf("missing-parent diagnostic = %q", msg)
	}
}

func TestScrollEffectOnReplacedView(t *testing.T) {
	var page *core.Signal[int]
	scroll := &core.Ref{}
	effects := 0
	app := Start(t, func(cx *core.Scope) core.Node {
		page = core.NewSignal(cx, 0)
		return core.Dynamic(func(cx *core.Scope) core.Node {
			if page.Get() != 1 {
				return core.View{}
			}
			cx.Effect(func() { effects++; scroll.ScrollToEnd(false) })
			return core.Scroll{Ref: scroll, Children: []core.Node{core.Text{Content: "temporary"}}}
		})
	})
	app.Post(func() { page.Set(1); page.Set(2) })
	app.Settle()
	if effects != 1 {
		t.Fatalf("temporary view effect ran %d times, want 1", effects)
	}
}

func TestImageReportsThePixelsItShows(t *testing.T) {
	frame := image.NewRGBA(image.Rect(0, 0, 32, 18))
	var version *core.Signal[uint64]
	app := Start(t, func(cx *core.Scope) core.Node {
		version = core.NewSignal(cx, uint64(1))
		return core.Image{TestID: "video", SourceFunc: func() core.ImageSource {
			return core.ImageSource{Pixels: frame, Version: version.Get()}
		}}
	})
	v := app.Find("video")
	if src := v.Image.Source; src.Version != 1 || src.Pixels == nil || src.Pixels.Rect.Size() != (image.Point{X: 32, Y: 18}) {
		t.Fatalf("image shows %+v, want 32x18 pixels at version 1", src)
	}
	frame.Pix[0] = 7
	app.Post(func() { version.Set(2) })
	app.Settle()
	if src := app.Find("video").Image.Source; src.Version != 2 || src.Pixels.Pix[0] != 7 {
		t.Fatalf("image shows version %d, pixel %d, want the second frame", src.Version, src.Pixels.Pix[0])
	}
}
