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
