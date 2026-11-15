//go:build nsgodev && !(js && wasm && !nsgominiprogram)

package dev

import devlocales "github.com/nsgo-dev/nsgo/internal/dev/locales"

// pageOrigin is where a web page was served from: no page runs the app.
func pageOrigin() string { return "" }

// showPageMessages does nothing: no page shows build failures.
func showPageMessages(devlocales.Messages) {}
