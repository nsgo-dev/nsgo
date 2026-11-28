### Fixed

- Land an animated `Ref.ScrollToIndexAt` on its row when the rows on the way are measured, even hundreds of rows past the measured ones, instead of stopping short of it with the list still pinned (#2719 by @houko).
