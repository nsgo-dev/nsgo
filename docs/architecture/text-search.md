# Text-level search

This is the shared design for issue [#47](https://github.com/nsgo-dev/nsgo/issues/47). `base/textview`, core's box reveal and `ui/chat` implement it. The matching rules and highlights belong to `base/textview`, so another document viewer can use them without importing `ui/chat`. Where it follows one of its [decisions](#decisions), confirmed by the user on 2026-11-15, the decision's number marks it.

## What a view searches

A search matches the text a `TextView` shows, which depends on the view as much as on its document: with `RenderMath` a formula is an attachment where a plain view shows its TeX, `CustomSpan` shows an inline custom node as a span of the app's, and a footnote reference shows "[n]". So matching runs in the renderer, over each `core.Text` it builds, rather than over a `Document` alone (decision 3):

```go
type FindOptions struct {
	CaseSensitive bool
	WholeWord     bool
}

// TextRef names a text a view shows.
type TextRef struct {
	Footnote int   // the footnote's index, -1 outside footnotes
	Path     []int // the text's place in the blocks, outermost first
}

type Match struct {
	Text     TextRef
	From, To int // UTF-16 offsets in that text, as core's text ranges take them
}
```

A view searches the texts it builds: paragraphs, headings, code blocks' code, table cells, a details' summary, and a formula's TeX, a custom node's content and an inline custom node's `CustomSpan` where the view shows them as text. What the app shows itself is not searched, since `base` can neither split nor measure it: attachments (images, `RenderMath`'s formulas, `RenderCustom`'s and plugins' nodes), `RenderCodeBlock`'s and `RenderImage`'s nodes, `RenderTableActions`, code block actions, list and footnote markers, the disclosure marker and image alt text (decision 7). HTML's hidden elements never reach the document, as the reader drops them, and a closed details' children are not built, so they are searched once it opens (decision 8). A match never crosses a text, so a query containing a line break does not join two paragraphs (decision 1). Regex is deliberately outside the first contract; the options can grow later without making plain search ambiguous (decision 4).

`Path` steps through the blocks as they nest: a block's index among its siblings, then inside a list the item's index and the block's in it, inside a table the row's (0 the head, the body's rows from 1) and the cell's, and inside a details -1 for its summary or a child block's index. Paths order one view's texts, a footnote's after the blocks, and are stable for a document revision; a view finds its matches again when its document changes rather than moving old offsets. A view caps its work and its matches so an untrusted transcript cannot allocate without bound, and an empty query matches nothing.

## Highlights

The renderer splits spans at match boundaries and adds `PartSearch` to every match and `PartSearchSelected` to the selected one. `base` supplies no colors: the app's `Styles` or `StyleOf` chooses them, and code-block highlighting merges with search parts.

Search highlights are not a text selection. A selectable view keeps its selection and copy behavior, while a search can mark a different range.

## One search across views

```go
func NewSearch(cx *core.Scope) *Search
func ProvideSearch(cx *core.Scope, s *Search)
func (s *Search) Open()
func (s *Search) Close()
func (s *Search) IsOpen() bool
func (s *Search) SetQuery(string)
func (s *Search) Query() string
func (s *Search) SetOptions(FindOptions)
func (s *Search) Count() int
func (s *Search) Selected() int // one-based, zero when there is none
func (s *Search) SelectedMatch() (Match, bool)
func (s *Search) Next()
func (s *Search) Previous()
func (s *Search) SelectedBounds() (core.Bounds, bool)
func (s *Search) SetRevealMargin(core.Edges)
```

A `TextView` joins the search in its `Search` field, else the one `ProvideSearch` gives every `TextView` below cx's owner, as `Provide` gives plugins (decision 2). A view registers itself while it is mounted, with no key: it replaces its matches when its document or the query changes, and they go when it unmounts, so none go stale. The search orders views by tree order (`Ref.Before` on each view's root), not by when they registered, so a view mounted late, such as one in a disclosure just opened, takes its place in reading order; within a view, matches follow their texts' paths and offsets. `Next` and `Previous` wrap. When the matches change, the selected match stays selected while it exists, else the selection moves to the next match after it.

`Open` and `Close` are the app's too, so a toolbar button can open the search on a phone with no keyboard. While the search is closed, views show no search parts and `Count` is zero; the query and options stay, so opening it again finds the same matches.

