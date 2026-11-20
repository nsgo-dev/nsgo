// Command grid is a stress case on NSGo alone: 600 cells, 30 by 20, whose
// numbers all change every display frame, so each frame changes 600 texts.
package main

import (
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/benchmark/internal/bench"
	"github.com/nsgo-dev/nsgo/core"
)

const (
	defaultCols, defaultRows = 30, 20
	cellW                    = 40
	cellH                    = 20
)

var (
	ink   = core.RGB(0x111111)
	zebra = core.RGB(0xf6f7f9)
)

// gridSize reads the optional dimensions used for a layer-capacity run, NSGO_GRID_COLS and NSGO_GRID_ROWS, which the runner's -grid-cols and -grid-rows set. The defaults keep the published 600-cell case unchanged. Invalid and nonpositive values fall back so a benchmark typo cannot make an empty window.
func gridSize() (cols, rows int) {
	return gridDimension(os.Getenv("NSGO_GRID_COLS"), defaultCols),
		gridDimension(os.Getenv("NSGO_GRID_ROWS"), defaultRows)
}

func gridDimension(value string, fallback int) int {
	if n, err := strconv.Atoi(value); err == nil && n > 0 {
		return n
	}
	return fallback
}

func root(cx *core.Scope) core.Node {
	cols, rows := gridSize()
	// A row is one signal of its numbers: a tick sets every row once.
	seed := uint32(42)
	next := func() int {
		seed = seed*1664525 + 1013904223
		return int(seed>>16) % 10000
	}
	values := make([]*core.Signal[[]int], rows)
	for r := range values {
		v := make([]int, cols)
		for c := range v {
			v[c] = next()
		}
		values[r] = core.NewSignalFunc(cx, v, slices.Equal)
	}
	var frame func(time.Time)
	frame = func(time.Time) {
		bench.Frame()
		for _, row := range values {
			v := make([]int, cols)
			for c := range v {
				v[c] = next()
			}
			row.Set(v)
		}
		cx.RequestAnimationFrame(frame)
	}
	cx.RequestAnimationFrame(frame)

	lines := make([]core.Node, rows)
	for r, row := range values {
		cells := make([]core.Node, cols)
		for c := range cells {
			cells[c] = core.Text{
				ContentFunc: func() string { return strconv.Itoa(row.Get()[c]) },
				Style:       core.Style{Width: core.Pt(cellW), FontSize: 12, Color: ink, TextAlign: core.TextAlignEnd},
			}
		}
		bg := core.RGB(0xffffff)
		if r%2 == 1 {
			bg = zebra
		}
		lines[r] = core.View{
			Style:    core.Style{Direction: core.Row, Height: core.Pt(cellH), AlignItems: core.AlignCenter, Background: bg},
			Children: cells,
		}
	}
	return core.View{Style: core.Style{FlexGrow: 1, Padding: core.All(16)}, Children: lines}
}

func main() {
	cols, rows := gridSize()
	nsgo.Run(root, nsgo.WindowSize(core.Points(cols*cellW+32), core.Points(rows*cellH+32)))
}
