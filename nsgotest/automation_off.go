//go:build !nsgoautomation

package nsgotest

// automationBuild is set in `nsgo dev -automation`'s builds, which link the agent into the running app.
const automationBuild = false
