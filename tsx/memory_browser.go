//go:build js

package tsx

// memory is nothing on the web, where Limits.Memory does not hold: the page's engine keeps its own heap.
type memory struct{}

// limitMemory does nothing on the web: the page's engine keeps its own heap, which Go cannot count.
func (r *runtime) limitMemory(int64) {}
