//go:build nsgodev && nsgoautomation

package nsgo

import (
	"os"

	"github.com/nsgo-dev/nsgo/internal/devtools"
	// The agent nsgo dev's automation drives the app through (nsgotest's automation.go), which sets devtools.Automation.
	_ "github.com/nsgo-dev/nsgo/nsgotest"
)

// devToken is the app's token for `nsgo dev -automation`'s server, which the tool gives each run of the app: NSGO_DEV_TOKEN, taken out of the environment so the programs the app starts never inherit it.
var devToken = os.Getenv("NSGO_DEV_TOKEN")

func init() { os.Unsetenv("NSGO_DEV_TOKEN") }

// startAutomation has the agent answer the automation over DevTools' channel, when nsgo dev gave the app a token (startDevClient); without one, it follows nothing.
func startAutomation(server string) {
	if devToken == "" || devtools.Automation == nil {
		devtools.Automation = nil
		return
	}
	openDevChannel(server)
}
