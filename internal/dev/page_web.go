//go:build nsgodev && js && wasm && !nsgominiprogram

package dev

import (
	"syscall/js"

	devlocales "github.com/nsgo-dev/nsgo/internal/dev/locales"
)

// pageOrigin is where the page running the app was served from: nsgo
// dev's server, for its builds.
func pageOrigin() string { return js.Global().Get("location").Get("origin").String() }

// showPageMessages gives the page the words of the build failures it shows, as nsgoDevMessages, which it reads when one comes.
func showPageMessages(msgs devlocales.Messages) {
	js.Global().Set("nsgoDevMessages", map[string]any{"buildFailed": msgs.BuildFailed(), "copy": msgs.Copy(), "reload": msgs.Reload()})
}
