package core

import "github.com/nsgo-dev/nsgo/internal/driver"

// CloudStore, CloudChangeReason, CloudChangeEvent, CloudConflict and
// CloudConflictError are core's part of package cloud, which apps use:
// hosts (such as nsgotest) pass an isolated CloudStore in RootOptions.Cloud
// and package cloud reads it through Scope.CloudStore.
type (
	CloudStore         = driver.CloudStore
	CloudChangeReason  = driver.CloudChangeReason
	CloudChangeEvent   = driver.CloudChangeEvent
	CloudConflict      = driver.CloudConflict
	CloudConflictError = driver.CloudConflictError
)

// CloudChangeServer and the other reasons are core's part of package cloud's ChangeReason.
const (
	CloudChangeServer         = driver.CloudChangeServer
	CloudChangeInitialSync    = driver.CloudChangeInitialSync
	CloudChangeQuotaViolation = driver.CloudChangeQuotaViolation
	CloudChangeAccount        = driver.CloudChangeAccount
)

// CloudStore returns the per-app cloud storage backend passed in RootOptions.Cloud,
// or nil when the app uses the platform backend. It is core's part of package cloud,
// which apps use.
func (cx *Scope) CloudStore() CloudStore { return cx.rt.cloud }
