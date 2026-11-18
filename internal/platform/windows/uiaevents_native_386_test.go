//go:build windows && 386

package windows

import (
	"syscall"
	"unsafe"
)

// propertyChangedSlot is HandlePropertyChangedEvent: 386 pushes the VARIANT, four words.
func propertyChangedSlot() uintptr {
	return syscall.NewCallback(func(this *propertyHandler, sender uintptr, property int32, w0, w1, w2, w3 uintptr) uintptr {
		words := [4]uintptr{w0, w1, w2, w3}
		this.heard(property, (*variant)(unsafe.Pointer(&words)))
		return sOK
	})
}
