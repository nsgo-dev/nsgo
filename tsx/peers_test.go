package tsx_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
)

func TestPeerClassesFollowTSXStateAndMarkers(t *testing.T) {
	app := run(t, `import { mount, signal, View, Text } from "nsgo";
import { Button } from "nsgo/base";
function App() {
    const bad = signal(false), marked = signal(true);
    return <View>
        <Text testID="before" class="peer-checked:bg-[#ff0000]">Before</Text>
        <View testID="control" class={marked() ? "peer peer/field" : ""}
            accessibilityState={{ checked: bad(), disabled: bad(), invalid: bad() }} />
        <Text testID="label" class="bg-[#001122] border peer-checked:bg-[#ff0000] peer-disabled:opacity-50 peer-aria-invalid:border-[3px]">Label</Text>
        <Text testID="named" class="bg-[#001122] peer-checked/field:bg-[#ff0000]">Named</Text>
        <Button testID="validate" onPress={() => bad.set(!bad())}><Text>Validate</Text></Button>
        <Button testID="mark" onPress={() => marked.set(!marked())}><Text>Mark</Text></Button>
    </View>;
}
mount(App);`)
	check := func(bad, marked bool) {
		t.Helper()
		color, border, opacity := core.RGB(0x001122), core.Points(1), float32(1)
		if bad && marked {
			color, border, opacity = core.RGB(0xff0000), 3, 0.5
		}
		label := app.FindMounted("id:label")
		if label.Style.Background != color || label.Style.BorderWidth != border || label.Style.Opacity != opacity {
			t.Errorf("bad=%v, marked=%v: color=%x border=%v opacity=%v", bad, marked, label.Style.Background, label.Style.BorderWidth, label.Style.Opacity)
		}
		if got := app.FindMounted("id:named").Style.Background; got != color {
			t.Errorf("named peer color=%x, want %x", got, color)
		}
		if got := app.FindMounted("id:control").Props.Invalid; got != bad {
			t.Errorf("TSX View invalid=%v, want %v", got, bad)
		}
		if app.FindMounted("id:before").Style.Background == core.RGB(0xff0000) {
			t.Error("a following marker styled an earlier sibling")
		}
	}
	check(false, true)
	app.Press("id:validate")
	check(true, true)
	app.Press("id:mark")
	check(true, false)
	app.Press("id:mark")
	check(true, true)
	app.Press("id:validate")
	check(false, true)
}
