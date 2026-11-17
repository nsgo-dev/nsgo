// Package moduletest holds the tests that check the whole module rather than one package: its import layers (layering_test.go), what apps link (linked_test.go), the public API's types (api_test.go), that base reads no theme (theme_test.go), that Objective-C blocks go through internal/platform/objcblock (blocks_test.go) and the skills it gives coding agents (skills_test.go). It has no code of its own.
package moduletest

// moduleRoot is the module's root, relative to this package's directory, where the tests run.
const moduleRoot = "../.."
