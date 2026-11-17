package core

import (
	"reflect"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func selectorTestSVG() *SVG {
	return MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path d="M0 0H24V24H0Z"/></svg>`))
}

func TestRelationalIconClasses(t *testing.T) {
	var direct, nested *Signal[bool]
	h := newHost(t, Size{Width: 400, Height: 300}, func(cx *Scope) Node {
		direct, nested = NewSignal(cx, false), NewSignal(cx, true)
		return View{TestID: "button", Class: "px-8 has-[>svg]:px-3 [&_svg]:size-4 [&_svg]:shrink-0", Children: []Node{
			Show{When: direct.Get, Then: func(*Scope) Node { return Icon{TestID: "direct", SVG: selectorTestSVG(), Class: "size-8 shrink"} }},
			View{Children: []Node{Show{When: nested.Get, Then: func(*Scope) Node { return SVGView{TestID: "nested", SVG: selectorTestSVG()} }}}},
			Canvas{TestID: "canvas", Class: "size-8"},
		}}
	})
	check := func(padding Points) {
		t.Helper()
		h.commit()
		if got := h.find("button").style.Padding; got.Left != padding || got.Right != padding {
			t.Fatalf("padding=%v, want %v", got, padding)
		}
		if got := h.find("canvas").style.Width; got != Pt(32) {
			t.Fatalf("canvas matched svg: %v", got)
		}
		for _, id := range []string{"direct", "nested"} {
			if id == "direct" && !direct.Get() || id == "nested" && !nested.Get() {
				continue
			}
			n := h.find(id)
			if n.style.Width != Pt(16) || n.style.Height != Pt(16) || n.style.FlexShrink != 0 {
				t.Fatalf("%s style=%+v", id, n.style)
			}
		}
	}
	check(32) // A grandchild does not satisfy has-[>svg].
	direct.Set(true)
	check(12) // A Show's region is transparent.
	nested.Set(false)
	check(12)
	direct.Set(false)
	check(32)
}

func TestRelationalStatesAndVariantTargets(t *testing.T) {
	var checked, disabled *Signal[bool]
	h := newHost(t, Size{Width: 400, Height: 300}, func(cx *Scope) Node {
		checked, disabled = NewSignal(cx, false), NewSignal(cx, false)
		return View{TestID: "parent", Class: "p-8 has-[:checked]:p-3 hover:[&_svg]:size-6 [&_svg]:hover:size-7 [&_[aria-invalid=true]]:opacity-50", Children: []Node{
			View{TestID: "check", Checked: checked.Get, Disabled: disabled.Get, Invalid: checked.Get},
			Icon{TestID: "icon", SVG: selectorTestSVG(), Class: "size-4"},
		}}
	})
	h.commit()
	if h.find("parent").style.Padding.Top != 32 {
		t.Fatal("has matched an unchecked child")
	}
	checked.Set(true)
	h.commit()
	if h.find("parent").style.Padding.Top != 12 || h.find("check").style.Opacity != Alpha(.5) {
		t.Fatal("child state did not update its parent and target")
	}
	parent, icon := h.find("parent"), h.find("icon")
	if !parent.events().Has(driver.EventMouseEnter) || !icon.events().Has(driver.EventMouseEnter) {
		t.Fatal("selector state dependencies were not mounted")
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseEnter, ID: parent.id})
	h.commit()
	if icon.style.Width != Pt(24) {
		t.Fatalf("parent hover sets %v, want 24", icon.style.Width)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseEnter, ID: icon.id})
	h.commit()
	if icon.style.Width != Pt(28) {
		t.Fatalf("target hover sets %v, want 28", icon.style.Width)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseLeave, ID: parent.id})
	h.commit()
	if icon.style.Width != Pt(28) {
		t.Fatal("target hover depended on its parent's hover")
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseLeave, ID: icon.id})
	h.commit()
	if icon.style.Width != Pt(16) {
		t.Fatal("hover selector failed to clear")
	}
	checked.Set(false)
	h.commit()
	if parent.style.Padding.Top != 32 || h.find("check").style.Opacity != (Opacity{}) {
		t.Fatal("has/attribute style failed to clear")
	}
}

func TestRelationalCascadeAndClassReplacement(t *testing.T) {
	var active *Signal[bool]
	h := newHost(t, Size{Width: 400, Height: 300}, func(cx *Scope) Node {
		active = NewSignal(cx, true)
		return View{TestID: "source", ClassFunc: func() string {
			if active.Get() {
				return "[&_svg]:size-6 [&_svg]:text-red-500 [&_svg]:p-3!"
			}
			return ""
		}, Children: []Node{
			Icon{TestID: "class", SVG: selectorTestSVG(), Class: "size-4 hover:size-8 p-1"},
			Icon{TestID: "inline", SVG: selectorTestSVG(), Class: "size-4", Style: Style{Width: Pt(40), Padding: All(20)}},
		}}
	})
	h.commit()
	class, inline := h.find("class"), h.find("inline")
	if class.style.Width != Pt(24) || inline.style.Width != Pt(40) || inline.style.Padding.Top != 12 {
		t.Fatal("selector cascade did not respect type specificity, inline style and important")
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseEnter, ID: class.id})
	h.commit()
	if class.style.Width != Pt(32) {
		t.Fatalf("more-specific local hover lost: %v", class.style.Width)
	}
	active.Set(false)
	h.commit()
	if class.style.Width != Pt(32) || class.style.Padding.Top != 4 || inline.style.Padding.Top != 20 || class.style.Color != 0 {
		t.Fatal("removing the source left derived styles")
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseLeave, ID: class.id})
	h.commit()
	if class.style.Width != Pt(16) {
		t.Fatal("local classes stopped following states after source removal")
	}
}

func TestRelationalSelectorGrammar(t *testing.T) {
	valid := []string{"has-[>svg]:px-3", "[&_svg]:size-4", "[&>svg:not(:first-child)]:ml-2", "[&_[data-part=icon]]:size-4", "[&:has(>svg)]:p-2", "[&_view]:[&>svg]:size-5", "has-checked:p-1"}
	for _, token := range valid {
		if got := UnknownClasses(token); len(got) != 0 {
			t.Errorf("valid %s is unknown: %v", token, got)
		}
	}
	invalid := []string{"has-[]:p-2", "[&>]:p-2", "[&+~svg]:p-2", "[&_svg::before]:p-2", "[&_svg:made-up]:p-2", "[&_svg[onclick=x]]:p-2", "[&_svg:not()]:p-2", "[&_[data-x=]]:p-2", "[&+&]:p-2", "[&:is(&)]:p-2", "has-[&]:p-2", "[&>svg,view]:p-2", "placeholder:[&_svg]:p-2"}
	for _, token := range invalid {
		if got := UnknownClasses(token); !reflect.DeepEqual(got, []string{token}) {
			t.Errorf("invalid %s accepted: %v", token, got)
		}
	}
}

func TestRelationalClassesFollowMovesAndSiblingChanges(t *testing.T) {
	var root, left, right, content, marker, sibling, icon *Element
	h := newHost(t, Size{Width: 400, Height: 300}, func(cx *Scope) Node {
		root = NewElement(cx, nil)
		left = NewElement(cx, View{TestID: "left", Class: "p-8 has-[>svg]:p-3 [&_svg]:size-6"})
		right = NewElement(cx, View{TestID: "right", Class: "[&_svg]:size-5"})
		content = NewElement(cx, nil)
		content.Mount(Icon{TestID: "moving", SVG: selectorTestSVG()})
		left.Insert(content, nil)
		marker = NewElement(cx, View{TestID: "shrink-0", Class: "[&+view]:p-3 has-[~svg]:opacity-50"})
		sibling = NewElement(cx, View{TestID: "sibling", Class: "p-8"})
		icon = NewElement(cx, nil)
		icon.Mount(Icon{TestID: "last", SVG: selectorTestSVG()})
		for _, item := range []*Element{left, right, marker, sibling, icon} {
			root.Insert(item, nil)
		}
		return root
	})
	h.commit()
	id := h.find("moving").id
	if h.find("left").style.Padding.Top != 12 || h.find("moving").style.Width != Pt(24) || h.find("sibling").style.Padding.Top != 12 || h.find("shrink-0").style.Opacity != Alpha(.5) {
		t.Fatal("initial descendant/sibling selectors did not match")
	}
	right.Insert(content, nil)
	h.commit()
	if h.find("moving").id != id || h.find("moving").style.Width != Pt(20) || h.find("left").style.Padding.Top != 32 {
		t.Fatal("moving a region did not remove the old selector and apply the new one")
	}
	left.Set(View{TestID: "left", Class: "p-8"})
	left.Insert(content, nil)
	h.commit()
	if h.find("moving").style.Width != (Length{}) {
		t.Fatalf("moved icon kept its former selector size: %v", h.find("moving").style.Width)
	}
	root.Remove(icon)
	h.commit()
	if h.find("shrink-0").style.Opacity != (Opacity{}) {
		t.Fatal("relative has kept a removed sibling")
	}
	marker.Set(View{TestID: "shrink-0"})
	h.commit()
	if h.find("sibling").style.Padding.Top != 32 {
		t.Fatal("removing a sibling selector left its target styled")
	}
}

func TestRelationalEnvironmentAndStackedSelectors(t *testing.T) {
	var themeScope *Scope
	h := newHost(t, Size{Width: 400, Height: 300}, func(cx *Scope) Node {
		themeScope = cx
		cx.SetTheme(DefaultTheme())
		return View{Class: "[&_view]:[&>svg]:size-5 dark:[&_svg]:text-white [&_svg]:text-black [&_svg]:border-accent [&_view[data-part=icon]]:p-3", Children: []Node{
			View{TestID: "wrapper", PartStates: func() PartStates { return PartStates{Data: []string{"part=icon"}} }, Children: []Node{
				Icon{TestID: "stacked", SVG: selectorTestSVG()},
			}},
			Icon{TestID: "unstacked", SVG: selectorTestSVG()},
		}}
	})
	h.commit()
	if h.find("stacked").style.Width != Pt(20) || h.find("unstacked").style.Width != (Length{}) || h.find("wrapper").style.Padding.Top != 12 {
		t.Fatal("stacked selector or data value selected the wrong targets")
	}
	if h.find("stacked").style.Color != RGB(0) {
		t.Fatal("light selector color")
	}
	h.tree.SetEnvironment(driver.Environment{Dark: true})
	h.commit()
	if h.find("stacked").style.Color != RGB(0xffffff) {
		t.Fatal("a target with no own classes did not follow appearance")
	}
	// The target reads its own subtree's theme, including after its first inherited rule created the class state for it.
	theme := *DefaultTheme()
	theme.Dark = theme.Light
	theme.Dark.Accent = RGB(0x123456)
	themeScope.SetTheme(&theme)
	h.commit()
	if h.find("stacked").style.Width != Pt(20) || h.find("stacked").style.BorderColor != RGB(0x123456) {
		t.Fatalf("theme replacement: width=%v border=%x envAccent=%x dark=%v", h.find("stacked").style.Width, h.find("stacked").style.BorderColor, h.find("stacked").cls.env.palette.Accent, h.find("stacked").cls.env.dark)
	}
}

func TestRelationalSpacingAndNestedConditions(t *testing.T) {
	var source, child *Element
	h := newHost(t, Size{Width: 400, Height: 300}, func(cx *Scope) Node {
		source = NewElement(cx, View{TestID: "source", Class: "has-checked:space-x-3 [&>view:not(:has(>svg))]:p-2 [&>view:is(.shrink-0,[data-part='a)b]'])]:opacity-50 [&>view:first-child:last-child]:m-4"})
		child = NewElement(cx, View{TestID: "first", Class: "shrink-0", Checked: func() bool { return true }})
		source.Insert(child, nil)
		source.Insert(NewElement(cx, View{TestID: "second"}), nil)
		return source
	})
	h.commit()
	if got := h.find("first").style; got.Margin.Right != 12 || got.Padding.Top != 8 || got.Opacity != Alpha(.5) || got.Margin.Top != 0 {
		t.Fatalf("combined conditions/spacing: %+v", got)
	}
	// Imperative props must invalidate has even without a signal binding.
	child.Set(View{TestID: "first"})
	h.commit()
	if got := h.find("first").style; got.Margin.Right != 0 || got.Opacity != (Opacity{}) {
		t.Fatalf("cleared checked/class left derived spacing or opacity: %+v", got)
	}
	icon := NewElement(child.n.owner, nil)
	icon.Mount(Icon{SVG: selectorTestSVG()})
	child.Insert(icon, nil)
	h.commit()
	if got := h.find("first").style.Padding.Top; got != 0 {
		t.Fatalf("nested :not(:has()) stayed matched: %v", got)
	}
	source.Set(View{TestID: "source"})
	h.commit()
	if len(h.tree.relationSources) != 0 {
		t.Fatal("removed class source was retained")
	}
}

func TestRelationalStateBackgroundsKeepGradientBehavior(t *testing.T) {
	h := newHost(t, Size{Width: 400, Height: 300}, func(*Scope) Node {
		return View{Class: "[&_svg]:p-1", Children: []Node{
			Icon{TestID: "background", SVG: selectorTestSVG(), Class: "bg-linear-to-b from-red-500 to-blue-500 hover:bg-black"},
			Icon{TestID: "stop", SVG: selectorTestSVG(), Class: "bg-linear-to-b from-red-500 to-blue-500 hover:bg-black hover:from-green-500"},
		}}
	})
	h.commit()
	for _, id := range []string{"background", "stop"} {
		n := h.find(id)
		if len(n.style.Gradients) != 1 {
			t.Fatal("initial gradient missing")
		}
		h.tree.Dispatch(driver.Event{Kind: driver.EventMouseEnter, ID: n.id})
	}
	h.commit()
	if n := h.find("background"); n.style.Background != RGB(0) || len(n.style.Gradients) != 0 {
		t.Fatal("flattened hover background failed to cover the gradient")
	}
	if n := h.find("stop"); len(n.style.Gradients) != 1 || n.style.Gradients[0].Stops[0].Color != tailwindPalette["green"][5] {
		t.Fatal("flattened hover stop failed to inherit the gradient")
	}
	for _, id := range []string{"background", "stop"} {
		h.tree.Dispatch(driver.Event{Kind: driver.EventMouseLeave, ID: h.find(id).id})
	}
	h.commit()
	if len(h.find("background").style.Gradients) != 1 {
		t.Fatal("hover exit failed to restore the gradient")
	}
}

func TestRelationalScopesStayIsolated(t *testing.T) {
	var unrelated *Element
	h := newHost(t, Size{Width: 400, Height: 300}, func(cx *Scope) Node {
		unrelated = NewElement(cx, View{TestID: "unrelated"})
		return View{Children: []Node{View{Class: "[&_svg]:size-4", Children: []Node{Icon{SVG: selectorTestSVG()}}}, unrelated}}
	})
	h.commit()
	unrelated.Set(View{TestID: "unrelated", Checked: func() bool { return true }})
	if len(h.tree.relationRoots) != 0 {
		t.Fatal("an unrelated state update queued a selector subtree")
	}
	h.commit()
}

func TestRelationalGroupAndImportantStatePrecedence(t *testing.T) {
	h := newHost(t, Size{Width: 400, Height: 300}, func(*Scope) Node {
		return View{TestID: "group", Class: "group", Children: []Node{
			View{Class: "group-hover:[&_svg]:size-6 [&_svg]:p-3!", Children: []Node{
				Icon{TestID: "icon", SVG: selectorTestSVG(), Class: "size-4", Style: Style{Hover: &Style{Padding: All(20)}}},
			}},
		}}
	})
	h.commit()
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseEnter, ID: h.find("group").id})
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseEnter, ID: h.find("icon").id})
	h.commit()
	if got := h.find("icon").style; got.Width != Pt(24) || got.Padding.Top != 12 {
		t.Fatalf("group hover or important class lost to inline hover: width=%v padding=%v", got.Width, got.Padding)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventMouseLeave, ID: h.find("group").id})
	h.commit()
	if got := h.find("icon").style.Width; got != Pt(16) {
		t.Fatalf("group hover did not clear: %v", got)
	}
}

func TestRelationalTargetsKeepPeerVariants(t *testing.T) {
	var checked *Signal[bool]
	h := newHost(t, Size{Width: 400, Height: 300}, func(cx *Scope) Node {
		checked = NewSignal(cx, false)
		return View{Class: "[&_svg]:size-4", Children: []Node{
			View{TestID: "peer", Class: "peer", Checked: checked.Get},
			View{TestID: "label", Class: "opacity-100 peer-checked:opacity-50"},
			Icon{SVG: selectorTestSVG()},
		}}
	})
	h.commit()
	if !h.find("label").cls.related {
		t.Fatal("the label is not a selector target")
	}
	if got := h.find("label").style.Opacity; got != Alpha(1) {
		t.Fatalf("unchecked peer opacity=%v", got)
	}
	checked.Set(true)
	h.commit()
	if got := h.find("label").style.Opacity; got != Alpha(.5) {
		t.Fatalf("a selector target lost its peer variant: opacity=%v", got)
	}
	checked.Set(false)
	h.commit()
	if got := h.find("label").style.Opacity; got != Alpha(1) {
		t.Fatalf("peer variant did not clear: opacity=%v", got)
	}
}

func TestRelationalStatesAskOnlyPressableViewsForPresses(t *testing.T) {
	h := newHost(t, Size{Width: 400, Height: 300}, func(*Scope) Node {
		return View{TestID: "source", Class: "[&_svg]:active:opacity-50 has-[:active]:p-2", Children: []Node{
			View{TestID: "plain", Children: []Node{Text{Content: "label"}}},
			View{TestID: "button", OnPress: func(PressEvent) {}},
			Icon{SVG: selectorTestSVG()},
		}}
	})
	h.commit()
	for _, id := range []string{"source", "plain"} {
		if h.find(id).events().Has(driver.EventPressDown) {
			t.Errorf("%s asks for presses for a selector it is not pressed for", id)
		}
	}
	if !h.find("button").events().Has(driver.EventPressDown) {
		t.Error("a pressable view in the selector's scope does not report presses")
	}
	// The pressable view's reported presses reach the selector around it.
	button := h.find("button")
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressDown, ID: button.id})
	h.commit()
	if got := h.find("source").style.Padding.Top; got != 8 {
		t.Fatalf("pressed button: source padding=%v, want 8", got)
	}
	h.tree.Dispatch(driver.Event{Kind: driver.EventPressUp, ID: button.id})
	h.commit()
	if got := h.find("source").style.Padding.Top; got != 0 {
		t.Fatalf("released button: source padding=%v, want 0", got)
	}
}

func TestRelationalSourcesOnlyResolveInTheirScope(t *testing.T) {
	h := newHost(t, Size{Width: 400, Height: 300}, func(*Scope) Node {
		kids := make([]Node, 50)
		for i := range kids {
			id := ""
			if i == 7 {
				id = "icon"
			}
			kids[i] = View{Class: "[&_svg]:size-4", Children: []Node{Icon{TestID: id, SVG: selectorTestSVG()}}}
		}
		return View{Children: kids}
	})
	h.commit()
	icon := h.find("icon")
	sources := h.tree.sourcesFor(icon)
	if len(sources) != 1 || sources[0] != icon.nativeParent() {
		t.Fatalf("sourcesFor=%d sources, want the icon's own button", len(sources))
	}
}

func TestRelationalSelectorsReadVisibleLabelStates(t *testing.T) {
	control := new(Ref)
	var disabled *Signal[bool]
	h := newHost(t, Size{Width: 400, Height: 300}, func(cx *Scope) Node {
		disabled = NewSignal(cx, false)
		return View{Children: []Node{
			View{TestID: "row", Class: "p-8 has-[:disabled]:p-3 [&_[aria-invalid=true]]:opacity-50", Children: []Node{
				Text{TestID: "label", Content: "Email", LabelFor: control},
			}},
			View{TestID: "control", Ref: control, OnPress: func(PressEvent) {}, Disabled: disabled.Get, Invalid: disabled.Get},
		}}
	})
	h.commit()
	if got := h.find("row").style.Padding.Top; got != 32 {
		t.Fatalf("enabled control: row padding=%v, want 32", got)
	}
	disabled.Set(true)
	h.commit()
	// The label shows its control's Disabled and Invalid states, so selectors around the label read them as the label's own.
	if got := h.find("row").style.Padding.Top; got != 12 {
		t.Fatalf("disabled control: row padding=%v, want 12", got)
	}
	if got := h.find("label").style.Opacity; got != Alpha(.5) {
		t.Fatalf("invalid control: label opacity=%v, want .5", got)
	}
	disabled.Set(false)
	h.commit()
	if got, op := h.find("row").style.Padding.Top, h.find("label").style.Opacity; got != 32 || op == Alpha(.5) {
		t.Fatalf("re-enabled control: row padding=%v, label opacity=%v", got, op)
	}
}
