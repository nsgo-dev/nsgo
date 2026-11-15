//go:build darwin

package apple

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// On iOS, tvOS and watchOS, where Go leaves it UTC, time.Local starts as the zone NSTimeZone says.
func TestTimeLocalStartsAsTheSystemsZone(t *testing.T) {
	if runtime.GOOS != "ios" {
		t.Skip("Go reads the zone itself on macOS")
	}
	withPool(func() {
		tz := objc.ID(Class("NSTimeZone")).Send(sel("localTimeZone"))
		if name := GoString(tz.Send(sel("name"))); time.Local.String() != name {
			t.Errorf("time.Local is %q, NSTimeZone's zone %q", time.Local, name)
		}
		if _, offset := time.Now().Zone(); offset != objc.Send[int](tz, sel("secondsFromGMT")) {
			t.Errorf("time.Local is %d seconds east of UTC, NSTimeZone's zone %d", offset, objc.Send[int](tz, sel("secondsFromGMT")))
		}
	})
}

// A change of the system's zone makes time.Local the new zone, with NSTimeZone's rules for its past and future offsets. The test changes the zone as NSTimeZone reads it, through the process's TZ.
func TestTimeLocalFollowsAChangeOfZone(t *testing.T) {
	libc := load("/usr/lib/libSystem.B.dylib")
	var setenv func(name, value string, overwrite int32) int32
	var unsetenv func(name string) int32
	purego.RegisterLibFunc(&setenv, libc, "setenv")
	purego.RegisterLibFunc(&unsetenv, libc, "unsetenv")
	nsTimeZone := objc.ID(Class("NSTimeZone"))
	center := objc.ID(Class("NSNotificationCenter")).Send(sel("defaultCenter"))
	changed := Global(foundationPath, "NSSystemTimeZoneDidChangeNotification")
	local := time.Local
	tz, hadTZ := os.LookupEnv("TZ")
	t.Cleanup(func() {
		if hadTZ {
			setenv("TZ", tz, 1)
		} else {
			unsetenv("TZ")
		}
		withPool(func() { nsTimeZone.Send(sel("resetSystemTimeZone")) })
		time.Local = local
	})
	instants := []time.Time{
		time.Date(1990, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		time.Date(2024, 7, 15, 12, 0, 0, 0, time.UTC),
		time.Date(2030, 3, 31, 2, 30, 0, 0, time.UTC),
	}
	for _, name := range []string{"America/Nuuk", "Asia/Kathmandu", "Australia/Lord_Howe", "Europe/Dublin"} {
		withPool(func() {
			setenv("TZ", name, 1)
			nsTimeZone.Send(sel("resetSystemTimeZone"))
			center.Send(sel("postNotificationName:object:"), changed, objc.ID(0))
			if time.Local.String() != name {
				t.Fatalf("after a change to %s, time.Local is %q", name, time.Local)
			}
			zone := nsTimeZone.Send(sel("localTimeZone"))
			for _, at := range instants {
				date := objc.ID(Class("NSDate")).Send(sel("dateWithTimeIntervalSince1970:"), float64(at.Unix()))
				want := objc.Send[int](zone, sel("secondsFromGMTForDate:"), date)
				if _, got := at.In(time.Local).Zone(); got != want {
					t.Errorf("%s at %v: time.Local is %d seconds east of UTC, NSTimeZone %d", name, at, got, want)
				}
			}
		})
	}
}

// A zone Go cannot read is the zone's offset now, under its name.
func TestZoneFallsBackToItsOffset(t *testing.T) {
	for _, c := range []struct {
		name   string
		data   []byte
		offset int
	}{
		{"Nowhere/Atlantis", nil, 5400},
		{"Nowhere/Atlantis", []byte("not TZif"), -12600},
		{"", nil, 3600},
	} {
		loc := zone(c.name, c.data, c.offset)
		name, offset := time.Now().In(loc).Zone()
		if loc.String() != c.name || offset != c.offset {
			t.Errorf("zone(%q, %q, %d) is %q, %d seconds east of UTC (%s)", c.name, c.data, c.offset, loc, offset, name)
		}
	}
}
