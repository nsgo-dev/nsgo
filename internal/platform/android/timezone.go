package android

import "time"

// Go on Android leaves time.Local UTC whatever TZ says (time's zoneinfo_android.go), as the device's zone is a system property Java reads. The runtime passes TimeZone.getDefault's ID as TZ, which init in android.go loads before the app's main runs, and sends msgTimeZone after starting the process and whenever ACTION_TIMEZONE_CHANGED arrives, which the reader makes time.Local. Times read after a change are in the new zone; the app is not rendered again for it.

// zone returns the location of the zone id, which is offset seconds east of UTC now: the zone from the system's tzdata, or a fixed zone of that offset when Go cannot load it, as on a system that keeps its tzdata only where Go does not look.
func zone(id string, offset int) *time.Location {
	if id == "" {
		return time.FixedZone("", offset)
	}
	if loc, err := time.LoadLocation(id); err == nil {
		return loc
	}
	return time.FixedZone(id, offset)
}

// readTimeZone reads msgTimeZone's zone as NsgoEngine.timeZone writes it: its ID, then its offset from UTC now in seconds.
func readTimeZone(r *reader) *time.Location {
	id := r.str()
	return zone(id, int(int32(r.u32())))
}
