package nsgo

import (
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/core"
)

// parseDevLayout reads devLayout, which nsgo dev writes from -layout once it has checked it: parts joined by commas, bounds for every view's boxes, grid for a grid of 8 points and grid=N for one of N. Only the dev client (nsgodev) calls it, so release builds link none of it; it is untagged so that nsgo test, which builds no dev client, checks it.
func parseDevLayout(value string) core.LayoutAudit {
	var l core.LayoutAudit
	for part := range strings.SplitSeq(value, ",") {
		name, n, _ := strings.Cut(part, "=")
		switch name {
		case "bounds":
			l.Bounds = true
		case "grid":
			l.Grid = 8
			if step, err := strconv.ParseFloat(n, 32); err == nil {
				l.Grid = core.Points(step)
			}
		}
	}
	return l
}
