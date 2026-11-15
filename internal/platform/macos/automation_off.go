//go:build darwin && !ios && !nsgoautomation

package macos

// automationBuild is set in `nsgo dev -automation`'s builds: surfaces keep their alerts for the automation to answer, and it acts on the running app's windows through liveSurface.
const automationBuild = false
