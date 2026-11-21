//go:build !nsgodev && !nsgotest && !nsgocheck

package core

import (
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Release builds compile signal-trigger diagnostics away completely: zero extra
// bytes on Signal and devState, and empty inlineable hooks everywhere else.
type (
	devSignalID struct{}
	devTriggers struct{}
	devWrite    struct{}
	pendingList struct{}
)

func (s *devSignalID) setDevID(uint64)     {}
func (s *devSignalID) devSignalID() uint64 { return 0 }

func noteSignalWriteStart[T any](*Signal[T], T) *devWrite { return nil }
func noteSignalWriteEnd(*runtime, *devWrite)              {}

func (*computation) noteMarkedDirty()                        {}
func (*computation) beginRunCausal() pendingList             { return pendingList{} }
func (*computation) endRunCausal(pendingList, time.Duration) {}
func (*computation) beginMemoPropagate(pendingList)          {}
func (*computation) endMemoPropagate()                       {}

func (*runtime) labelComp(*computation, string, any) {}

func (*devState) initTriggers()                     {}
func (*devState) forgetTriggers(*Scope)             {}
func (*devState) compLabel(*computation) string     { return "" }
func (*devState) takeTriggers(int) []driver.Trigger { return nil }
