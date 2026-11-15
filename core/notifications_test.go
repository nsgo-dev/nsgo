package core_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestNotificationsAskShowCloseAndReportPresses(t *testing.T) {
	var scope *core.Scope
	var asked []core.NotificationPermission
	var shown []bool
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	})
	answer := func(p core.NotificationPermission) { asked = append(asked, p) }
	show := func(n core.Notification) {
		if err := scope.ShowNotification(n, func(ok bool) { shown = append(shown, ok) }); err != nil {
			t.Fatal(err)
		}
	}

	show(core.Notification{ID: "early", Title: "Too early"})
	scope.NotificationPermission(answer)
	scope.RequestNotificationPermission(answer)
	scope.NotificationPermission(answer)
	app.Settle()
	if want := []core.NotificationPermission{core.NotificationsNotAsked, core.NotificationsGranted, core.NotificationsGranted}; !slices.Equal(asked, want) {
		t.Errorf("permissions %v, want %v", asked, want)
	}
	show(core.Notification{ID: "chat:1", Title: "Ada", Body: "Hi"})
	show(core.Notification{ID: "chat:2", Title: "Bo", Silent: true})
	show(core.Notification{ID: "chat:1", Title: "Ada", Body: "Hi again"})
	scope.CloseNotification("chat:2")
	scope.SetBadge(3)
	app.Settle()
	if !slices.Equal(shown, []bool{false, true, true, true}) {
		t.Errorf("shown %v", shown)
	}
	if len(app.Notifications) != 1 || app.Notifications[0].Body != "Hi again" || app.Badge != 3 {
		t.Errorf("notifications %+v, badge %d", app.Notifications, app.Badge)
	}
	if err := scope.ShowNotification(core.Notification{ID: "x"}, nil); !errors.Is(err, core.ErrNotification) {
		t.Errorf("a notification without a title: %v", err)
	}

	// A press before any handler waits for the first.
	app.PressNotification("chat:1")
	var pressed []string
	scope.OnNotificationPress(func(id string) { pressed = append(pressed, id) })
	app.Settle()
	show(core.Notification{ID: "chat:3", Title: "Cy"})
	app.Settle()
	app.PressNotification("chat:3")
	if !slices.Equal(pressed, []string{"chat:1", "chat:3"}) {
		t.Errorf("pressed %v", pressed)
	}

	// An empty ID names no notification, and closing all presses none.
	show(core.Notification{ID: "chat:4", Title: "Dee"})
	show(core.Notification{ID: "chat:5", Title: "Eli"})
	scope.CloseNotification("")
	app.Settle()
	if len(app.Notifications) != 2 {
		t.Fatalf("CloseNotification(\"\") left %+v", app.Notifications)
	}
	scope.CloseNotifications()
	scope.SetBadge(2)
	app.Settle()
	if len(app.Notifications) != 0 || app.Badge != 2 {
		t.Errorf("after CloseNotifications: notifications %+v, badge %d", app.Notifications, app.Badge)
	}
	if !slices.Equal(pressed, []string{"chat:1", "chat:3"}) {
		t.Errorf("closing all pressed %v", pressed)
	}
}

func TestCloseNotificationsHeldToAccess(t *testing.T) {
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	})
	scope.RequestNotificationPermission(nil)
	app.Settle()
	for _, id := range []string{"a", "b"} {
		if err := scope.ShowNotification(core.Notification{ID: id, Title: id}, nil); err != nil {
			t.Fatal(err)
		}
	}
	refused := scope.Child()
	refused.ProvideAccess(denyAll{})
	refused.CloseNotifications()
	app.Settle()
	if len(app.Notifications) != 2 {
		t.Errorf("refused code closed notifications: %+v", app.Notifications)
	}
}

func TestNotificationsHeldToAccess(t *testing.T) {
	var got core.NotificationPermission
	var shown = true
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		cx.ProvideAccess(denyAll{})
		cx.RequestNotificationPermission(func(p core.NotificationPermission) { got = p })
		_ = cx.ShowNotification(core.Notification{ID: "a", Title: "A"}, func(ok bool) { shown = ok })
		cx.SetBadge(2)
		return core.View{}
	})
	app.Settle()
	if got != core.NotificationsUnavailable || shown || app.Badge != 0 {
		t.Errorf("refused code: permission %v, shown %v, badge %d", got, shown, app.Badge)
	}
}

// TestTaskProgressClampsAndClears checks what nsgotest records; TestTaskProgressRepeatsWhatAnotherTreeChanged checks which repeats are sent.
func TestTaskProgressClampsAndClears(t *testing.T) {
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	})
	scope.SetTaskProgress(core.TaskProgress{State: core.TaskProgressNormal, Value: 2})
	app.Settle()
	if got := app.TaskProgress; got.State != core.TaskProgressNormal || got.Value != 1 {
		t.Errorf("progress = %+v, want normal 1", got)
	}
	scope.SetTaskProgress(core.TaskProgress{State: core.TaskProgressIndeterminate, Value: .8})
	app.Settle()
	if got := app.TaskProgress; got != (core.TaskProgress{State: core.TaskProgressIndeterminate}) {
		t.Errorf("indeterminate progress = %+v, want no value", got)
	}
	scope.SetTaskProgress(core.TaskProgress{})
	app.Settle()
	if got := app.TaskProgress; got != (core.TaskProgress{}) {
		t.Errorf("cleared progress = %+v", got)
	}
}

func TestTaskProgressRejectsNaN(t *testing.T) {
	var scope *core.Scope
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		scope = cx
		return core.View{}
	})
	scope.SetTaskProgress(core.TaskProgress{State: core.TaskProgressNormal, Value: math.NaN()})
	app.Settle()
	if got := app.TaskProgress.Value; got != 0 {
		t.Errorf("NaN progress = %g, want 0", got)
	}
}

type denyAll struct{}

func (denyAll) Allows(core.AccessRequest) bool { return false }
func (denyAll) WebHosts() []string             { return nil }
