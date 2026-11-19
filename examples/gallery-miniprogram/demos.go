package main

import (
	"strconv"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/core"
)

// The demos, a page of cards each, in examples/gallery's order and look.

func buttonDemo(cx *core.Scope, _ *gallery) []core.Node {
	count := core.NewSignal(cx, 0)
	return []core.Node{
		card(cx, "Variants", "The main action, a quiet one, a rim and none",
			row(button{Text: "Primary"}, button{Text: "Secondary", Variant: secondary}, button{Text: "Outline", Variant: outline}, button{Text: "Ghost", Variant: ghost})),
		card(cx, "Sizes", "", row(button{Text: "Small", Small: true}, button{Text: "Default"})),
		card(cx, "Counter", "",
			row(button{Text: "Press", TestID: "press", OnPress: func() { count.Update(func(n int) int { return n + 1 }) }},
				button{Text: "Reset", Variant: outline, TestID: "reset", Disabled: func() bool { return count.Get() == 0 }, OnPress: func() { count.Set(0) }}),
			value(cx, "count", func() string { return times("Pressed", "Not pressed yet", count.Get()) })),
	}
}

// field is a text field in ui's look: the platform's own field on a well.
func field(cx *core.Scope, placeholder, id string, s *core.Signal[string], masked, multiline bool) core.Node {
	c := colorsOf(cx)
	height := core.Pt(controlHeight)
	if multiline {
		height = core.Pt(88)
	}
	return core.Input{Value: s.Get, OnChange: s.Set, Placeholder: placeholder, Label: placeholder, TestID: id, Masked: masked, Multiline: multiline,
		Style: core.Style{Height: height, Padding: core.Symmetric(8, 10), CornerRadius: radiusControl, BorderWidth: 1, FontSize: 15},
		StyleFunc: func() core.Style {
			return core.Style{Background: c.field(), BorderColor: c.fieldBorder(), Color: c.text(), Focus: &core.Style{BorderColor: c.accent()}}
		}}
}

func inputDemo(cx *core.Scope, _ *gallery) []core.Node {
	name, secret, notes := core.NewSignal(cx, ""), core.NewSignal(cx, ""), core.NewSignal(cx, "")
	return []core.Node{
		card(cx, "Text field", "The platform's own field, which its keyboard types into",
			field(cx, "Your name", "name", name, false, false),
			value(cx, "greeting", func() string {
				if name.Get() == "" {
					return "Type a name"
				}
				return "Hello, " + name.Get() + "!"
			})),
		card(cx, "Password", "Masked as it is typed",
			field(cx, "Password", "secret", secret, true, false),
			value(cx, "secret-length", func() string { return itoa(len([]rune(secret.Get()))) + " characters" })),
		card(cx, "Multiline", "", field(cx, "Notes", "notes", notes, false, true)),
	}
}