`SelectedBounds` is the selected match's first box in surface points, from its text's `Ref.TextRangeBoxes` rather than the block's frame, and false while that text is not laid out. Revealing the match is the view's (decision 6): selecting a match reveals it through every scroll view around its text, innermost first, so a code block's or a table's horizontal scroll reaches the match's column rather than only the block showing. `Ref.Reveal` reveals a whole element and `Ref.ScrollTo` takes an offset, so core gains a reveal of a box within an element, with a margin kept clear at the outermost scroll view's edges for the bars that cover it, which the search's owner sets with `SetRevealMargin`.

## In `base/textview`

`find.go` matches a text with KMP over its characters, compared by Unicode's simple case folding unless `CaseSensitive`; `WholeWord` takes letters, numbers, marks and the underscore as a word's characters. A view and each of its texts keep at most 1,000 matches (`maxMatches`): a text stops finding there, the view drops what lies past it and highlights only what it counts. `search.go` keeps a view's texts in their places' order, each text's matches in a memo of its own, and the views' in tree order, sorted again whenever a view's matches or the views change. A text in a search is a `SpansFunc` text with a `Ref`, so its view stays as its matches change. A search starts closed with no query. The selected match is kept by its view, text and range, so a text built again, as a streaming table's are, keeps it; where its view is gone, the match at its index is selected. A newly selected match, by `Next` and `Previous` or because the matches moved it, is revealed once its text is laid out, nearest on both axes and without animation.

## Chat integration

`ui/chat` supplies a find bar with a field, count, previous/next and close actions. `Conversation` and `FloatingConversation` provide one search state to the text views for their turns, open it from Mod+F (Ctrl+F elsewhere) and focus the field; Escape closes it. Closing gives the focus back to where it was when the bar opened, unless it already left the bar, as a popover gives it back to its trigger ([components](components.md)); `core.FocusedRef` records that element for the overlay. Each sets the reveal margin to the find bar's height, and `FloatingConversation` adds its `TopInset` and its composer's height at the bottom, so a match lands clear of both. Revealing a match stops a following transcript from following (`ConversationState`'s `pause`), so a streaming reply does not pull it back to the end. The bar's words, through `ui/chat/locales` in ICU MessageFormat ([languages](languages.md)), are “Find”, “Previous”, “Next”, “Close”, “No results” and “{selected, number} of {count, number}”. It does not implement matching, span splitting or cross-view counting.

A completed turn folds its steps and a completed reasoning block its detail, building nothing hidden, so their text views do not exist to register. While the search is open with a query, a turn and a reasoning block that share the transcript's `ConversationState` unfold without recording a choice of the user's, and fold again by their own rules when it closes (decision 5); the transcript does not virtualize, so this mounts no more than a transcript the user unfolded. A reasoning block's default body is a one-paragraph `TextView` of its text, which it does not read as Markdown, so it is searched. Only text views register: what a step or a prompt shows with other components is not searched.

## Verification

The implementation is complete only when these checks exist:

- `base/textview` tests cover a three-block document, case and whole-word options, a match at an inline span boundary, code and math TeX, a table cell, a details' summary, a closed details opening, paths into lists, tables, details and footnotes, a `CustomSpan` and a `RenderMath` attachment, UTF-16 offsets past a character outside the BMP, and the result cap;
- `TextView` tests cover search parts, selected parts, syntax-highlight merging, stale registrations and selection remaining independent;
- shared-search tests cover tree order with a view mounted late, replacement, removal, opening and closing, next/previous wrapping, selected bounds across three views and revealing a match in a code block's horizontal scroll; and
- `ui/chat` tests type a query over three turns, one match in a folded turn's step and one in folded reasoning, show the count, navigate and reveal the selected match clear of the bar, then close with Escape, the focus back where it was and the transcript selection unchanged.

## Decisions

The user confirmed these on 2026-11-15, each the first answer of the question it settled:

1. Matches stay within one text; no document-level plain-text view joins blocks.
2. `ProvideSearch` includes every descendant `TextView`, and a view's `Search` field joins it to another search instead.
3. A view matches each text synchronously as it builds it, with no asynchronous index.
4. Regular expressions and accent- or Unicode-insensitive matching come later as options, not as a separate package.
5. An open search unfolds a transcript's turns and reasoning rather than leaving folded content out.
6. `TextView` reveals its own match through core's box reveal; the transcript does not scroll to `SelectedBounds` itself.
7. What the app renders itself (`RenderMath`, `RenderCustom`, `RenderCodeBlock`, plugins) stays out of the search.
8. A closed details' children are searched once it opens, not while it is closed.
