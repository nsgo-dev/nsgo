package driver

// The permissions an ItemPermission names (PermissionRequest.Name), as core.Permission's values.
const (
	PermissionAccessibility   = "accessibility"
	PermissionScreenRecording = "screen-recording"
	PermissionInputMonitoring = "input-monitoring"
	PermissionFullDiskAccess  = "full-disk-access"
	PermissionAppManagement   = "app-management"
	PermissionDeveloperTools  = "developer-tools"
	PermissionMicrophone      = "microphone"
	PermissionCamera          = "camera"
	PermissionCalendars       = "calendars"
	PermissionReminders       = "reminders"
	PermissionBluetooth       = "bluetooth"
	PermissionMediaLibrary    = "media-library"
)

// The statuses EventPermission answers with (Index), as core.PermissionStatus.
const (
	PermissionNotAsked = iota
	PermissionGranted
	PermissionDenied
	PermissionUnknown
	PermissionNotGated
	PermissionUnavailable
)

// PermissionCommand is what an ItemPermission asks.
type PermissionCommand uint8

const (
	// PermissionCheck answers the status without asking the user.
	PermissionCheck PermissionCommand = iota
	// PermissionAsk shows the system's prompt where it has one and has not asked yet, then answers the status.
	PermissionAsk
	// PermissionOpenSettings opens the page of the system's settings that gives the permission; EventPermission's Finished says whether one opened.
	PermissionOpenSettings
	// PermissionGuide opens the settings and guides the user there; it is answered once, as the guide ends, with the status then.
	PermissionGuide
	// PermissionStopGuide ends the guide whose request was Count, which still answers its own request; it is not answered itself.
	PermissionStopGuide
)

// PermissionRequest is an ItemPermission's.
type PermissionRequest struct {
	Command PermissionCommand
	// Name is the permission's key, one of the Permission* keys; "" for PermissionStopGuide.
	Name string
	// App is the path of the app PermissionGuide holds out to drag, and AppName what it calls it; "" for the defaults.
	App, AppName string
}
