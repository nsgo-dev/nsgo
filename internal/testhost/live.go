package testhost

var live func(surface uint32) Surface

// RegisterLive gives `nsgo dev -automation`'s agent the running app's surfaces: find returns the backend's Surface for the surface numbered surface, to act on with native input, or nil where none shows; a backend's init in the builds with the nsgoautomation tag calls it.
func RegisterLive(find func(surface uint32) Surface) { live = find }

// Live returns the backend's Surface for the running app's surface, nil where none shows or no backend registered one.
func Live(surface uint32) Surface {
	if live == nil {
		return nil
	}
	return live(surface)
}
