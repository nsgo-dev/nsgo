package tsx_test

import (
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx"
)

func TestBaseControlsFollowInvalidFromTSX(t *testing.T) {
	app := run(t, `import { mount, signal, View, Text } from "nsgo";
import { Button, Checkbox, Switch, Radio, RadioGroup, Toggle, ToggleGroup, Link, Tab, Tabs, Slider, NumberInput, OtpInput, Select, Combobox, TimeField, DatePicker, Calendar, ColorPicker, ColorArea, ColorSlider, ColorField, ColorEyeDropper, List, Tree } from "nsgo/base";
function App() {
    const bad = signal(false);
    return <View>
        <Button testID="flip" onPress={() => bad.set(!bad())}><Text>Validate</Text></Button>
        <Button testID="Button" onPress={() => bad.set(false)} invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <Checkbox testID="Checkbox" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <Switch testID="Switch" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <Radio testID="Radio" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <RadioGroup testID="RadioGroup" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <Toggle testID="Toggle" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <ToggleGroup testID="ToggleGroup" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <Link testID="Link" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <Tab testID="Tab" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <Tabs testID="Tabs" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <Slider testID="Slider" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <NumberInput testID="NumberInput" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <OtpInput testID="OtpInput" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} length={4} />
        <Select testID="Select" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} items={[]} />
        <Combobox testID="Combobox" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} items={[]} />
        <TimeField testID="TimeField" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <DatePicker testID="DatePicker" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <Calendar testID="Calendar" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <ColorPicker testID="ColorPicker" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <ColorArea testID="ColorArea" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <ColorSlider testID="ColorSlider" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <ColorField testID="ColorField" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <ColorEyeDropper testID="ColorEyeDropper" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} />
        <List testID="List" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} each={[]} />
        <Tree testID="Tree" invalid={bad()} class="aria-invalid:bg-[#ff0000]" style={{ width: 160, height: 40 }} items={[]} />
    </View>;
}
mount(App);`)
	names := strings.Fields("Button Checkbox Switch Radio RadioGroup Toggle ToggleGroup Link Tab Tabs Slider NumberInput OtpInput Select Combobox TimeField DatePicker Calendar ColorPicker ColorArea ColorSlider ColorField ColorEyeDropper List Tree")
	for _, invalid := range []bool{false, true, false} {
		if invalid || app.FindMounted("id:Button").Props.Invalid {
			if invalid {
				app.Press("id:flip")
			} else {
				app.Press("id:Button")
			}
		}
		color := core.Color(0)
		if invalid {
			color = core.RGB(0xff0000)
		}
		for _, name := range names {
			v := app.FindMounted("id:" + name)
			if v.Props.Invalid != invalid || v.Style.Background != color {
				t.Errorf("%s invalid=%v: props %v, background %x; want %x", name, invalid, v.Props.Invalid, v.Style.Background, color)
			}
		}
	}
	decls := tsx.Declarations()["types/nsgo/base.d.ts"]
	for _, name := range names {
		_, rest, ok := strings.Cut(decls, "export interface "+name+"Props")
		if !ok {
			t.Errorf("no %sProps declaration", name)
			continue
		}
		body, _, _ := strings.Cut(rest, "\n}")
		if !strings.Contains(body, "invalid?: boolean;") {
			t.Errorf("%sProps does not declare invalid", name)
		}
	}
}
