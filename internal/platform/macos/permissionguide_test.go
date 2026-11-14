//go:build darwin && !ios

package macos

import (
	"testing"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

func TestGuideEndsOnceAndUnansweredAsItsSurfaceCloses(t *testing.T) {
	var answers []driver.Event
	newGuide := func(s *surface, id uint32) *permissionGuide {
		return &permissionGuide{s: s, id: id, dispatch: func(ev driver.Event) { answers = append(answers, ev) }, stop: make(chan struct{})}
	}
	ended := func(g *permissionGuide) bool {
		select {
		case <-g.stop:
			return true
		default:
			return false
		}
	}
	s, other := &surface{}, &surface{}

	// A new guide ends the one before, which still answers, once.
	first, second := newGuide(s, 1), newGuide(s, 2)
	s.adoptGuide(first)
	s.adoptGuide(second)
	if !ended(first) || ended(second) {
		t.Fatalf("first ended %v, second ended %v", ended(first), ended(second))
	}
	first.finish(driver.PermissionDenied)
	if len(answers) != 1 || answers[0].Count != 1 || answers[0].Index != driver.PermissionDenied || answers[0].Kind != driver.EventPermission {
		t.Errorf("answers %+v", answers)
	}

	// StopGuide ends only its own surface's guide of its number.
	stopGuide(other, 2)
	stopGuide(s, 1)
	if ended(second) {
		t.Error("another surface's or number's stop ended the guide")
	}
	stopGuide(s, 2)
	if !ended(second) {
		t.Error("the guide's stop did not end it")
	}
	second.finish(driver.PermissionGranted)
	if len(answers) != 2 || answers[1].Count != 2 || answers[1].Index != driver.PermissionGranted {
		t.Errorf("answers %+v", answers)
	}

	// The surface keeps one stop for all its guides and nothing of those that answered.
	third := newGuide(s, 3)
	s.adoptGuide(third)
	if len(s.stops) != 1 {
		t.Errorf("the surface keeps %d stops for its guides", len(s.stops))
	}
	if live := liveGuides[s]; len(live) != 1 {
		t.Errorf("the surface keeps %d guides", len(live))
	}

	// Its surface closing ends it unanswered.
	for _, stop := range s.stops {
		stop()
	}
	if !ended(third) {
		t.Fatal("closing the surface did not end its guide")
	}
	third.finish(driver.PermissionDenied)
	if len(answers) != 2 {
		t.Errorf("a closed surface's guide answered: %+v", answers[2:])
	}
	if currentGuide != nil {
		t.Error("a guide is still running")
	}
	if _, ok := liveGuides[s]; ok {
		t.Error("a closed surface's guides are kept")
	}
}
