package ui

import (
	"testing"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// TestToggleGroupAndToolbar put toggle buttons in one stop of the focus.
func TestToggleGroupAndToolbar(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var bold *core.Signal[bool]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			bold = core.NewSignal(cx, false)
			return Toolbar{TestID: "bar", Label: "Format", Children: []core.Node{
				ToggleGroup{TestID: "marks", Children: []core.Node{
					ToggleButton{Text: "Bold", TestID: "bold", On: bold.Get, OnChange: bold.Set},
					ToggleButton{Text: "Italic", TestID: "italic"},
				}},
				ToolbarSeparator{TestID: "line"},
				ToolbarGroup{Label: "More", TestID: "more", Children: []core.Node{Link{Text: "Help", Href: "/help", OpenWith: base.LinkOpenApp, TestID: "help"}}},
			}}
		})
		if app.Find("bar").Props.Role != core.RoleToolbar || app.Find("marks").Props.Role != core.RoleGroup {
			t.Fatal("a toggle group in a toolbar is one of its groups")
		}
		if !app.Find("bold").Props.Focusable || app.Find("italic").Props.Focusable || app.Find("help").Props.Focusable {
			t.Fatal("the toolbar is not one stop of the focus")
		}
		app.Press("bold")
		if !bold.Get() || !app.Find("bold").Props.Pressed {
			t.Fatal("a press does not turn the toggle on")
		}
	})
}

// TestLink reports where it goes, then the press.
func TestLink(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var log []string
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return Link{Text: "Docs", Href: "/docs", OpenWith: base.LinkOpenApp, TestID: "docs",
				OnOpen: func(href string) { log = append(log, href) }, OnPress: func() { log = append(log, "press") }}
		})
		app.Press("docs")
		if len(log) != 2 || log[0] != "/docs" || log[1] != "press" || app.Find("docs").Props.Role != core.RoleLink {
			t.Fatalf("log = %v", log)
		}
	})
}

// TestLinkTextColor draws a link's text and hover underline in the color its Style gives, the accent when it gives none.
func TestLinkTextColor(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		ink, hover := core.RGB(0xf2dcb5), core.RGB(0xffeedd)
		app, p := start(t, e, func(cx *core.Scope) core.Node {
			return core.View{Children: []core.Node{
				Link{Text: "Default", TestID: "default"},
				Link{Text: "Plain", TestID: "plain", Style: core.Style{Color: ink}},
				Link{Text: "Custom", TestID: "custom", Style: core.Style{Color: ink, Hover: &core.Style{Color: hover}}},
				Link{Text: "Transparent", TestID: "transparent", Style: core.Style{Color: core.Transparent}},
				Link{Text: "Hovered", TestID: "hovered", Style: core.Style{Hover: &core.Style{Color: hover}}},
			}}
		})
		if got := app.Find("default >> Default").TextStyle.Color; got != p.Accent {
			t.Fatalf("default link color = %#x, want %#x", got, p.Accent)
		}
		if got := app.Find("transparent >> Transparent").TextStyle.Color; got != core.Transparent {
			t.Fatalf("transparent link color = %#x", got)
		}
		text := app.Find("custom >> Custom")
		if text.TextStyle.Color != ink {
			t.Fatalf("custom link color = %#x, want %#x", text.TextStyle.Color, ink)
		}
		id := text.ID
		app.Hover("plain")
		text = app.Find("plain >> Plain")
		// A zero decoration color uses the glyph color on every backend.
		if text.TextStyle.Color != ink || text.TextStyle.Decoration != core.TextDecorationUnderline || text.TextStyle.DecorationColor != 0 {
			t.Fatalf("custom link underline = %+v", text.TextStyle)
		}
		app.Hover("custom")
		text = app.Find("custom >> Custom")
		if text.TextStyle.Color != hover || text.TextStyle.Decoration != core.TextDecorationUnderline || text.TextStyle.DecorationColor != 0 {
			t.Fatalf("hovered link style = %+v", text.TextStyle)
		}
		app.Hover("default")
		text = app.Find("custom >> Custom")
		if text.TextStyle.Color != ink || text.TextStyle.Decoration != 0 || text.ID != id {
			t.Fatalf("link did not restore its custom color in place: %+v", text.TextStyle)
		}
		text = app.Find("default >> Default")
		if text.TextStyle.Color != p.Accent || text.TextStyle.Decoration != core.TextDecorationUnderline || text.TextStyle.DecorationColor != 0 {
			t.Fatalf("default link underline = %+v", text.TextStyle)
		}
		e.Dark = !e.Dark
		app.SetEnvironment(e)
		want := core.DefaultTheme().Light.Accent
		if e.Dark {
			want = core.DefaultTheme().Dark.Accent
		}
		if text = app.Find("default >> Default"); text.TextStyle.Color != want || text.TextStyle.Decoration != core.TextDecorationUnderline || text.TextStyle.DecorationColor != 0 {
			t.Fatalf("default link did not follow the new appearance: %+v", text.TextStyle)
		}
		if text = app.Find("custom >> Custom"); text.TextStyle.Color != ink || text.ID != id {
			t.Fatal("an appearance change replaced the custom color or rebuilt the link")
		}
		// A hover color alone keeps the accent until the pointer enters.
		if got := app.Find("hovered >> Hovered").TextStyle.Color; got != want {
			t.Fatalf("hover-only link color = %#x, want the accent %#x", got, want)
		}
		app.Hover("hovered")
		if text = app.Find("hovered >> Hovered"); text.TextStyle.Color != hover || text.TextStyle.Decoration != core.TextDecorationUnderline {
			t.Fatalf("hover-only link under the pointer = %+v", text.TextStyle)
		}
	})
}

