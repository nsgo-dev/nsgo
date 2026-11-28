### Fixed

- Link the iOS share extension with `_NSExtensionMain` as its entry point, which App Store Connect requires (error 90898), and fail `nsgo build` when it is not (#2718 by @houko).
