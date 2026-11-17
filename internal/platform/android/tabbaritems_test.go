package android

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestTabBarItemsEncodeCanvasIcons(t *testing.T) {
	svg, err := driver.ParseSVG([]byte(`<svg viewBox="0 0 48 24"><path d="M0 12H48" stroke="currentColor" stroke-width="2"/></svg>`))
	if err != nil {
		t.Fatal(err)
	}
	d := &driver.Drawing{}
	svg.Draw(d, driver.Size{Width: 24, Height: 24}, driver.Color(0xffffffff))
	source := map[string]any{"title": "Home", "icon": "fallback", "drawing": d}
	data, err := tabBarItems([]any{source, map[string]any{"title": "Search", "icon": "android:ic_menu_search"}})
	if err != nil {
		t.Fatal(err)
	}
	var tabs []map[string]string
	if err := json.Unmarshal(data, &tabs); err != nil {
		t.Fatal(err)
	}
	ops, err := base64.StdEncoding.DecodeString(tabs[0]["drawing"])
	if err != nil {
		t.Fatal(err)
	}
	w := &writer{}
	writeDrawing(w, d)
	n := binary.BigEndian.Uint32(w.buf)
	if len(ops) == 0 || !bytes.Equal(ops, w.buf[4:4+n]) {
		t.Fatal("the native icon did not receive the canvas player's operations")
	}
	if source["drawing"] != d || tabs[0]["icon"] != "fallback" || tabs[1]["icon"] != "android:ic_menu_search" || tabs[1]["drawing"] != "" {
		t.Fatal("encoding changed the source or resource icons")
	}
}
