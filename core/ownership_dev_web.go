//go:build nsgodev && js && wasm

package core

// Web callbacks can resume a Go callback on a browser-managed goroutine even though the WebAssembly runtime is single-threaded. The engine still serializes the graph; the nsgotest build checks ownership on native hosts.
const developmentOwnership = false
