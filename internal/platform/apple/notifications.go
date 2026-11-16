//go:build darwin

package apple

import (
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/driver"
	"github.com/nsgo-dev/nsgo/internal/platform/objcblock"
)

// System notifications for macOS and iOS through UserNotifications' UNUserNotificationCenter, as NSH's notifications: its delegate shows them while the app is in front and reports their presses, the one that launched the app included. The center serves only an app in a bundle, and raises for a bare executable, so one outside a bundle answers unavailable. A scheduled notification is the center's pending request, which the system shows at its trigger with the app's process ended.

const notificationsPath = "/System/Library/Frameworks/UserNotifications.framework/UserNotifications"

// notifications holds what the delegate reports: the presses it heard
// before a surface listened, and where they go once one does.
var notifications struct {
	mu      sync.Mutex
	press   func(id string)
	pending []string
}

// hasNotificationCenter reports whether the process has a notification center, an app bundle with the framework, without touching the center, which on macOS ties the Dock badge to the user's badge setting.
var hasNotificationCenter = sync.OnceValue(func() bool {
	bundle := objc.ID(Class("NSBundle")).Send(sel("mainBundle"))
	if bundle.Send(sel("bundleIdentifier")) == 0 || !strings.HasSuffix(GoString(bundle.Send(sel("bundlePath"))), ".app") {
		return false
	}
	if _, err := purego.Dlopen(notificationsPath, purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		return false
	}
	return Class("UNUserNotificationCenter") != 0
})

// notificationCenter returns the current center, or 0 where the process has none: outside an app bundle, or without the framework.
var notificationCenter = sync.OnceValue(func() objc.ID {
	if !hasNotificationCenter() {
		return 0
	}
	return objc.ID(Class("UNUserNotificationCenter")).Send(sel("currentNotificationCenter"))
})

// notificationsUsedKey is the defaults key NSGo sets once the app checks or asks for the notification permission or shows a notification, so later launches set the delegate before launching ends and hear a press on a notification an earlier run showed.
const notificationsUsedKey = "NSGoNotifications"

// NotificationsUsed reports whether the app checked or asked for the notification permission or showed a notification, in this run or an earlier one, without touching the center: a bundled macOS app whose process touched it has its Dock badge shown only while the user allows its badges.
func NotificationsUsed() bool {
	defaults := objc.ID(Class("NSUserDefaults")).Send(sel("standardUserDefaults"))
	return MsgBool(defaults, sel("boolForKey:"), uintptr(NSString(notificationsUsedKey)))
}

// useNotifications returns the center for the app's use of notifications, or 0 where it has none, recording that use and making NSGo its delegate.
func useNotifications() objc.ID {
	center := notificationCenter()
	if center == 0 {
		return 0
	}
	if !NotificationsUsed() {
		defaults := objc.ID(Class("NSUserDefaults")).Send(sel("standardUserDefaults"))
		defaults.Send(sel("setBool:forKey:"), true, NSString(notificationsUsedKey))
	}
	WatchNotifications()
	return center
}

// notificationDelegate is the one delegate WatchNotifications gives the center: its class can be registered only once.
var notificationDelegate = sync.OnceValue(func() objc.ID {
	// UNNotificationPresentationOptions: banner, list, sound and badge, as the notification asks.
	const present = 1<<4 | 1<<3 | 1<<1 | 1<<0
	var protocols []*objc.Protocol
	if p := objc.GetProtocol("UNUserNotificationCenterDelegate"); p != nil {
		protocols = append(protocols, p)
	}
	return objc.ID(RegisterClass("NsgoNotificationDelegate", Class("NSObject"), protocols, []objc.MethodDef{
		{Cmd: sel("userNotificationCenter:willPresentNotification:withCompletionHandler:"),
			Fn: func(_ objc.ID, _ objc.SEL, _, _, done objc.ID) { CallBlock(done, present) }},
		{Cmd: sel("userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:"),
			Fn: func(_ objc.ID, _ objc.SEL, _, response, done objc.ID) {
				action := GoString(response.Send(sel("actionIdentifier")))
				if action == "com.apple.UNNotificationDefaultActionIdentifier" {
					notificationPressed(GoString(response.Send(sel("notification")).Send(sel("request")).Send(sel("identifier"))))
				}
				CallBlock(done)
			}},
	})).Send(sel("new"))
})

// WatchNotifications makes NSGo the center's delegate; calling it again changes nothing. Call it as the app starts, before it finishes launching, so the press that launched it is heard; macOS calls it then only for an app that used notifications before (NotificationsUsed), so one that only sets a Dock badge never touches the center.
func WatchNotifications() {
	center := notificationCenter()
	if center == 0 {
		return
	}
	center.Send(sel("setDelegate:"), notificationDelegate())
}

// SetNotificationPress sends the presses of notifications to press, those
// heard before it first.
func SetNotificationPress(press func(id string)) {
	notifications.mu.Lock()
	notifications.press = press
	pending := notifications.pending
	notifications.pending = nil
	notifications.mu.Unlock()
	for _, id := range pending {
		press(id)
	}
}

func notificationPressed(id string) {
	notifications.mu.Lock()
	press := notifications.press
	if press == nil {
		notifications.pending = append(notifications.pending, id)
	}
	notifications.mu.Unlock()
	if press != nil {
		press(id)
	}
}

