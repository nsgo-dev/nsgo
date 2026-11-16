---
title: Notifications
description: Ask to show notifications, show and close them, count them and show a task's progress on the app's icon and hear presses.
group: Platform features
order: 45
---

## Permission

`cx.RequestNotificationPermission` asks the user to let the app show notifications, with the system's prompt the first time, and answers with a `core.NotificationPermission`: `NotificationsGranted`, `NotificationsDenied` or `NotificationsUnavailable` where the platform or the build shows none. Ask as the user does something that needs notifications: iOS and Android ask only once, and browsers refuse a prompt the user did not cause. `cx.NotificationPermission` reads the answer without asking.

```go
cx.RequestNotificationPermission(func(p core.NotificationPermission) {
	if p == core.NotificationsGranted {
		cx.ShowNotification(core.Notification{ID: "chat:42", Title: "Ada", Body: "Lunch?"}, nil)
	}
})
```

## Showing one

`cx.ShowNotification` shows a notification, also while the app is in front, and replaces one of the same `ID`; `Silent` leaves out the system's sound and vibration. It answers `done` with whether the system took the notification, false without the permission, and returns `core.ErrNotification`, calling no `done`, for one without an ID or a title. `cx.CloseNotification` removes the one an ID names.

```go
err := cx.ShowNotification(core.Notification{ID: "sync", Title: "Synced", Body: "12 notes"}, func(shown bool) {})
cx.CloseNotification("sync")
```

`cx.CloseNotifications` removes every notification the app showed. On macOS, iOS, Windows and Android that includes those earlier runs left in Notification Center, Action Center or the shade; on Linux and the web it reaches only those this run showed. Call it as the app comes to the front when its window shows what they told:

```go
cx.Effect(func() {
	if cx.Env().Lifecycle() == core.LifecycleActive {
		cx.CloseNotifications()
		cx.SetBadge(0)
	}
})
```

## Presses

`cx.OnNotificationPress` runs its function with the `ID` of a notification the user pressed, while the component that set it lives, and brings the app to the front. A press that started the app reaches the first handler registered. Keep in `ID` what the press opens (`"chat:42"`), as a press can start the app anew. On Windows it does so for an app its installer put in the Start menu; one run otherwise hears presses only while it runs.

```go
cx.OnNotificationPress(func(id string) { openChat(strings.TrimPrefix(id, "chat:")) })
```

An Android app asking from Android 13 lists `android.permission.POST_NOTIFICATIONS` in `nsgo.yaml`'s `android.permissions`.

## Badge

`cx.SetBadge` puts a count on the app's icon: the Dock's on macOS, the Home Screen's on iOS, which needs the notification permission, the taskbar button's on Windows, Unity-compatible Linux launchers and an installed web app's. A count of 0 or less clears it. A macOS app needs no permission for its Dock badge until it checks or asks for the notification permission or shows a notification; from then on, in that run and later ones, the Dock shows the badge only while the user allows the app's badges in System Settings, which `cx.RequestNotificationPermission` asks for with the alerts. Launchers without a Linux badge interface and Android have no count to set; Android launchers mark an app whose notifications show.

```go
cx.SetBadge(len(unread.Get()))
```

## Task progress

`cx.SetTaskProgress` shows how far a long task has gone on the app's icon: a bar over the Dock tile on macOS, the taskbar button on Windows and Unity-compatible Linux launchers. It needs no permission and shows beside the badge. `TaskProgressNormal` shows `Value`, a fraction from 0 to 1; `TaskProgressPaused` and `TaskProgressError` color it on Windows and macOS; `TaskProgressIndeterminate` moves on Windows and is a still, half-filled bar on macOS and Linux; the zero `TaskProgress` clears it. On Windows it shows on the button of the scope's window, and progress set where no window has a button of its own, such as the app's own scope or a tray panel, shows on every window of the app that sets none. iOS, Android and the web show nothing.

```go
cx.SetTaskProgress(core.TaskProgress{State: core.TaskProgressIndeterminate}) // no fraction yet
cx.SetTaskProgress(core.TaskProgress{State: core.TaskProgressNormal, Value: .43})
cx.SetTaskProgress(core.TaskProgress{}) // done
```

## Next

- [Permissions](permissions.md): the system's other permissions, such as the microphone and Accessibility.
- [Haptics](haptics.md): the feedback the device plays under the user's finger.
- [Platform services](services.md): alerts, the clipboard, files, storage and networking.
- [Live Activities](live-activities.md): what the app follows, on the Lock Screen and in the Dynamic Island.
- [Building and shipping](building.md): the permissions a package needs.
