//go:build nsgodev && !nsgoautomation

package nsgo

// devToken is the app's token for `nsgo dev -automation`'s server, which only its builds take (dev_automation.go).
const devToken = ""

// startAutomation does nothing: only nsgo dev -automation's builds link the agent.
func startAutomation(string) {}
