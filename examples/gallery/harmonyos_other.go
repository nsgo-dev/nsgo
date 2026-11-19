//go:build !nsgoharmony

package main

// harmonyOS is whether this is the HarmonyOS preview build (harmonyos.go).
const harmonyOS = false

// omitted are the demos this build leaves out: none.
var omitted map[string]bool
