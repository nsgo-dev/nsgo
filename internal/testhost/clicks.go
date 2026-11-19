package testhost

import (
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Clicks counts nearby scripted presses for hosts whose input entry point takes the click count from the window system.
type Clicks struct {
	at     driver.Point
	target any
	time   time.Time
	count  int
}

// Press counts a press at at of target, the view the host finds there, within interval and four points of the last. A press of another view starts a new sequence, as headless steps count a press of the view pressed last: a page pushed by a press shows its own control where the press was, which takes the next press as a single click, not the second of a double click that the window system may still send to the view that took the first.
func (c *Clicks) Press(at driver.Point, target any, now time.Time, interval time.Duration) int {
	dx, dy := at.X-c.at.X, at.Y-c.at.Y
	if c.count > 0 && target == c.target && now.Sub(c.time) <= interval && dx >= -4 && dx <= 4 && dy >= -4 && dy <= 4 {
		c.count++
	} else {
		c.count = 1
	}
	c.at, c.target, c.time = at, target, now
	return c.count
}
