package android

import (
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
