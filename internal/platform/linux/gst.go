//go:build linux && !android

package linux

import (
	"sync"

	"github.com/ebitengine/purego"
)

// gst is GStreamer's registry, opened on the first video that declares alpha to ask whether a decoder that keeps it is installed. GTK's media backend links the same library and initializes it again harmlessly.
var gst struct {
	once        sync.Once
	ok          bool
	initCheck   func(argc, argv, gerr uintptr) int32
	factoryFind func(name *byte) uintptr
	objectUnref func(object uintptr)
}

// gstFeature reports whether GStreamer has the element factory name, false when GStreamer cannot be loaded.
func gstFeature(name string) bool {
	gst.once.Do(func() {
		for _, lib := range []string{"libgstreamer-1.0.so.0", "libgstreamer-1.0.so"} {
			h, err := purego.Dlopen(lib, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err != nil {
				continue
			}
			purego.RegisterLibFunc(&gst.initCheck, h, "gst_init_check")
			purego.RegisterLibFunc(&gst.factoryFind, h, "gst_element_factory_find")
			purego.RegisterLibFunc(&gst.objectUnref, h, "gst_object_unref")
			gst.ok = gst.initCheck(0, 0, 0) != 0
			return
		}
	})
	if !gst.ok {
		return false
	}
	f := gst.factoryFind(cstr(name))
	if f == 0 {
		return false
	}
	gst.objectUnref(f)
	return true
}
