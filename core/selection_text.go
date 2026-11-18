package core

// TextSelection is the selection of a Selectable view, as OnTextSelection receives it.
type TextSelection struct {
	// Text is the selected text as a copy gives it: texts on one row joined by a tab and the others by a line break.
	Text string
	// Bounds is the box of the texts the selection touches, in the view: no Size without a selection. Ref.Bounds of the view gives the surface's.
	Bounds Bounds
}
