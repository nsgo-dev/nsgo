package ui

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/tsx/compiler"
)

func TestReadIcon(t *testing.T) {
	Icons(map[string]func() *core.SVG{"plus": lucide.Plus})
	if readIcon("plus") != lucide.Plus() {
		t.Error("a name reads as the icon Icons gave")
	}
	if readIcon(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 4 4"><rect width="4" height="4"/></svg>`) == nil {
		t.Error("an SVG document reads as an icon")
	}
	if readIcon("unknown") != nil || readIcon(nil) != nil {
		t.Error("what names no icon reads as none")
	}
}

// TestTouchControlSizingFromTSX sizes a Button through its style, filling the column or at a percentage width and a height of the app's, with the face filling it, and an OtpInput's cells through size, on phones of both platforms.
func TestTouchControlSizingFromTSX(t *testing.T) {
	files := fstest.MapFS{"App.tsx": {Data: []byte(`import { mount, View } from "nsgo";
import { Button, OtpInput } from "nsgo/ui";
mount(() => <View style={{width: 320, alignItems: "flex-start"}}>
    <Button testID="action" size="large" style={{width: "100%"}}>Continue</Button>
    <Button testID="custom" style={{width: "50%", height: 50}}>Custom</Button>
    <OtpInput testID="code" length={4} size={50} label="Code" />
</View>);`)}}
	for _, os := range []core.OS{core.OSIOS, core.OSAndroid} {
		t.Run(string(os), func(t *testing.T) {
			app := nsgotest.Start(t, compiler.App(files, "App.tsx"), nsgotest.Size(390, 844),
				nsgotest.Environment(core.Environment{OS: os, Idiom: core.IdiomPhone}))
			for _, c := range []struct {
				id            string
				width, height core.Points
			}{{"action", 320, 48}, {"custom", 160, 50}} {
				button := app.Find(c.id)
				if got := button.Frame.Size; got.Width != c.width || got.Height != c.height || button.Children[0].Frame.Size != got {
					t.Fatalf("%s button and face = %v, %v; want %v x %v", c.id, got, button.Children[0].Frame.Size, c.width, c.height)
				}
			}
			if got := app.Find("code").Children[0].Children[0].Frame.Size; got.Width != 50 || got.Height != 50 {
				t.Fatalf("OTP size prop was lost: %v", got)
			}
		})
	}
}

func TestAlertDialogActionRolesFromTSX(t *testing.T) {
	for _, c := range []struct{ key, want string }{{"Escape", "cancel"}, {"Return", "delete"}} {
		t.Run(c.key, func(t *testing.T) {
			files := fstest.MapFS{"App.tsx": {Data: []byte(`import { mount, signal, Text, View } from "nsgo";
import { AlertDialog } from "nsgo/ui";
function App() {
    const open = signal(true), result = signal("");
    return <View>
        <AlertDialog open={open()} title="Delete?" onOpenChange={(e) => open.set(e.open)} actions={[
            { text: "Cancel", role: "cancel", onPress: () => result.set("cancel") },
            { text: "Delete", role: "destructive", default: true, testID: "default", onPress: () => result.set("delete") },
        ]} />
        <Text>{() => "result " + result()}</Text>
    </View>;
}
mount(App);`)}}
			// On a desktop, as nsgotest presses the focused default there on Return.
			app := nsgotest.Start(t, compiler.App(files, "App.tsx"), nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
			app.Find("role:alertdialog")
			if !app.Find("id:default").Focused {
				t.Fatal("TSX default action was not focused")
			}
			app.Key(c.key)
			app.Advance(time.Second)
			app.WaitFor("text:result "+c.want, time.Second)
		})
	}
}

func TestSliderBoundsFromTSXStayReactive(t *testing.T) {
	files := fstest.MapFS{"App.tsx": {Data: []byte(`import { mount, signal, View } from "nsgo";
import { Button, Slider } from "nsgo/ui";
function App() {
    const minimum = signal(0), maximum = signal(0), step = signal(0), value = signal(60);
    return <View style={{width: 240}}>
        <Button testID="load" onPress={() => setTimeout(() => {
            minimum.set(10); maximum.set(300); step.set(5);
        }, 1000)}>Load duration</Button>
        <Slider testID="position" label="Position" min={minimum()} max={maximum()} step={step()}
            value={value()} onChange={(event) => value.set(event.value)} />
    </View>;
}
mount(App);`)}}
	app := nsgotest.Start(t, compiler.App(files, "App.tsx"), nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	thumb := app.Find("position-thumb").ID
	if v := app.Find("position-thumb").Props.Value; v.Min != 0 || v.Max != 0 || v.Now != 0 {
		t.Fatalf("unknown duration: %+v", v)
	}
	app.Press("load")
	app.Focus("position-thumb")
	app.Advance(time.Second)
	if v := app.Find("position-thumb"); v.ID != thumb || !v.Focused || v.Props.Value.Min != 10 || v.Props.Value.Max != 300 || v.Props.Value.Now != 60 {
		t.Fatalf("TSX bounds lost the thumb, focus or value: %+v, focused %v", v.Props.Value, v.Focused)
	}
	app.Key("right")
	if v := app.Find("position-thumb").Props.Value; v.Now != 65 {
		t.Fatalf("TSX step did not change: %+v", v)
	}
}
