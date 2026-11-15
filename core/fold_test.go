package core

import (
	"reflect"
	"testing"
	"time"
)

func frame(x, y, w, h Points) Bounds {
	return Bounds{Origin: Point{X: x, Y: y}, Size: Size{Width: w, Height: h}}
}

// hinge is an open hinge between two panels of a 800 by 600 surface: occluding and separating at once.
var hinge = Obstruction{Frame: frame(390, 0, 20, 600), Occluding: true, Separating: true, State: HingeOpen}

func TestGeometryDefaultsToOneRegion(t *testing.T) {
	var got Geometry
	h := newHost(t, Size{Width: 800, Height: 600}, func(cx *Scope) Node {
		got = cx.Env().Geometry()
		return View{}
	})
	h.commit()
	want := Geometry{Regions: []Region{{Frame: frame(0, 0, 800, 600)}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("geometry %+v, want %+v", got, want)
	}
}

func TestGeometryChangesWithTheSize(t *testing.T) {
	// What reads the size and the geometry sees both new in one run, never one of them at the other's old value.
	type seen struct {
		size Size
		g    Geometry
	}
	var runs []seen
	h := newHost(t, Size{Width: 400, Height: 600}, func(cx *Scope) Node {
		cx.Effect(func() { runs = append(runs, seen{cx.Env().Window(), cx.Env().Geometry()}) })
		return View{}
	})
	h.commit()
	unfolded := Geometry{Obstructions: []Obstruction{hinge}}
	h.tree.Resize(Size{Width: 800, Height: 600}, unfolded)
	h.commit()
	if len(runs) != 2 {
		t.Fatalf("%d runs, want 2: %+v", len(runs), runs)
	}
	last := runs[1]
	if last.size != (Size{Width: 800, Height: 600}) || len(last.g.Regions) != 2 || len(last.g.Obstructions) != 1 {
		t.Fatalf("after the resize: %+v", last)
	}
	if last.g.Regions[1].Frame != frame(410, 0, 390, 600) {
		t.Errorf("right region %v, want right of the hinge", last.g.Regions[1].Frame)
	}

	// A fold half closing at the same size still applies.
	half := hinge
	half.State = HingeHalfOpen
	h.tree.Resize(Size{Width: 800, Height: 600}, Geometry{Obstructions: []Obstruction{half}})
	h.commit()
	if len(runs) != 3 || runs[2].g.Obstructions[0].State != HingeHalfOpen {
		t.Fatalf("a new state at the same size: %+v", runs[2:])
	}
}

func TestGeometryChangesOnlyWithItself(t *testing.T) {
	var runs int
	h := newHost(t, Size{Width: 800, Height: 600}, func(cx *Scope) Node {
		cx.Effect(func() { cx.Env().Geometry(); runs++ })
		return View{}
	})
	h.commit()
	h.tree.Resize(Size{Width: 800, Height: 600}, Geometry{Obstructions: []Obstruction{hinge}})
	h.commit()
	if runs != 2 {
		t.Fatalf("%d runs after a hinge appeared, want 2", runs)
	}
	// The same geometry again, the keyboard, the appearance and the lifecycle leave it as it was.
	h.tree.Resize(Size{Width: 800, Height: 600}, Geometry{Obstructions: []Obstruction{hinge}})
	h.tree.SetEnvironment(Environment{KeyboardHeight: 300, KeyboardDuration: 250 * time.Millisecond})
	h.tree.SetEnvironment(Environment{KeyboardHeight: 300, Dark: true, Lifecycle: LifecycleInactive})
	h.commit()
	if runs != 2 {
		t.Fatalf("%d runs after changes that leave the geometry, want 2", runs)
	}
}

func TestGeometryRegionsTakeTheSafeArea(t *testing.T) {
	var got Geometry
	h := newHost(t, Size{Width: 800, Height: 600}, func(cx *Scope) Node {
		cx.Effect(func() { got = cx.Env().Geometry() })
		return View{}
	})
	h.tree.Resize(Size{Width: 800, Height: 600}, Geometry{Obstructions: []Obstruction{hinge}})
	h.tree.SetEnvironment(Environment{SafeArea: Edges{Top: 40, Left: 30, Bottom: 20}})
	h.commit()
	want := []Edges{{Top: 40, Left: 30, Bottom: 20}, {Top: 40, Bottom: 20}}
	for i, r := range got.Regions {
		if r.SafeArea != want[i] {
			t.Errorf("region %d's safe area %+v, want %+v", i, r.SafeArea, want[i])
		}
	}
	// A caller changing what it got leaves the environment's alone.
	got.Regions[0].Frame = Bounds{}
	if again := h.tree.root.Env().Geometry(); again.Regions[0].Frame != frame(0, 0, 390, 600) {
		t.Errorf("the environment's geometry changed with a copy: %+v", again.Regions[0])
	}
}

func TestGeometryOrdersRegionsAndReserves(t *testing.T) {
	var got Geometry
	h := newHost(t, Size{Width: 800, Height: 600}, func(cx *Scope) Node {
		cx.Effect(func() { got = cx.Env().Geometry() })
		return View{}
	})
	// A fold whose pixels do not show and which does not separate, given before one that does: both go by position, the first reserving its frame in the region it lies in.
	covered := Obstruction{Frame: frame(0, 290, 800, 20), Horizontal: true, Occluding: true, State: HingeOpen}
	h.tree.Resize(Size{Width: 800, Height: 600}, Geometry{Obstructions: []Obstruction{covered, {Frame: frame(400, 0, 0, 600), Separating: true, State: HingeHalfOpen}}})
	h.commit()
	want := []Region{
		{Frame: frame(0, 0, 400, 600), Reserved: frame(0, 290, 400, 20)},
		{Frame: frame(400, 0, 400, 600), Reserved: frame(400, 290, 400, 20)},
	}
	if !reflect.DeepEqual(got.Regions, want) {
		t.Fatalf("regions %+v, want %+v", got.Regions, want)
	}
	if !got.Obstructions[0].Separating || got.Obstructions[1].Separating {
		t.Errorf("obstructions %+v, want the one whose top is higher first", got.Obstructions)
	}
}

func TestGeometryStaysOnADetachedSurface(t *testing.T) {
	// A window's surface keeps the geometry it last had while nothing reports to it.
	var got Geometry
	h := newHost(t, Size{Width: 800, Height: 600}, func(cx *Scope) Node {
		cx.Effect(func() { got = cx.Env().Geometry() })
		return View{}
	})
	h.tree.Resize(Size{Width: 800, Height: 600}, Geometry{Obstructions: []Obstruction{hinge}})
	h.commit()
	h.tree.SetEnvironment(Environment{Lifecycle: LifecycleBackground})
	h.commit()
	if len(got.Obstructions) != 1 || len(got.Regions) != 2 {
		t.Fatalf("geometry %+v, want the hinge kept", got)
	}
}

func TestPanesOnEitherSideOfAFold(t *testing.T) {
	// An app places two panes in the regions a separating fold leaves; core adds no gap and moves nothing of its own.
	h := newHost(t, Size{Width: 800, Height: 600}, func(cx *Scope) Node {
		pane := func(i int) Node {
			return View{TestID: []string{"list", "detail"}[i], StyleFunc: func() Style {
				regions := cx.Env().Geometry().Regions
				f := regions[min(i, len(regions)-1)].Frame
				if len(regions) == 1 && i == 1 {
					return Style{Display: DisplayNone}
				}
				return Style{Position: Absolute, Left: Pt(f.Origin.X), Top: Pt(f.Origin.Y),
					Width: Pt(f.Size.Width), Height: Pt(f.Size.Height)}
			}}
		}
		return View{Style: Style{FlexGrow: 1}, TestID: "root", Children: []Node{pane(0), pane(1)}}
	})
	h.commit()
	if got := h.find("list").frame; got != frame(0, 0, 800, 600) {
		t.Fatalf("one region: list %v, want the whole surface", got)
	}
	h.tree.Resize(Size{Width: 800, Height: 600}, Geometry{Obstructions: []Obstruction{hinge}})
	h.commit()
	for id, want := range map[string]Bounds{"root": frame(0, 0, 800, 600), "list": frame(0, 0, 390, 600), "detail": frame(410, 0, 390, 600)} {
		if got := h.find(id).frame; got != want {
			t.Errorf("%s = %v, want %v", id, got, want)
		}
	}
}
