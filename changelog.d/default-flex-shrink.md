### Breaking

- `core.Style.FlexShrink` is a `core.FlexShrink` instead of a `float32`, and a box that grows, `core.Scroll`, `core.List` and the boxes around them now shrink by default: write `FlexShrink: core.NoShrink` for `FlexShrink: 0` where such a box must keep its size, `FlexShrink: core.Shrink(n)` for any other factor, and drop `FlexShrink: 1` beside `FlexGrow: 1`.

### Changed

- Shrink a box that grows by default, as well as `core.Scroll`, `core.List` and the boxes around them that set no size on their scroll axis, so a list or scroll view scrolls however deep it sits, and make `Style.FlexShrink` a `core.FlexShrink` set with `core.Shrink(n)` or `core.NoShrink`.
