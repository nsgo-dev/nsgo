package ui

import "github.com/nsgo-dev/nsgo/core"

// FormSection is related rows on a rounded card, divided by hairlines from the start of the titles above them, as an inset grouped list of iOS Settings: Title above the card and Footer below it, in the muted color. Its rows are FormRow and FormField, or the app's own nodes, which pad themselves; a core.Each among them gives each of its rows a hairline of its own.
type FormSection struct {
	Title, Footer string
	Children      []core.Node
	TestID        string
	Style         core.Style
}

// Render implements core.Node.
func (s FormSection) Render(cx *core.Scope) core.Node {
	colors := ColorsOf(cx)
	// Each row has its hairline below it, so a row from core.Each takes and keeps its own as rows come, go and move; the card's last view hides its hairline.
	var rows []core.Node
	for _, child := range s.Children {
		if each, ok := core.AfterEachRow(child, func(_ *core.Scope, row core.Node) core.Node { return formDivider(colors, row) }); ok {
			rows = append(rows, each)
			continue
		}
		rows = append(rows, child, formDivider(colors, child))
	}
	var nodes []core.Node
	if s.Title != "" {
		title := Text(s.Title, 12, Medium, colors.Muted)
		title.Role = core.RoleHeader
		title.Style.Padding = core.Symmetric(0, formRowPad)
		nodes = append(nodes, title)
	}
	nodes = append(nodes, core.View{Style: core.Style{CornerRadius: RadiusCard, BorderCurve: core.BorderCurveContinuous, Overflow: core.Hidden},
		StyleFunc: func() core.Style { return core.Style{Background: Alpha(colors.Text(), 0.045)} }, Children: rows})
	if s.Footer != "" {
		footer := Text(s.Footer, 11.5, Regular, colors.Muted)
		footer.Style.LineHeight, footer.Style.Padding = 16, core.Symmetric(0, formRowPad)
		nodes = append(nodes, footer)
	}
	return core.View{TestID: s.TestID, Style: core.Style{Gap: 6}.Merge(s.Style), Children: nodes}
}

// formDivider is the hairline below row, which hides as the card's last view.
func formDivider(colors Colors, row core.Node) core.Node {
	return core.View{Class: "last:hidden", Style: core.Style{Height: core.Pt(1), Margin: core.Edges{Left: formRowLead(row)}}, StyleFunc: Filled(colors.Line)}
}

// The rows' metrics: their least height, their padding at the sides and
// the tile an icon sits on.
const (
	formRowHeight core.Points = 40
	formRowPad    core.Points = 12
	formRowGap    core.Points = 10
	formIconTile  core.Points = 24
)

// formRowLead is where the hairline below row starts: the start of its
// title, past an icon, as iOS insets a cell's separator.
func formRowLead(row core.Node) core.Points {
	if r, ok := row.(FormRow); ok && r.Icon != nil {
		return formRowPad + formIconTile + formRowGap
	}
	return formRowPad
}