// TestPagination moves between pages, the current one selected.
func TestPagination(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var page *core.Signal[int]
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			page = core.NewSignal(cx, 1)
			return Pagination{Page: page.Get, TotalPages: func() int { return 20 }, OnChange: page.Set, Label: "Pages",
				PreviousLabel: "Previous", NextLabel: "Next", TestID: "pages"}
		})
		if !app.Find("label:Previous").Props.Disabled {
			t.Fatal("Previous is not disabled on the first page")
		}
		app.Press("label:Next")
		if page.Get() != 2 || app.Find("label:Previous").Props.Disabled {
			t.Fatalf("page = %d after Next", page.Get())
		}
		app.Press(`text:"20"`)
		if page.Get() != 20 || !app.Find("label:Next").Props.Disabled {
			t.Fatalf("page = %d after the last page", page.Get())
		}
	})
}

// TestOtpInput keeps digits up to its length and reports the full code.
func TestOtpInput(t *testing.T) {
	each(t, func(t *testing.T, e core.Environment) {
		var code, done string
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			return OtpInput{Length: 4, Groups: 2, Label: "Code", OnChange: func(v string) { code = v }, OnComplete: func(v string) { done = v }}
		})
		app.Type("label:Code", "12a345")
		if code != "1234" || done != "1234" {
			t.Fatalf("code = %q, done = %q", code, done)
		}
	})
}

// TestScrollbarTheme replaces the indicators, and the view's own bar goes
// with the theme.
func TestScrollbarTheme(t *testing.T) {
	headless(t)
	each(t, func(t *testing.T, e core.Environment) {
		app, _ := start(t, e, func(cx *core.Scope) core.Node {
			base.SetScrollbarTheme(cx, ScrollbarTheme(cx))
			return core.Scroll{TestID: "list", Style: core.Style{Height: core.Pt(100), Width: core.Pt(100)},
				Children: []core.Node{core.View{Style: core.Style{Height: core.Pt(800)}}}}
		})
		if s := app.Find("list").Scroll; !s.HideIndicator {
			t.Fatal("the theme does not replace the platform's indicator")
		}
	})
}
