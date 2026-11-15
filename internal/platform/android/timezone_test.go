package android

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"
)

// TestTimeZoneReadsTheRuntimesZone reads msgTimeZone as NsgoEngine.timeZone writes it: a zone the system's tzdata has keeps its rules, daylight saving included, and one Go cannot load keeps the offset Java gave.
func TestTimeZoneReadsTheRuntimesZone(t *testing.T) {
	message := func(id string, offset int32) *reader {
		var w writer
		w.str(id)
		w.u32(uint32(offset))
		return &reader{w.buf}
	}
	tokyo := readTimeZone(message("Asia/Tokyo", 9*3600))
	if got := time.Date(2026, 11, 15, 10, 32, 0, 0, time.UTC).In(tokyo).Format("15:04 MST"); got != "19:32 JST" {
		t.Errorf("10:32 UTC in Asia/Tokyo is %s, want 19:32 JST", got)
	}
	// New York's offset is the one of summer, which the zone's rules correct in winter.
	newYork := readTimeZone(message("America/New_York", -4*3600))
	if got := time.Date(2026, 1, 15, 17, 0, 0, 0, time.UTC).In(newYork).Format("15:04 MST"); got != "12:00 EST" {
		t.Errorf("17:00 UTC in America/New_York in January is %s, want 12:00 EST", got)
	}
	r := message("Nowhere/Unknown", -(5*3600 + 30*60))
	unknown := readTimeZone(r)
	if _, offset := time.Date(2026, 6, 1, 0, 0, 0, 0, unknown).Zone(); offset != -(5*3600+30*60) || unknown.String() != "Nowhere/Unknown" || len(r.buf) != 0 {
		t.Errorf("an unknown zone is %v at %d, %d bytes left", unknown, offset, len(r.buf))
	}
	if _, offset := time.Now().In(readTimeZone(message("", 3600))).Zone(); offset != 3600 {
		t.Errorf("a zone without an ID is at %d, want 3600", offset)
	}
}

// TestTimeZoneTakesAndroidsNewestRules loads zones from the first of Android's tzdata files that has them, as its libc does, past one that does not exist, and from Go's own sources for the rest.
func TestTimeZoneTakesAndroidsNewestRules(t *testing.T) {
	// fixed is the TZif data of a zone always offset seconds east of UTC, named name.
	fixed := func(name string, offset int32) []byte {
		b := append([]byte("TZif"), make([]byte, 16)...)
		for _, n := range []uint32{0, 0, 0, 0, 1, uint32(len(name) + 1)} {
			b = binary.BigEndian.AppendUint32(b, n)
		}
		b = binary.BigEndian.AppendUint32(b, uint32(offset))
		return append(append(b, 0, 0), name+"\x00"...)
	}
	// tzdata writes an Android tzdata file of zones as libcore's ZoneInfoDb does.
	tzdata := func(zones map[string][]byte, names ...string) string {
		var index, data []byte
		for _, name := range names {
			entry := append([]byte(name), make([]byte, 40-len(name))...)
			entry = binary.BigEndian.AppendUint32(entry, uint32(len(data)))
			entry = binary.BigEndian.AppendUint32(entry, uint32(len(zones[name])))
			index = append(index, binary.BigEndian.AppendUint32(entry, 0)...)
			data = append(data, zones[name]...)
		}
		b := []byte("tzdata2026e\x00")
		b = binary.BigEndian.AppendUint32(b, 24)
		b = binary.BigEndian.AppendUint32(b, uint32(24+len(index)))
		b = binary.BigEndian.AppendUint32(b, uint32(24+len(index)+len(data)))
		file := filepath.Join(t.TempDir(), "tzdata")
		if err := os.WriteFile(file, append(append(b, index...), data...), 0o644); err != nil {
			t.Fatal(err)
		}
		return file
	}
	// Rules no real zone has, so that only these files can give them.
	module := tzdata(map[string][]byte{"Asia/Tokyo": fixed("+05", 5*3600), "Europe/Paris": fixed("+07", 7*3600)}, "Europe/Paris", "Asia/Tokyo")
	update := tzdata(map[string][]byte{"Asia/Tokyo": fixed("+04", 4*3600), "Asia/Almaty": fixed("+03", 3*3600)}, "Asia/Tokyo", "Asia/Almaty")
	saved := tzdataFiles
	t.Cleanup(func() { tzdataFiles = saved })
	tzdataFiles = []string{filepath.Join(t.TempDir(), "missing"), module, update}
	at := time.Date(2026, 1, 15, 17, 0, 0, 0, time.UTC)
	for id, want := range map[string]string{"Asia/Tokyo": "22:00 +05", "Europe/Paris": "00:00 +07", "Asia/Almaty": "20:00 +03", "America/New_York": "12:00 EST"} {
		loc, err := loadZone(id)
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		if got := at.In(loc).Format("15:04 MST"); got != want || loc.String() != id {
			t.Errorf("17:00 UTC in %s is %s in %v, want %s", id, got, loc, want)
		}
	}
	if loc := zone("Asia/Tokyo", 9*3600); loc.String() != "Asia/Tokyo" || at.In(loc).Format("MST") != "+05" {
		t.Errorf("msgTimeZone's Asia/Tokyo is %v, %s, not the module's", loc, at.In(loc).Format("MST"))
	}
}

// TestTimeZoneMessageMatchesJava holds Protocol.java's TIME_ZONE to msgTimeZone, which android.go, built for Android alone, declares.
func TestTimeZoneMessageMatchesJava(t *testing.T) {
	java, err := os.ReadFile(filepath.Join("java", "dev", "oneo", "nsgo", "Protocol.java"))
	if err != nil {
		t.Fatal(err)
	}
	goSource, err := os.ReadFile("android.go")
	if err != nil {
		t.Fatal(err)
	}
	inJava := regexp.MustCompile(`static final int TIME_ZONE = (\d+);`).FindSubmatch(java)
	inGo := regexp.MustCompile(`(?m)^\s*msgTimeZone\s*=\s*(\d+)`).FindSubmatch(goSource)
	if inJava == nil || inGo == nil || string(inJava[1]) != string(inGo[1]) {
		t.Errorf("Protocol.java's TIME_ZONE is %q, android.go's msgTimeZone %q", inJava, inGo)
	}
}
