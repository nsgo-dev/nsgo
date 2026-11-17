package tsx_test

import (
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx"
)

type relationalIcon struct{ TestID string }

func (i relationalIcon) Render(*core.Scope) core.Node {
	return core.Icon{TestID: i.TestID, SVG: core.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path d="M0 0H24V24H0Z"/></svg>`))}
}

func init() {
	tsx.Register("app/relational", map[string]core.Node{"Icon": relationalIcon{}})
}

func TestRelationalClassesFromTSX(t *testing.T) {
	app := run(t, `import { mount, signal, View, Text } from "nsgo";
import { Icon } from "app/relational";
function App() {
    const visible = signal(false);
    return <View>
        <View testID="toggle" class="h-8" onPress={() => visible.set(!visible())}><Text>Toggle icon</Text></View>
        <View testID="button" class="flex-row px-8 has-[>svg]:px-3 [&_svg]:size-4 [&_svg]:shrink-0 hover:[&_svg]:opacity-50">
            <View testID="wrapper"><Icon testID="nested" /></View>
            <If when={visible()}><Icon testID="direct" /></If>
        </View>
    </View>;
}
mount(App);`)
	check := func(padding float32) {
		t.Helper()
		button, icon := app.Find("id:button"), app.Find("id:nested")
		if app.Find("id:wrapper").Bounds().Origin.X-button.Bounds().Origin.X != core.Points(padding) || icon.Frame.Size.Width != 16 || icon.Frame.Size.Height != 16 {
			t.Fatalf("padding/size: %v, %+v", button.Frame, icon.Frame)
		}
	}
	check(32)
	app.Press("id:toggle")
	check(12)
	if icon := app.Find("id:direct"); icon.Frame.Size.Width != 16 || icon.Frame.Size.Height != 16 {
		t.Fatalf("new icon did not inherit selector size: %+v", icon.Frame)
	}
	app.Hover("id:button")
	if got := app.Find("id:nested").Style.Opacity; got != .5 {
		t.Fatalf("ancestor hover opacity=%v", got)
	}
	app.HoverOut()
	app.Press("id:toggle")
	check(32)
	if got := app.Find("id:nested").Style.Opacity; got != 1 {
		t.Fatalf("ancestor hover did not clear: %v", got)
	}
}
