package apk

import "testing"

// Exercise ZIP's 32-bit size boundary without allocating a 4 GiB slice, including compressed data that crosses it while the source does not.
func TestEntrySizeLimit(t *testing.T) {
	for _, tc := range []struct {
		name             string
		size, compressed uint64
		refused          bool
	}{
		{"empty", 0, 0, false},
		{"largest ZIP32 sizes", 0xfffffffe, 0xfffffffe, false},
		{"ZIP64 source marker", 0xffffffff, 1, true},
		{"ZIP64 compressed marker", 1, 0xffffffff, true},
		{"source beyond uint32", 1 << 32, 1, true},
		{"compressed beyond uint32", 1, 1 << 32, true},
		{"no uint64 wrap", ^uint64(0), ^uint64(0), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkEntrySize("classes.dex", tc.size, tc.compressed)
			if (err != nil) != tc.refused {
				t.Fatalf("size %d, compressed %d: %v, want refused %v", tc.size, tc.compressed, err, tc.refused)
			}
			if err != nil && err.Error() != "classes.dex: an APK holds files below 4 GiB" {
				t.Fatalf("entry error = %v", err)
			}
		})
	}
}

// Entries that each fit can still push a later local header or the central directory past a 32-bit offset; that archive must be refused, not wrapped.
func TestArchiveSizeLimit(t *testing.T) {
	for _, tc := range []struct {
		dir     uint64
		refused bool
	}{
		{0, false},
		{0xfffffffe, false},
		{0xffffffff, true},
		{1 << 32, true},
		{^uint64(0), true},
	} {
		err := checkArchiveSize(tc.dir)
		if (err != nil) != tc.refused {
			t.Fatalf("directory at %d: %v, want refused %v", tc.dir, err, tc.refused)
		}
		if err != nil && err.Error() != "an APK holds at most 4 GiB" {
			t.Fatalf("archive error = %v", err)
		}
	}
}
