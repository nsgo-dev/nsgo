package tsx_test

import (
	"strings"
	"testing"

	"github.com/nsgo-dev/nsgo/tsx"
)

func TestVisibleLabelsInTSX(t *testing.T) {
	app := run(t, `import { mount, signal, Text, View } from "nsgo";
import { Checkbox, Input, Slider } from "nsgo/base";
function App() {
  const field = signal(null), check = signal(null), heading = signal(null);
  const name = signal("Email"), off = signal(false), bad = signal(false), on = signal(false);
  return <View>
    <Text testID="field-label" labelFor={field()}>{name()}</Text>
    <Input testID="field" ref={n => field.set(n)} />
    <Text testID="check-label" labelFor={check()}>Updates</Text>
    <Checkbox testID="check" ref={n => check.set(n)} disabled={off()} invalid={bad()}
      checked={on()} onChange={e => on.set(e.checked)} />
    <Text ref={n => heading.set(n)}>Volume</Text>
    <Slider testID="slider" thumbTestID="thumb" labelledBy={[heading()]} style={{height: 32}} thumbStyle={{width: 12, height: 12}} />
    <View testID="change" style={{height: 24}} onPress={() => { name.set("Address"); off.set(!off()); bad.set(!bad()); }} />
    <Text testID="state">{String(on())}</Text>
  </View>;
}
mount(App);`)
	if p := app.Find("id:field").Props; p.Label != "Email" || p.LabelledBy == nil {
		t.Fatalf("input name = %+v", p)
	}
	app.Press("id:field-label")
	if !app.Find("id:field").Focused {
		t.Fatal("label did not focus TSX input")
	}
	app.Press("id:check-label")
	if app.Find("id:state").Text != "true" {
		t.Fatal("label did not toggle Go component")
	}
	if p := app.Find("id:thumb").Props; p.Label != "Volume" || p.LabelledBy == nil {
		t.Fatalf("component refs = %+v", p)
	}
	app.Press("id:change")
	if p := app.Find("id:field").Props; p.Label != "Address" {
		t.Fatalf("dynamic label = %+v", p)
	}
	if p := app.Find("id:check-label").Props; !p.Disabled || !p.Invalid {
		t.Fatalf("label states = %+v", p)
	}
	app.Press("id:change")
	app.Press("id:check-label")
	if app.Find("id:state").Text != "false" {
		t.Fatal("restored label did not toggle")
	}
	decls := tsx.Declarations()["types/nsgo/base.d.ts"]
	if !strings.Contains(decls, "labelledBy?: (YNode | null)[]") {
		t.Fatalf("reference declaration missing from base types: %s", decls)
	}
}
