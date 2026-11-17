package android

import (
	"encoding/json"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// tabBarItems encodes the tab bar's items as JSON for TabBar.java, each SVG drawing as the operations CanvasNode plays, which json writes in base64; the runtime rasterizes them once at the display's density and tints the result as it does a drawable.
func tabBarItems(value any) ([]byte, error) {
	items, ok := value.([]any)
	if !ok {
		return json.Marshal(value)
	}
	list := make([]any, len(items))
	for i, item := range items {
		props, ok := item.(map[string]any)
		if !ok {
			list[i] = item
			continue
		}
		tab := make(map[string]any, len(props))
		for name, v := range props {
			if d, ok := v.(*driver.Drawing); name == "drawing" && ok {
				w := &writer{}
				writeDrawing(w, d)
				// The first word is the operations' byte count; the slots and accessibility nodes that follow mean nothing to a static icon.
				r := &reader{buf: w.buf}
				n := r.u32()
				v = w.buf[4 : 4+int(n)]
			}
			tab[name] = v
		}
		list[i] = tab
	}
	return json.Marshal(list)
}
