package desktop

import "github.com/nsgo-dev/nsgo/core"

// GuideOptions say which app GuidePermission holds out for the user to drag.
type GuideOptions = core.PermissionGuideOptions

// GuidePermission opens the page of the system's settings that gives p and waits for the user to give it. On macOS, for the panes whose list takes an app dragged into it (all but the microphone's, the camera's, the calendars' and the reminders'), a panel under System Settings' window follows it, never takes the focus, and holds the app for the user to drag into the list. It ends, and done hears the status then, once the app has p, the user closes the panel or System Settings, stop is called or the current owner goes; one guide shows at a time, and a new one ends the one before. On Windows it opens the microphone's or the camera's page and ends once the app has p or Settings closes. Where nothing guides, as on Linux and off desktops, done hears the status at once.
func GuidePermission(cx *core.Scope, p core.Permission, opts GuideOptions, done func(core.PermissionStatus)) (stop func()) {
	if !IsAvailable(cx) {
		cx.PermissionStatus(p, done)
		return func() {}
	}
	return cx.GuidePermission(p, opts, done)
}
