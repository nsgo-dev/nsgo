package ui

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
	"github.com/nsgo-dev/nsgo/tsx"
	"github.com/nsgo-dev/nsgo/tsx/compiler"
)

func TestCheckboxChangeEventFromTSX(t *testing.T) {
	files := fstest.MapFS{"App.tsx": {Data: []byte(`import { mount, signal, View, Text } from "nsgo";
import { Checkbox as Base } from "nsgo/base";
import { Checkbox as Styled } from "nsgo/ui";
function App() {
    const checked = signal(false), result = signal(""), legacy = signal(0);
    const receive = (e) => {
        const p = e.press;
        result.set([e.checked, p.modifiers.control, p.modifiers.alt, p.modifiers.shift, p.modifiers.platform,
            p.clickCount > 0, typeof p.position.x, typeof p.pagePosition.y].join("|"));
        checked.set(e.checked);
    };
    return <View>
        <Base testID="base" checked={checked()} onChange={() => legacy.set(legacy() + 1)} onChangeEvent={receive}
            style={{width: 180, height: 40}}><Text>Base</Text></Base>
        <Styled testID="styled" text="Styled" checked={checked()}
            onChange={() => legacy.set(legacy() + 1)} onChangeEvent={receive} />
        <Text>{() => "result " + result() + " legacy " + legacy()}</Text>
    </View>;
}
mount(App);`)}}
	app := nsgotest.Start(t, compiler.App(files, "App.tsx"),
		nsgotest.Environment(core.Environment{OS: core.OSMacOS, Idiom: core.IdiomDesktop}))
	for _, id := range []string{"base", "styled"} {
		app.PressWith("id:"+id, core.Modifiers{Control: true, Alt: true, Shift: true, Platform: true})
		app.Find("text:result true|true|true|true|true|true|number|number legacy 0")
		app.Focus("id:" + id)
		app.Key("shift+space")
		app.Find("text:result false|false|false|true|false|true|number|number legacy 0")
	}
	for _, module := range []string{"base", "ui"} {
		decl := tsx.Declarations()["types/nsgo/"+module+".d.ts"]
		if !strings.Contains(decl, "onChangeEvent?: (e: { checked: boolean; press: PressEvent }) => void") {
			t.Fatalf("%s declarations lack checked and PressEvent in onChangeEvent", module)
		}
	}
}