func togglesDemo(cx *core.Scope, _ *gallery) []core.Node {
	c := colorsOf(cx)
	on := core.NewSignal(cx, true)
	fruits := []string{"Apple", "Banana", "Cherry"}
	picked := make([]*core.Signal[bool], len(fruits))
	size := core.NewSignal(cx, 1)
	choice := core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8, MinHeight: core.Pt(28)}
	var boxes, radios []core.Node
	for i, f := range fruits {
		picked[i] = core.NewSignal(cx, i == 0)
		boxes = append(boxes, base.Checkbox{Label: f, TestID: "check-" + f, Checked: picked[i].Get, OnChange: picked[i].Set, Style: choice,
			IndicatorStyle: core.Style{Width: core.Pt(18), Height: core.Pt(18), CornerRadius: 4, BorderWidth: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			IndicatorStyleFunc: func() core.Style {
				return core.Style{Background: c.field(), BorderColor: c.fieldBorder(), Checked: &core.Style{Background: c.accentFill(), BorderColor: c.accentFill()}}
			},
			Indicator: core.Show{When: picked[i].Get, Then: func(*core.Scope) core.Node { return text("✓", 12, bold, c.onAccent) }},
			Children:  []core.Node{text(f, 15, regular, c.text)}})
	}
	for i, s := range []string{"Small", "Medium", "Large"} {
		checked := func() bool { return size.Get() == i }
		radios = append(radios, base.Radio{Label: s, TestID: "radio-" + s, Checked: checked, OnChange: func(bool) { size.Set(i) }, Style: choice,
			Children: []core.Node{
				core.View{Style: core.Style{Width: core.Pt(18), Height: core.Pt(18), CornerRadius: 9, BorderWidth: 1, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
					StyleFunc: func() core.Style {
						if checked() {
							return core.Style{Background: c.accentFill(), BorderColor: c.accentFill()}
						}
						return core.Style{Background: c.field(), BorderColor: c.fieldBorder()}
					},
					Children: []core.Node{core.View{Style: core.Style{Width: core.Pt(6), Height: core.Pt(6), CornerRadius: 3},
						StyleFunc: func() core.Style { return core.Style{Background: c.onAccent()} }}}},
				text(s, 15, regular, c.text),
			}})
	}
	return []core.Node{
		card(cx, "Switch", "",
			row(base.Switch{Label: "Wi-Fi", Checked: on.Get, OnChange: on.Set, TestID: "switch",
				TrackStyle: core.Style{Width: core.Pt(44), Height: core.Pt(26), Padding: core.All(2), CornerRadius: 13},
				TrackStyleFunc: func() core.Style {
					return core.Style{Background: c.track(), Checked: &core.Style{Background: c.accentFill(), AlignItems: core.AlignEnd}}
				},
				ThumbStyle: core.Style{Width: core.Pt(22), Height: core.Pt(22), CornerRadius: 11}, ThumbStyleFunc: func() core.Style { return core.Style{Background: c.knob()} }},
				value(cx, "switch-state", func() string {
					if on.Get() {
						return "Wi-Fi on"
					}
					return "Wi-Fi off"
				}))),
		card(cx, "Checkbox", "",
			append(boxes, value(cx, "checked", func() string {
				n := 0
				for _, p := range picked {
					if p.Get() {
						n++
					}
				}
				return itoa(n) + " of 3 picked"
			}))...),
		card(cx, "Radio", "One of a group", base.RadioGroup{Label: "Size", Style: core.Style{Gap: 4}, Children: radios}),
	}
}

// progress is base.Progress in ui's look, v its percent done.
func progress(cx *core.Scope, label, id string, v func() float32) core.Node {
	c := colorsOf(cx)
	return base.Progress{Label: label, TestID: id, Value: v,
		TrackStyle: core.Style{Height: core.Pt(6), CornerRadius: 3, Overflow: core.Hidden}, TrackStyleFunc: func() core.Style { return core.Style{Background: c.track()} },
		IndicatorStyle: core.Style{Height: core.Pct(100), CornerRadius: 3}, IndicatorStyleFunc: func() core.Style { return core.Style{Background: c.accentFill()} }}
}

func sliderDemo(cx *core.Scope, _ *gallery) []core.Node {
	c := colorsOf(cx)
	level := core.NewSignal(cx, 40.0)
	return []core.Node{
		card(cx, "Slider", "Drag the thumb, or tap along the track",
			base.Slider{Label: "Level", TestID: "slider", Step: func() float64 { return 1 },
				Value:    func() base.SliderValue { return base.SingleValue(level.Get()) },
				OnChange: func(v base.SliderValue) { level.Set(v.Start) },
				Style:    core.Style{Height: core.Pt(28)}, TrackStyle: core.Style{Height: core.Pt(28)},
				IndicatorStyle: core.Style{Height: core.Pt(4), CornerRadius: 2, Margin: core.Edges{Top: 12}}, IndicatorStyleFunc: func() core.Style { return core.Style{Background: c.track()} },
				RangeStyle: core.Style{CornerRadius: 2}, RangeStyleFunc: func() core.Style { return core.Style{Background: c.accentFill()} },
				ThumbStyle:     core.Style{Width: core.Pt(22), Height: core.Pt(22), CornerRadius: 11, BorderWidth: 1, Margin: core.Edges{Top: 3}},
				ThumbStyleFunc: func() core.Style { return core.Style{Background: c.knob(), BorderColor: c.border()} }},
			value(cx, "level", func() string { return itoa(int(level.Get())) + "%" })),
		card(cx, "Progress", "The slider's value as progress", progress(cx, "Level", "level-progress", func() float32 { return float32(level.Get()) })),
	}
}

func badgeDemo(cx *core.Scope, _ *gallery) []core.Node {
	c := colorsOf(cx)
	var avatars []core.Node
	for _, n := range []string{"Ada Lovelace", "Grace Hopper", "Alan Turing", "NSGo"} {
		a := base.Avatar{Name: n, Style: core.Style{Width: core.Pt(40), Height: core.Pt(40), CornerRadius: 20, Overflow: core.Hidden},
			StyleFunc:     func() core.Style { return core.Style{Background: c.raised(), BorderWidth: 1, BorderColor: c.border()} },
			ImageStyle:    core.Style{Width: core.Pct(100), Height: core.Pct(100)},
			FallbackStyle: core.Style{Width: core.Pct(100), Height: core.Pct(100), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			Fallback:      text(n[:1], 15, semibold, c.muted)}
		if n == "NSGo" {
			a.Src = logo
		}
		avatars = append(avatars, a)
	}
	return []core.Node{
		card(cx, "Badge", "", row(badge(cx, "Neutral", nil), badge(cx, "Accent", c.accent), badge(cx, "Done", c.success), badge(cx, "Warn", c.warning), badge(cx, "Danger", c.danger))),
		card(cx, "Avatar", "Initials, or a picture", row(avatars...)),
		card(cx, "Progress", "", progress(cx, "Upload", "upload", func() float32 { return 65 })),
	}
}

// logo is the gallery's logo, in the package's resources.
var logo = core.ImageSource{URI: "asset:logo.png", Size: core.Size{Width: 256, Height: 256}}

// photo is a fixed photo from the web.
const photo = "https://picsum.photos/id/1015/600/300"

func imageDemo(cx *core.Scope, _ *gallery) []core.Node {
	c := colorsOf(cx)
	frame := func(s core.Style) func() core.Style {
		return func() core.Style {
			return core.Style{CornerRadius: radiusControl, BorderCurve: core.BorderCurveContinuous, BorderWidth: 1, BorderColor: c.border(), Background: c.raised()}.Merge(s)
		}
	}
	var modes []core.Node
	for _, m := range []struct {
		name string
		mode core.ResizeMode
	}{{"Cover", core.Cover}, {"Contain", core.Contain}} {
		modes = append(modes, core.View{Style: core.Style{Gap: 4, AlignItems: core.AlignCenter}, Children: []core.Node{
			core.Image{Source: logo, ResizeMode: m.mode, TestID: "logo-" + m.name, StyleFunc: frame(core.Style{Width: core.Pt(120), Height: core.Pt(72)})},
			text(m.name, 12, regular, c.muted),
		}})
	}
	return []core.Node{
		card(cx, "From the package", "A file in the app's resources", row(modes...)),
		card(cx, "From the web", "Loaded by its address",
			core.Image{Source: core.ImageSource{URI: photo}, ResizeMode: core.Cover, TestID: "photo", StyleFunc: frame(core.Style{Width: core.Pct(100), AspectRatio: 2})}),
	}
}

func listDemo(cx *core.Scope, _ *gallery) []core.Node {
	c := colorsOf(cx)
	selection := base.NewSelection(cx, false)
	picked := core.NewSignal(cx, -1)
	label := func(i int) string { return "Row " + itoa(i+1) }
	return []core.Node{
		card(cx, "List", "200 rows, made as they scroll in; a tap selects one",
			base.List{Label: "Rows", Selection: selection, TapSelects: true, TestID: "list", EstimatedSize: 40,
				Count: func() int { return 200 }, Key: func(i int) any { return i }, RowLabel: label,
				OnSelectionChange: func(s *base.Selection) {
					if k, ok := s.Cursor().(int); ok {
						picked.Set(k)
					}
				},
				Row: func(cx *core.Scope, r core.ListRow) core.Node {
					return textf(func() string { return label(r.Index()) }, 15, regular, c.text)
				},
				Style:     core.Style{Height: core.Pt(320)},
				ItemStyle: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Height: core.Pt(40), Padding: core.Symmetric(0, 10), CornerRadius: 6},
				ItemStyleFunc: func() core.Style {
					return core.Style{Selected: &core.Style{Background: tint(c.accent())}}
				}},
			value(cx, "picked", func() string {
				if picked.Get() < 0 {
					return "Nothing selected"
				}
				return label(picked.Get()) + " selected"
			})),
	}
}

func layoutDemo(cx *core.Scope, _ *gallery) []core.Node {
	c := colorsOf(cx)
	box := func(label string, s core.Style) core.Node {
		return core.View{StyleFunc: func() core.Style {
			return core.Style{Background: tint(c.accent()), BorderWidth: 1, BorderColor: c.accent().Faded(0.4), CornerRadius: 8,
				Padding: core.Symmetric(8, 12), AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter}.Merge(s)
		}, Children: []core.Node{text(label, 13, medium, c.text)}}
	}
	frame := func(s core.Style, children ...core.Node) core.Node {
		return core.View{StyleFunc: func() core.Style {
			return core.Style{Background: c.inset(), CornerRadius: 10, Padding: core.All(8), Gap: 8}.Merge(s)
		}, Children: children}
	}
	var justify []core.Node
	for _, j := range []struct {
		name string
		v    core.Justify
	}{{"Start", core.JustifyStart}, {"Center", core.JustifyCenter}, {"SpaceBetween", core.JustifySpaceBetween}} {
		justify = append(justify, text(j.name, 12, regular, c.muted),
			frame(core.Style{Direction: core.Row, JustifyContent: j.v}, box("A", core.Style{}), box("B", core.Style{}), box("C", core.Style{})))
	}
	var tags []core.Node
	for _, t := range []string{"Go", "WebAssembly", "WeChat", "QQ", "Alipay", "Douyin", "Flexbox", "Taffy"} {
		tags = append(tags, box(t, core.Style{}))
	}
	return []core.Node{
		card(cx, "Row", "Direction and JustifyContent", justify...),
		card(cx, "Grow", "FlexGrow shares the space left",
			frame(core.Style{Direction: core.Row}, box("1", core.Style{FlexGrow: 1}), box("2", core.Style{FlexGrow: 2}), box("Fixed", core.Style{}))),
		card(cx, "Wrap", "What does not fit moves to the next line",
			frame(core.Style{Direction: core.Row, Wrap: core.Wrap}, tags...)),
	}
}

func tabsDemo(cx *core.Scope, _ *gallery) []core.Node {
	c := colorsOf(cx)
	selected := core.NewSignal(cx, 0)
	names := []string{"Today", "Week", "Month"}
	var tabs []core.Node
	for i, n := range names {
		on := func() bool { return selected.Get() == i }
		tabs = append(tabs, base.Tab{Label: n, TestID: "tab-" + n, Selected: on, OnPress: func(core.PressEvent) { selected.Set(i) },
			Style:     core.Style{FlexGrow: 1, Height: core.Pt(30), CornerRadius: 6, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
			StyleFunc: func() core.Style { return core.Style{Selected: &core.Style{Background: c.raised()}} },
			Children: []core.Node{text(n, 13, medium, func() core.Color {
				if on() {
					return c.text()
				}
				return c.muted()
			})}})
	}
	shown := textf(func() string { return names[selected.Get()] + "'s view" }, 15, semibold, c.text)
	shown.TestID = "tab-content"
	return []core.Node{
		card(cx, "Segmented", "",
			base.Tabs{Label: "Period", TestID: "tabs", Style: core.Style{Direction: core.Row, Padding: core.All(3), Gap: 2, CornerRadius: radiusControl},
				StyleFunc: func() core.Style { return core.Style{Background: c.inset()} }, Children: tabs},
			core.View{Style: core.Style{Height: core.Pt(80), CornerRadius: radiusControl, AlignItems: core.AlignCenter, JustifyContent: core.JustifyCenter},
				StyleFunc: func() core.Style { return core.Style{Background: c.inset()} }, Children: []core.Node{shown}}),
	}
}

func navigationDemo(cx *core.Scope, g *gallery) []core.Node {
	return []core.Node{
		card(cx, "Push", "Each page goes over the last; back pops it",
			row(button{Text: "Push a page", TestID: "push", OnPress: func() { g.nav.Push(route{depth: 1}) }})),
	}
}

// pushedPage is the page the NavStack demo pushed depth deep.
func pushedPage(cx *core.Scope, g *gallery, depth int) []core.Node {
	return []core.Node{
		card(cx, "Page "+itoa(depth), "Back pops it, and Back to the index pops them all",
			row(button{Text: "Push another", TestID: "push", OnPress: func() { g.nav.Push(route{depth: depth + 1}) }},
				button{Text: "Back to the index", Variant: outline, TestID: "root", OnPress: g.nav.PopToRoot})),
	}
}

func servicesDemo(cx *core.Scope, _ *gallery) []core.Node {
	answer := core.NewSignal(cx, "")
	copied := core.NewSignal(cx, false)
	store := cx.Storage()
	visits := 0
	if v, ok := store.GetItem("visits"); ok {
		visits, _ = strconv.Atoi(v)
	}
	visits++
	store.SetItem("visits", itoa(visits))
	ask := func(sheet bool) func() {
		return func() {
			cx.ShowAlert(core.AlertConfig{Title: "Delete the draft?", Message: "It cannot be restored.", Sheet: sheet, Cancelable: true,
				Actions: []core.AlertAction{{Label: "Cancel", Role: core.AlertCancel}, {Label: "Delete", Role: core.AlertDestructive}}},
				func(i int) {
					if i == 1 {
						answer.Set("Deleted")
					} else {
						answer.Set("Kept")
					}
				})
		}
	}
	return []core.Node{
		card(cx, "Alert", "The platform's modal and action sheet",
			row(button{Text: "Alert", TestID: "alert", OnPress: ask(false)}, button{Text: "Action sheet", Variant: secondary, TestID: "sheet", OnPress: ask(true)}),
			value(cx, "answer", func() string {
				if answer.Get() == "" {
					return "Not answered yet"
				}
				return answer.Get()
			})),
		card(cx, "Clipboard", "Text the user pastes elsewhere",
			row(button{Text: "Copy a greeting", Variant: outline, TestID: "copy", OnPress: func() {
				cx.SetClipboard("Hello from NSGo")
				copied.Set(true)
			}}),
			value(cx, "copied", func() string {
				if copied.Get() {
					return "Copied"
				}
				return "Nothing copied yet"
			})),
		card(cx, "Storage", "Kept between launches",
			value(cx, "visits", func() string { return times("Opened", "", visits) })),
	}
}
