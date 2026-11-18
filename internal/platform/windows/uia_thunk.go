//go:build windows && (amd64 || arm64)

package windows

import (
	"syscall"
	"unsafe"
)

// The thunks' targets, read by uia_*.s, and their addresses.
var (
	uiaFromPointTarget    = syscall.NewCallback(uiaFromPointWords)
	uiaSetRangeTarget     = syscall.NewCallback(uiaSetRangeWords)
	uiaFromPointThunkAddr uintptr
	uiaSetRangeThunkAddr  uintptr
)

func fromPointSlot() uintptr { return uiaFromPointThunkAddr }
func setRangeSlot() uintptr  { return uiaSetRangeThunkAddr }

// variantArg is a whole VARIANT, 24 bytes on amd64 and arm64: variant and the rest of its union, which the callee may copy.
type variantArg struct {
	v variant
	_ uint64
}

// raisePropertyChanged calls UiaRaiseAutomationPropertyChangedEvent, whose VARIANTs come by value: amd64 and arm64 pass each as a pointer to a copy.
func raisePropertyChanged(provider uintptr, property int32, was, now *variant) {
	a, b := variantArg{v: *was}, variantArg{v: *now}
	uiaRaiseAutomationPropertyChangedEvent.Call(provider, uintptr(property), uintptr(unsafe.Pointer(&a)), uintptr(unsafe.Pointer(&b)))
}
