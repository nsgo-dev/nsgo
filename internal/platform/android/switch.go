//go:build android

package android

import "github.com/nsgo-dev/nsgo/internal/driver"

// systemSwitch is Android's Material 3 switch, implemented by the committed
// framework view so it follows the platform density, theme and accessibility
// stack rather than ui's drawn capsule.
type systemSwitch struct{ cx *driver.NativeContext }

func newSystemSwitch(cx *driver.NativeContext) driver.NativeComponent {
	cx.SetSize(driver.Size{Width: 52, Height: 32})
	return &systemSwitch{cx: cx}
}

func (*systemSwitch) View() any { return "dev.oneo.nsgo.Switch" }

func (*systemSwitch) SetProp(string, any) {}

func (s *systemSwitch) ViewEvent(name string, args []any) {
	if name != "onCheckedChanged" || len(args) < 2 {
		return
	}
	if checked, ok := args[1].(bool); ok {
		s.cx.Emit("change", checked)
	}
}

func (s *systemSwitch) Command(string, any) {}
func (*systemSwitch) Close()                {}
