//go:build nsgoautomation

package nsgotest

import "github.com/nsgo-dev/nsgo/internal/devtools"

// automationBuild is set in `nsgo dev -automation`'s builds, which link the agent into the running app.
const automationBuild = true

func init() { devtools.Automation = newAgent() }
