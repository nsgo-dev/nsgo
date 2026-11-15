package driver

import "testing"

func TestNoNotification(t *testing.T) {
	permission := NoNotification(&MountItem{Kind: ItemNotificationPermission, Count: 7})
	if permission.Kind != EventNotification || permission.Count != 7 || permission.Index != NotificationsUnavailable || permission.Finished {
		t.Fatalf("permission = %+v", permission)
	}
	show := NoNotification(&MountItem{Kind: ItemShowNotification, Count: 8})
	if show.Kind != EventNotification || show.Count != 8 || show.Index != 0 || show.Finished {
		t.Fatalf("show = %+v", show)
	}
}
