package driver

// FocusStop reports whether a non-input view takes keyboard focus.
// Platforms also require it to be mounted, visible and in the active modal.
func (p *ViewProps) FocusStop(events EventMask) bool {
	return !p.Disabled && !p.NoFocus && (p.Focusable || events.Has(EventPress) || events.Has(EventLongPress) || events.Has(EventPressDown) || events.Has(EventPressUp))
}

// FocusesViews reports whether the platform supports keyboard focus on
// ordinary views beside text inputs. Mobile touch platforms focus inputs.
func (e *Environment) FocusesViews() bool {
	return e.OS == OSMacOS || e.OS == OSWindows || e.OS == OSLinux || e.OS == OSWeb || e.OS == OSIOS && e.Idiom == IdiomTV
}
