//go:build !nsgodev && !nsgotest && !nsgocheck

package core

import (
	"github.com/nsgo-dev/nsgo/internal/devtools"
	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Release builds compile owner-disposal diagnostics away completely: zero extra
// bytes on devState, and empty inlineable hooks everywhere else.
type (
	devDisposals  struct{}
	disposalGuard struct{}
)

func (*computation) beginRerunDispose() disposalGuard { return disposalGuard{} }

func (*Scope) beginExplicitDispose() disposalGuard       { return disposalGuard{} }
func (disposalGuard) finish()                            {}
func (*Scope) noteCleanupAdded(int, string, string, any) {}
func (*Scope) noteImmediateCleanup(string, string)       {}
func (*Scope) noteCleanupRunning(int)                    {}
func (*Scope) releaseResourceCleanup(any)                {}
func (*Scope) noteTimersStopped(int)                     {}
func (*Scope) noteContextCancelled()                     {}
func (*runtime) labelScope(*Scope, string, string)       {}

func (*devState) initDisposals()                            {}
func (*devState) forgetDisposals(*Scope)                    {}
func (*devState) scopeLabelOf(*Scope) (string, string)      { return "", "" }
func (*devState) takeDisposals() []driver.Disposal          { return nil }
func (*devState) recentDisposals() []driver.Disposal        { return nil }
func (*devState) clearDisposals()                           {}
func (*devState) ownerResources(*Scope) []devtools.Resource { return nil }
