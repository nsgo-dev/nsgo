//go:build (!nsgodev && !nsgotest && !nsgocheck) || (js && wasm)

package core

// Release and WebAssembly builds inline these empty helpers away, including Signal.Set's. Browser callbacks can enter the serialized graph on another managed goroutine, so their goroutine IDs cannot establish ownership.
type graphOwner struct{}

func newGraphOwner(bool) graphOwner { return graphOwner{} }
func (*runtime) check()             {}
