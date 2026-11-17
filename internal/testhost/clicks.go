package testhost

import (
	"time"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Clicks counts nearby scripted presses for hosts whose input entry point takes the click count from the window system.
type Clicks struct {
	at    driver.Point
	time  time.Time
	count int
}

// Press counts a press at at, within interval and four points of the last.
func (c *Clicks) Press(at driver.Point, now time.Time, interval time.Duration) int {
	dx, dy := at.X-c.at.X, at.Y-c.at.Y
	if c.count > 0 && now.Sub(c.time) <= interval && dx >= -4 && dx <= 4 && dy >= -4 && dy <= 4 {
		c.count++
	} else {
		c.count = 1
	}
	c.at, c.time = at, now
	return c.count
}
