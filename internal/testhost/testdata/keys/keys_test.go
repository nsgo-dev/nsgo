package keys_test

import (
	"sync/atomic"
	"testing"

	_ "github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

type fieldEvents struct {
	bindings, submits, blurs atomic.Int32
	text, selection          atomic.Value
}

func startField(t *testing.T, key string, options *core.KeyOptions) (*nsgotest.App, *fieldEvents) {
	t.Helper()
	events := new(fieldEvents)
	events.text.Store("")
	events.selection.Store(core.TextRange{})
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		value := core.NewSignal(cx, "")
		if options != nil {
			cx.OnKeyWith(key, *options, func() { events.bindings.Add(1) })
		}
		return core.Input{
			TestID: "field", AutoFocus: true, Value: value.Get,
			Style: core.Style{Width: core.Pt(240), Height: core.Pt(40)},
			OnChange: func(text string) {
				events.text.Store(text)
				value.Set(text)
			},
			OnSubmit:          func(string) { events.submits.Add(1) },
			OnBlur:            func() { events.blurs.Add(1) },
			OnSelectionChange: func(r core.TextRange) { events.selection.Store(r) },
		}
	}, nsgotest.Size(300, 120))
	app.Type("id:field", "entry")
	if events.text.Load() != "entry" || !app.Find("id:field").Focused {
		t.Fatal("the field did not receive the setup text and focus")
	}
	return app, events
}

// Editing keys take the same keymap path as ordinary keys, before an editor acts.
func TestEditingKeysHonorAlwaysBindings(t *testing.T) {
	for _, key := range []string{"enter", "return", "backspace", "mod+a"} {
		t.Run(key, func(t *testing.T) {
			app, events := startField(t, key, &core.KeyOptions{When: core.KeyAlways})
			selection := events.selection.Load()
			for n := range 2 {
				app.Key(key)
				if got := events.bindings.Load(); got != int32(n+1) {
					t.Fatalf("%s fired %d times, want %d; each press must release the binding", key, got, n+1)
				}
				if events.text.Load() != "entry" || events.submits.Load() != 0 || events.blurs.Load() != 0 ||
					!app.Find("id:field").Focused || events.selection.Load() != selection {
					t.Fatalf("%s reached the editor despite its binding: text=%q submits=%d blurs=%d selection=%v",
						key, events.text.Load(), events.submits.Load(), events.blurs.Load(), events.selection.Load())
				}
			}
		})
	}
}

func TestEditingKeysKeepTheirUnboundActions(t *testing.T) {
	for _, binding := range []struct {
		name    string
		options *core.KeyOptions
	}{
		{"unbound", nil},
		{"auto", &core.KeyOptions{When: core.KeyAuto}},
		{"not-editing", &core.KeyOptions{When: core.KeyNotEditing}},
	} {
		t.Run(binding.name, func(t *testing.T) {
			for _, key := range []string{"enter", "return", "backspace", "mod+a"} {
				t.Run(key, func(t *testing.T) {
					app, events := startField(t, key, binding.options)
					app.Key(key)
					if events.bindings.Load() != 0 {
						t.Fatal("a binding that yields to text editing fired in the field")
					}
					switch key {
					case "enter", "return":
						if events.submits.Load() != 1 || events.blurs.Load() != 1 || app.Find("id:field").Focused {
							t.Fatalf("Return: submits=%d blurs=%d focused=%v", events.submits.Load(), events.blurs.Load(), app.Find("id:field").Focused)
						}
					case "backspace":
						if events.text.Load() != "entr" {
							t.Fatalf("Backspace left %q, want entr", events.text.Load())
						}
					case "mod+a":
						if got := events.selection.Load(); got != (core.TextRange{Start: 0, End: 5}) {
							t.Fatalf("select all selected %v, want 0-5", got)
						}
					}
					if key == "backspace" || key == "mod+a" {
						if events.submits.Load() != 0 || events.blurs.Load() != 0 || !app.Find("id:field").Focused {
							t.Fatal("editing submitted or blurred the field")
						}
					}
				})
			}
		})
	}
}
