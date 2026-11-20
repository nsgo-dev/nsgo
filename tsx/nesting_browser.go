//go:build js

package tsx

// limitNesting does nothing on the web, whose engine's built-ins throw a RangeError of their own past its stack.
func (r *runtime) limitNesting() {}
