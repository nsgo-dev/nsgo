//go:build !nsgodev && !nsgotest && !nsgocheck

package core

// Release builds inline these empty helpers away, including Signal.Set's.
type graphOwner struct{}

func newGraphOwner(bool) graphOwner { return graphOwner{} }
func (*runtime) check()             {}
