package tsx

import (
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/tsx/internal/js"
)

func (r *runtime) labelRef(v js.Value) *core.Ref {
	if n := r.nodeOf(v); n != nil {
		return n.ref
	}
	return nil
}

func (r *runtime) labelRefs(v js.Value) []*core.Ref {
	var refs []*core.Ref
	for _, item := range r.arrayItems(v) {
		if ref := r.labelRef(item); ref != nil {
			refs = append(refs, ref)
		}
	}
	return refs
}
