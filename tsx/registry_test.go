package tsx

import (
	"reflect"
	"testing"

	"github.com/nsgo-dev/nsgo/base"
)

func init() {
	registerPart("nsgo/base", "HiddenPart", base.ResizablePanel{})
}

// A part sandboxes import is marked SandboxSafe, as ResizablePanel is; one without it is not.
func TestPartsAreHiddenUnlessSandboxSafe(t *testing.T) {
	if isSandboxSafe("nsgo/base", "HiddenPart") || !isSandboxSafe("nsgo/base", "ResizablePanel") {
		t.Fatalf("HiddenPart %v, ResizablePanel %v", isSandboxSafe("nsgo/base", "HiddenPart"), isSandboxSafe("nsgo/base", "ResizablePanel"))
	}
}

// A function's package is its name up to the first dot after the last slash.
func TestFuncPackage(t *testing.T) {
	for name, want := range map[string]string{
		"github.com/nsgo-dev/nsgo/tsx/ui.init.0":          "github.com/nsgo-dev/nsgo/tsx/ui",
		"github.com/nsgo-dev/nsgo/tsx.RegisterClass[...]": "github.com/nsgo-dev/nsgo/tsx",
		"example.com/app.(*T).M.func1":                    "example.com/app",
		"gopkg.in/yaml%2ev3.Unmarshal":                    "gopkg.in/yaml%2ev3",
		"main.init.0":                                     "main",
		"runtime.call32":                                  "runtime",
	} {
		if got := funcPackage(name); got != want {
			t.Errorf("funcPackage(%q) = %q, want %q", name, got, want)
		}
	}
}

// A default must be of the type its field, or its getter, gives.
func TestDefaultOfTheWrongTypePanics(t *testing.T) {
	type gauge struct {
		Max     float64
		MaxFunc func() float64
		Step    func() float64
	}
	planFields(reflect.TypeFor[gauge](), newSpec([]Option{Default("Max", float64(100)), Default("Step", float64(1))}))
	for _, opt := range []Option{Default("Max", 100), Default("Step", "1")} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("a default of the wrong type planned without a panic")
				}
			}()
			planFields(reflect.TypeFor[gauge](), newSpec([]Option{opt}))
		}()
	}
}
