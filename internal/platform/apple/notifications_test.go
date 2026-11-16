//go:build darwin

package apple

import "testing"

// The delegate's class registers once: asking for the permission, showing notifications and launching each set the delegate, and a second registration would panic.
func TestNotificationDelegateRegistersOnce(t *testing.T) {
	first, second := notificationDelegate(), notificationDelegate()
	if first == 0 || first != second {
		t.Fatalf("notificationDelegate() = %#x then %#x, want one delegate", first, second)
	}
	if !MsgBool(first, sel("respondsToSelector:"), uintptr(sel("userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:"))) {
		t.Fatal("the delegate does not answer notification presses")
	}
}
