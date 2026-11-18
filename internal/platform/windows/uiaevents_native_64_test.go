//go:build windows && !386

package windows

import "syscall"

// propertyChangedSlot is HandlePropertyChangedEvent: amd64 and arm64 pass the VARIANT as a pointer to a copy.
func propertyChangedSlot() uintptr {
	return syscall.NewCallback(func(this *propertyHandler, sender uintptr, property int32, value *variant) uintptr {
		this.heard(property, value)
		return sOK
	})
}
