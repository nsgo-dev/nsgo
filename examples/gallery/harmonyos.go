//go:build nsgoharmony

package main

// harmonyOS is whether this is the HarmonyOS preview build, which reads as Android but whose render layer draws a first set of ArkUI views only (docs/architecture/harmonyos.md): it has no system tab bar (https://github.com/nsgo-dev/nsgo/issues/807), so the gallery draws its own.
const harmonyOS = true

// omitted are the demos this build leaves out, as the render layer cannot draw what they show yet: a Shader view has no ArkUI counterpart (the roadmap's HarmonyOS Platform parity row), and DataTable's rows scroll inside views that clip with Overflow: Hidden, which the render layer does not apply (https://github.com/nsgo-dev/nsgo/issues/808), so they draw over the rest of the page.
var omitted = map[string]bool{"Shader": true, "DataTable": true}
