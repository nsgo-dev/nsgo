//go:build darwin

package apple

import (
	"runtime"
	"time"

	"github.com/ebitengine/purego/objc"

	"github.com/nsgo-dev/nsgo/internal/platform/objcblock"
)

// Go reads the local zone from /etc/localtime on macOS, once, but leaves time.Local UTC on iOS, tvOS and watchOS, which Go builds as GOOS=ios (time's zoneinfo_ios.go), where time.LoadLocation reads no system files either: it finds a zone there only in a time/tzdata the app links. loadTimeZone makes time.Local the zone NSTimeZone says, from the zone's own TZif data, before the app's main runs on those three, and again on every Apple target whenever NSSystemTimeZoneDidChangeNotification arrives. Times read after a change are in the new zone; the app is not rendered again for it.

const foundationPath = "/System/Library/Frameworks/Foundation.framework/Foundation"

func loadTimeZone() {
	load(foundationPath)
	withPool(func() {
		if runtime.GOOS == "ios" {
			time.Local = localZone()
		}
		// The block runs on the thread that posts the notification, so a change applies whether or not the main thread runs a run loop, as a test's does not.
		block := objcblock.New(func(objc.Block, objc.ID) {
			// Go has no other way to change the zone: the store races with readers of time.Local, but it is one pointer, which none sees torn.
			time.Local = localZone()
		})
		center := objc.ID(Class("NSNotificationCenter")).Send(sel("defaultCenter"))
		center.Send(selAddObserver, Global(foundationPath, "NSSystemTimeZoneDidChangeNotification"), objc.ID(0), objc.ID(0), block).Send(sel("retain"))
		block.Release()
	})
}

// localZone returns NSTimeZone's local zone, which follows the system's. The local zone's data stays that of the zone it first had (macOS 27), so the data comes from the zone of its name.
func localZone() *time.Location {
	nsTimeZone := objc.ID(Class("NSTimeZone"))
	tz := nsTimeZone.Send(sel("localTimeZone"))
	name := tz.Send(sel("name"))
	if named := nsTimeZone.Send(sel("timeZoneWithName:"), name); named != 0 {
		tz = named
	}
	return zone(GoString(name), Bytes(tz.Send(sel("data"))), objc.Send[int](tz, sel("secondsFromGMT")))
}

// zone returns the location of the zone name, whose TZif data is data and which is offset seconds east of UTC now: the zone data describes, else the one time.LoadLocation finds, else a fixed zone of that offset.
func zone(name string, data []byte, offset int) *time.Location {
	if name == "" {
		return time.FixedZone("", offset)
	}
	if loc, err := time.LoadLocationFromTZData(name, data); err == nil {
		return loc
	}
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	return time.FixedZone(name, offset)
}
