package driver

// TouchGeometry captures the presentation coordinates when touches arrive, before the engine handles them or their views move again. Positions follow the order of Event.Touches; Targets names each changed finger's initial view.
type TouchGeometry struct {
	Targets   map[int]NodeID
	Positions map[NodeID][]Point
}

// TouchTracker keeps the listening paths for a native touch sequence. The paths survive a finger lifting while another remains: the responder may still belong to that finger's path. No geometry is cached between reports.
type TouchTracker struct {
	views map[NodeID]struct{}
}

// Capture snapshots the native hit test and local positions for e. parent returns a view's native parent and event mask, and position converts a surface point using the view's current presentation, returning false for a view that has gone. Call on the platform thread before dispatching e.
func (t *TouchTracker) Capture(e *Event, hit func(Point) NodeID,
	parent func(NodeID) (NodeID, EventMask), position func(NodeID, Point) (Point, bool)) {
	g := &TouchGeometry{Positions: make(map[NodeID][]Point, len(t.views))}
	if e.Phase == GestureBegan {
		g.Targets = map[int]NodeID{}
		if t.views == nil {
			t.views = map[NodeID]struct{}{}
		}
		for _, p := range e.Touches {
			if !p.Changed {
				continue
			}
			id := hit(p.PagePosition)
			g.Targets[p.ID] = id
			for id != 0 {
				up, events := parent(id)
				if events.Has(EventTouch) {
					t.views[id] = struct{}{}
				}
				id = up
			}
		}
	}
	for id := range t.views {
		points := make([]Point, 0, len(e.Touches))
		for _, p := range e.Touches {
			local, ok := position(id, p.PagePosition)
			if !ok {
				break
			}
			points = append(points, local)
		}
		if len(points) == len(e.Touches) {
			g.Positions[id] = points
		}
	}
	e.TouchGeometry = g
	if e.Phase == GestureEnded || e.Phase == GestureCancelled {
		down := false
		for _, p := range e.Touches {
			down = down || !p.Changed
		}
		if !down {
			t.views = nil
		}
	}
}
