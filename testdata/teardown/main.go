// Command teardown shows one text: cmd/nsgo's TestTargetTeardownEndToEnd runs its tests on a desktop target to check that nsgotest.MainFunc's teardown runs after them.
package main

import (
	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/core"
)

func main() { nsgo.Run(Root) }

// Root is the app's one view.
func Root(cx *core.Scope) core.Node { return core.Text{Content: "Hello"} }