// NotificationPermission answers with whether the app may show
// notifications (driver.NotificationsNotAsked, ...), asking the user
// first when ask is set and the user was not asked yet. answer runs on
// any goroutine.
func NotificationPermission(ask bool, answer func(int)) {
	center := useNotifications()
	if center == 0 {
		answer(driver.NotificationsUnavailable)
		return
	}
	query := func() {
		read := objcblock.New(func(_ objc.Block, settings objc.ID) {
			// UNAuthorizationStatus: not determined, denied, authorized,
			// provisional and ephemeral.
			switch objc.Send[int](settings, sel("authorizationStatus")) {
			case 0:
				answer(driver.NotificationsNotAsked)
			case 1:
				answer(driver.NotificationsDenied)
			default:
				answer(driver.NotificationsGranted)
			}
		})
		center.Send(sel("getNotificationSettingsWithCompletionHandler:"), read)
		read.Release()
	}
	if !ask {
		query()
		return
	}
	// UNAuthorizationOptions: badge, sound and alert.
	const options = 1<<0 | 1<<1 | 1<<2
	asked := objcblock.New(func(_ objc.Block, _ bool, _ objc.ID) { query() })
	center.Send(sel("requestAuthorizationWithOptions:completionHandler:"), uint(options), asked)
	asked.Release()
}

// NotificationsAvailable reports whether the process has a notification center, which also shows the notifications it schedules after the process ends: Environment.ScheduledNotifications. It leaves the center untouched, as the environment is read before the app uses notifications.
func NotificationsAvailable() bool { return hasNotificationCenter() }

// ShowNotification shows n, or schedules it for its At, replacing a notification of its ID, delivered or pending, and answers with whether the center took it: not without the permission. answer runs on any goroutine.
func ShowNotification(n driver.Notification, answer func(bool)) {
	center := useNotifications()
	if center == 0 {
		answer(false)
		return
	}
	content := objc.ID(Class("UNMutableNotificationContent")).Send(sel("new"))
	content.Send(sel("setTitle:"), NSString(n.Title))
	content.Send(sel("setBody:"), NSString(n.Body))
	if !n.Silent {
		content.Send(sel("setSound:"), objc.ID(Class("UNNotificationSound")).Send(sel("defaultSound")))
	}
	request := objc.ID(Class("UNNotificationRequest")).Send(sel("requestWithIdentifier:content:trigger:"),
		NSString(n.ID), content, notificationTrigger(n))
	content.Send(sel("release"))
	added := objcblock.New(func(_ objc.Block, failure objc.ID) { answer(failure == 0) })
	center.Send(sel("addNotificationRequest:withCompletionHandler:"), request, added)
	added.Release()
}

// notificationTrigger is when the center shows n: nil for now, an interval's trigger for one At, and for a repeated one a calendar trigger matching At's time of day, and its weekday for a weekly one, in the device's time zone as it changes.
func notificationTrigger(n driver.Notification) objc.ID {
	if n.At.IsZero() {
		return 0
	}
	if n.Repeat == driver.NotificationOnce {
		// The interval must be positive; one just passed shows at once.
		return objc.ID(Class("UNTimeIntervalNotificationTrigger")).Send(sel("triggerWithTimeInterval:repeats:"),
			max(time.Until(n.At).Seconds(), 0.001), false)
	}
	parts := objc.ID(Class("NSDateComponents")).Send(sel("new"))
	parts.Send(sel("setHour:"), n.At.Hour())
	parts.Send(sel("setMinute:"), n.At.Minute())
	parts.Send(sel("setSecond:"), n.At.Second())
	if n.Repeat == driver.NotificationWeekly {
		// NSCalendar's weekdays start at 1, Sunday.
		parts.Send(sel("setWeekday:"), int(n.At.Weekday())+1)
	}
	trigger := objc.ID(Class("UNCalendarNotificationTrigger")).Send(sel("triggerWithDateMatchingComponents:repeats:"), parts, true)
	parts.Send(sel("release"))
	return trigger
}

// CloseNotification removes the notification called id, shown or still to show, and every one the app's notification center holds when id is empty, those earlier runs left included. A macOS app that never used notifications has none to close, and closing leaves its center untouched, as an app clearing them each time it comes to the front would otherwise make its Dock badge wait for the user to allow badges.
func CloseNotification(id string) {
	if runtime.GOOS == "darwin" && !NotificationsUsed() {
		return
	}
	center := notificationCenter()
	if center == 0 {
		return
	}
	if id == "" {
		center.Send(sel("removeAllDeliveredNotifications"))
		center.Send(sel("removeAllPendingNotificationRequests"))
		return
	}
	ids := objc.ID(Class("NSArray")).Send(sel("arrayWithObject:"), NSString(id))
	center.Send(sel("removeDeliveredNotificationsWithIdentifiers:"), ids)
	center.Send(sel("removePendingNotificationRequestsWithIdentifiers:"), ids)
}

// SetCenterBadge sets the badge through the center, as iOS 16 and later
// do, and reports whether it could.
func SetCenterBadge(count uint32) bool {
	center := notificationCenter()
	if center == 0 || !MsgBool(center, sel("respondsToSelector:"), uintptr(sel("setBadgeCount:withCompletionHandler:"))) {
		return false
	}
	center.Send(sel("setBadgeCount:withCompletionHandler:"), int(count), objc.ID(0))
	return true
}
