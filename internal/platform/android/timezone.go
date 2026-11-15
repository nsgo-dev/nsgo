package android

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"time"
)

// Go on Android leaves time.Local UTC whatever TZ says (time's zoneinfo_android.go), as the device's zone is a system property Java reads. The runtime passes TimeZone.getDefault's ID as TZ, which init in android.go loads before the app's main runs, and sends msgTimeZone after starting the process and whenever ACTION_TIMEZONE_CHANGED arrives, which the reader makes time.Local. Times read after a change are in the new zone; the app is not rendered again for it.

// tzdataFiles are where Android keeps the zones' rules, in the order its libc reads them: the time zone module's, which updates apart from the system, and the updates Android 9 installs. time.LoadLocation reads the system image's copy first, as old as the device's build: a Pixel 3 XL's on Android 12 has 2021a's rules, which put Asia/Almaty, Asia/Amman and America/Nuuk an hour from the device's clock.
var tzdataFiles = []string{"/apex/com.android.tzdata/etc/tz/tzdata", "/data/misc/zoneinfo/current/tzdata"}

// loadZone loads the zone id with the rules Android itself uses: from the first of tzdataFiles that has it, else time.LoadLocation's.
func loadZone(id string) (*time.Location, error) {
	for _, file := range tzdataFiles {
		if data, err := tzdataZone(file, id); err == nil {
			if loc, err := time.LoadLocationFromTZData(id, data); err == nil {
				return loc, nil
			}
		}
	}
	return time.LoadLocation(id)
}

// tzdataZone returns the TZif data of the zone id in an Android tzdata file, as libcore's ZoneInfoDb writes it: a 24-byte header, "tzdata" and the version, then the offsets of the index and of the data; the index, an entry of 52 bytes per zone, its name padded with NULs to 40 bytes, then its data's offset from the data's start and its length; then the data.
func tzdataZone(file, id string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	corrupt := errors.New("android: corrupt tzdata " + file)
	var header [24]byte
	if _, err := f.ReadAt(header[:], 0); err != nil || string(header[:6]) != "tzdata" {
		return nil, corrupt
	}
	index, data := int64(binary.BigEndian.Uint32(header[12:])), int64(binary.BigEndian.Uint32(header[16:]))
	if index < int64(len(header)) || data < index || data > info.Size() {
		return nil, corrupt
	}
	entries := make([]byte, data-index)
	if _, err := f.ReadAt(entries, index); err != nil {
		return nil, corrupt
	}
	for e := entries; len(e) >= 52; e = e[52:] {
		if name, _, _ := bytes.Cut(e[:40], []byte{0}); string(name) != id {
			continue
		}
		start, size := data+int64(binary.BigEndian.Uint32(e[40:])), int64(binary.BigEndian.Uint32(e[44:]))
		if start+size > info.Size() {
			return nil, corrupt
		}
		zone := make([]byte, size)
		if _, err := f.ReadAt(zone, start); err != nil {
			return nil, corrupt
		}
		return zone, nil
	}
	return nil, fs.ErrNotExist
}

// zone returns the location of the zone id, which is offset seconds east of UTC now: the zone as Android's tzdata has it (loadZone), or a fixed zone of that offset when Go cannot load it.
func zone(id string, offset int) *time.Location {
	if id == "" {
		return time.FixedZone("", offset)
	}
	if loc, err := loadZone(id); err == nil {
		return loc
	}
	return time.FixedZone(id, offset)
}

// readTimeZone reads msgTimeZone's zone as NsgoEngine.timeZone writes it: its ID, then its offset from UTC now in seconds.
func readTimeZone(r *reader) *time.Location {
	id := r.str()
	return zone(id, int(int32(r.u32())))
}
