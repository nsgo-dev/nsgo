### Fixed

- Show the app's icon and name in the iOS share extension and say the share was saved and to open the app, or why it failed, in the app's languages, then close the whole share sheet, which completing before the extension's view appeared had left open over an undismissable view; a failure cancels the request with its error and a provider that never answers times out after 30 seconds (#2668 by @houko).
