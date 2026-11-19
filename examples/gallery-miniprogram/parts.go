package main

import (
	"strconv"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// The look the demos share: examples/gallery's, which is package ui's, written on base and the theme's colors. A mini program's package has no room for ui, whose buttons link an SVG parser for their spinners, so this example styles base's components the way ui does.

// Metrics, as ui's.
const (
	radiusCard    core.Points = 12
	radiusControl core.Points = 8
	radiusFull    core.Points = 9999
	controlHeight core.Points = 36
)

const (
	regular  = core.FontWeightNormal
	medium   = core.FontWeightMedium
	semibold = core.FontWeightSemibold
	bold     = core.FontWeightBold
)

// colors are the theme's colors where a scope is, by ui.Colors' names: the default theme carries ui's roles, such as "inset", in Palette.Colors.
type colors struct{ cx *core.Scope }

func colorsOf(cx *core.Scope) colors { return colors{cx} }

func (c colors) role(name string) core.Color { return c.cx.Palette().Colors[name] }
func (c colors) background() core.Color      { return c.cx.Palette().Background }
func (c colors) surface() core.Color         { return c.cx.Palette().Surface }
func (c colors) border() core.Color          { return c.cx.Palette().Border }
func (c colors) pressed() core.Color         { return c.cx.Palette().SurfacePressed }
func (c colors) text() core.Color            { return c.cx.Palette().Text }
func (c colors) muted() core.Color           { return c.cx.Palette().TextMuted }
func (c colors) accent() core.Color          { return c.cx.Palette().Accent }
func (c colors) onAccent() core.Color        { return c.cx.Palette().AccentText }
func (c colors) danger() core.Color          { return c.cx.Palette().Danger }
func (c colors) success() core.Color         { return c.cx.Palette().Success }
func (c colors) warning() core.Color         { return c.cx.Palette().Warning }
func (c colors) inset() core.Color           { return c.role("inset") }
func (c colors) raised() core.Color          { return c.role("raised") }
func (c colors) field() core.Color           { return c.role("field") }
func (c colors) fieldBorder() core.Color     { return c.role("fieldBorder") }
func (c colors) line() core.Color            { return c.role("line") }
func (c colors) faint() core.Color           { return c.role("faint") }
func (c colors) accentFill() core.Color      { return c.role("accentFill") }
func (c colors) track() core.Color           { return c.role("track") }
func (c colors) knob() core.Color            { return c.role("knob") }

// clear is no color that is set, unlike 0: a rim kept for its width, a fill a state replaces.
var clear = core.RGBA(0xffffff, 0)

// tint is c faded for a fill behind text.
func tint(c core.Color) core.Color { return c.Faded(0.14) }

// text is a line of content in size and weight, colored by color.
func text(content string, size core.Points, weight core.FontWeight, color func() core.Color) core.Text {
	return core.Text{Content: content, Style: core.Style{FontSize: size, FontWeight: weight}, StyleFunc: func() core.Style { return core.Style{Color: color()} }}
}

// textf is a line whose content follows fn.
func textf(fn func() string, size core.Points, weight core.FontWeight, color func() core.Color) core.Text {
	t := text("", size, weight, color)
	t.ContentFunc = fn
	return t
}

// card is a demo's titled card: a bold title and a quieter note on the page, then a surface of the children.
func card(cx *core.Scope, title, note string, children ...core.Node) core.Node {
	c := colorsOf(cx)
	heading := text(title, 13, bold, c.text)
	heading.Role = core.RoleHeader
	head := []core.Node{heading}
	if note != "" {
		n := text(note, 12, regular, c.muted)
		n.MaxLines = 2
		head = append(head, n)
	}
	return core.View{TestID: "card-" + title, Style: core.Style{Gap: 8}, Children: []core.Node{
		core.View{Style: core.Style{Gap: 2, Padding: core.Symmetric(0, 2)}, Children: head},
		core.View{Style: core.Style{Gap: 12, Padding: core.All(14), CornerRadius: radiusCard, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1},
			StyleFunc: func() core.Style { return core.Style{Background: c.surface(), BorderColor: c.border()} }, Children: children},
	}}
}

// cards is a demo's page: its cards in one column, scrolling.
func cards(nodes []core.Node) core.Node {
	return core.Scroll{TestID: "demo-scroll", Style: fillPage, Children: []core.Node{
		core.View{Style: core.Style{Padding: core.Edges{Top: 12, Right: 12, Bottom: 32, Left: 12}, Gap: 20}, Children: nodes},
	}}
}

// row lays its children out in a row that wraps.
func row(children ...core.Node) core.Node {
	return core.View{Style: core.Style{Direction: core.Row, Wrap: core.Wrap, AlignItems: core.AlignCenter, Gap: 8}, Children: children}
}

// value is a quiet line that follows fn, for what a control changed.
func value(cx *core.Scope, id string, fn func() string) core.Node {
	t := textf(fn, 13, regular, colorsOf(cx).muted)
	t.TestID = id
	return t
}

// variant is a button's look, as ui.ButtonVariant's.
type variant uint8

const (
	primary variant = iota
	secondary
	outline
	ghost
)

// button is base.Button in ui's look: Text on a fill of v's, Disabled faded.
type button struct {
	Text     string
	Variant  variant
	Small    bool
	Disabled func() bool
	OnPress  func()
	TestID   string
}

// Render implements core.Node.
func (b button) Render(cx *core.Scope) core.Node {
	c := colorsOf(cx)
	height, inset, size := controlHeight, core.Points(14), core.Points(14)
	if b.Small {
		height, inset, size = 28, 10, 13
	}
	fill := func() (rest, held, rim, ink core.Color) {
		switch b.Variant {
		case secondary:
			return c.text().Faded(0.08), c.text().Faded(0.14), clear, c.text()
		case outline:
			return clear, c.pressed(), c.fieldBorder(), c.text()
		case ghost:
			return clear, c.pressed(), clear, c.text()
		}
		return c.accentFill(), c.accentFill().Faded(0.85), clear, c.onAccent()
	}
	label := textf(func() string { return b.Text }, size, medium, func() core.Color { _, _, _, ink := fill(); return ink })
	return base.Button{Label: b.Text, TestID: b.TestID, Disabled: b.Disabled, OnPress: func(core.PressEvent) {
		if b.OnPress != nil {
			b.OnPress()
		}
	},
		Style: core.Style{Height: core.Pt(height), Padding: core.Symmetric(0, inset), CornerRadius: radiusControl, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1,
			Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter, Disabled: &core.Style{Opacity: core.Alpha(0.4)}},
		StyleFunc: func() core.Style {
			rest, held, rim, _ := fill()
			return core.Style{Background: rest, BorderColor: rim, Active: &core.Style{Background: held}}
		},
		Children: []core.Node{label}}
}

// badge is a capsule of text tinted by color, or bordered when color is nil.
func badge(cx *core.Scope, label string, color func() core.Color) core.Node {
	c := colorsOf(cx)
	ink := c.text
	if color != nil {
		ink = color
	}
	return core.View{Style: core.Style{Padding: core.Symmetric(2, 8), CornerRadius: radiusFull, BorderWidth: 1},
		StyleFunc: func() core.Style {
			if color == nil {
				return core.Style{Background: c.surface(), BorderColor: c.border()}
			}
			return core.Style{Background: tint(color()), BorderColor: clear}
		},
		Children: []core.Node{text(label, 12, medium, ink)}}
}

// times says how many times something was done: none when n is 0, else verb once or n times.
func times(verb, none string, n int) string {
	switch n {
	case 0:
		return none
	case 1:
		return verb + " once"
	}
	return verb + " " + itoa(n) + " times"
}

// itoa formats an integer.
func itoa(n int) string { return strconv.Itoa(n) }

// fillPage grows to fill its container.
var fillPage = core.Style{FlexGrow: 1, FlexShrink: 1, MinHeight: core.Pt(0), MinWidth: core.Pt(0)}
