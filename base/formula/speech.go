package formula

import (
	"strings"
	"unicode"

	"github.com/caelyreth/untie/display"

	"github.com/nsgo-dev/nsgo/base/formula/locales"
)

// speak says a formula's description in m's language, as Untie's English speech says it, with the words from the catalogs: its literal letters, digits and text stay as written.
func speak(m locales.Messages, d *display.AltDescription) string {
	if d == nil {
		return m.Formula()
	}
	s := speaker{m}
	out := s.node(d.Root)
	for _, tag := range d.Tags {
		if tag.Suppressed || tag.Body == nil || tag.Line != display.FormulaTagLine {
			continue
		}
		out = join(out, m.Equation(s.node(*tag.Body)))
	}
	if strings.TrimSpace(out) == "" {
		return m.Formula()
	}
	return out
}

type speaker struct{ m locales.Messages }

// join joins words with a space, leaving out empty ones.
func join(words ...string) string {
	var b strings.Builder
	for _, w := range words {
		if w = strings.TrimSpace(w); w == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(w)
	}
	return b.String()
}

// part returns n's child in role and whether it has one.
func part(n display.AltNode, role display.AltPartRole) (display.AltNode, bool) {
	for _, p := range n.Parts {
		if p.Role == role {
			return p.Node, true
		}
	}
	return display.AltNode{}, false
}

// partSpeech says n's child in role, or "" without one.
func (s speaker) part(n display.AltNode, role display.AltPartRole) string {
	if c, ok := part(n, role); ok {
		return s.node(c)
	}
	return ""
}

// literal returns the text of a node that is one character or word as written, as an exponent's 2.
func literal(n display.AltNode) string {
	for n.Kind == display.AltKindRow && len(n.Parts) == 1 {
		n = n.Parts[0].Node
	}
	switch n.Kind {
	case display.AltKindGlyph, display.AltKindText, display.AltKindOperator:
		return strings.TrimSpace(n.Text)
	}
	return ""
}

func (s speaker) node(n display.AltNode) string {
	m := s.m
	switch n.Kind {
	case display.AltKindRow:
		words := make([]string, 0, len(n.Parts))
		for i := 0; i < len(n.Parts); i++ {
			if number := numberAt(n.Parts, i); number != "" {
				// A number's digits read as the number, as 16.7.
				words = append(words, number)
				i += len([]rune(number)) - 1
				continue
			}
			words = append(words, s.node(n.Parts[i].Node))
		}
		return join(words...)
	case display.AltKindGlyph, display.AltKindSizedDelimiter:
		if n.Text == "." && n.Kind == display.AltKindSizedDelimiter {
			return ""
		}
		return s.variant(n.Variant, s.glyph(n.Text))
	case display.AltKindText:
		return n.Text
	case display.AltKindOperator:
		if f, ok := operatorNames[strings.TrimSpace(n.Text)]; ok {
			return f(m)
		}
		return s.glyph(n.Text)
	case display.AltKindFraction:
		num, den := s.part(n, display.AltPartNumerator), s.part(n, display.AltPartDenominator)
		switch {
		case n.FractionRole == display.AltFractionBinomial:
			return m.Binomial(num, den)
		case n.FractionStyle == display.AltFractionHorizontal:
			return m.DividedBy(num, den)
		}
		return m.Fraction(num, den)
	case display.AltKindRadical:
		radicand := s.part(n, display.AltPartRadicand)
		index, ok := part(n, display.AltPartIndex)
		switch {
		case !ok || literal(index) == "" && s.node(index) == "" || literal(index) == "2":
			return m.SquareRoot(radicand)
		case literal(index) == "3":
			return m.CubeRoot(radicand)
		case literal(index) == "4":
			return m.FourthRoot(radicand)
		}
		return m.Root(s.node(index), radicand)
	case display.AltKindScripts:
		return s.scripts(n)
	case display.AltKindFenced:
		return join(s.part(n, display.AltPartOpen), s.part(n, display.AltPartContent), s.part(n, display.AltPartClose))
	case display.AltKindAccent:
		body := s.part(n, display.AltPartBody)
		accent, _ := part(n, display.AltPartAccent)
		name := accentName(m, literal(accent))
		if name == "" {
			name = m.AccentMark(literal(accent))
		}
		if n.Position == display.AltPositionBelow && !underMark(literal(accent)) {
			return m.AccentBelow(body, name)
		}
		return m.Accent(body, name)
	case display.AltKindLine:
		body := s.part(n, display.AltPartBody)
		if n.Position == display.AltPositionBelow {
			return m.Underline(body)
		}
		return m.Overline(body)
	case display.AltKindCancel:
		body := s.part(n, display.AltPartBody)
		switch {
		case n.CancelRole == display.AltCancelNegation:
			return m.Negated(body)
		case n.CancelRole == display.AltCancelSlashed:
			return m.Slashed(body)
		case hasFlag(n, display.AltFlagCross):
			return m.CrossedOut(body)
		case hasFlag(n, display.AltFlagInverted):
			return m.CanceledBackslash(body)
		}
		return m.Canceled(body)
	case display.AltKindTable:
		rows := s.rows(n.Rows, true)
		switch n.TableRole {
		case display.AltTableArray:
			return m.Array(len(n.Rows), rows)
		case display.AltTableCases:
			return m.Cases(len(n.Rows), rows)
		case display.AltTableVector:
			return m.Vector(len(n.Rows), rows)
		}
		return m.Matrix(len(n.Rows), rows)
	case display.AltKindMultiline:
		return m.Multiline(s.rows(n.Rows, false))
	case display.AltKindBox:
		return m.Boxed(s.part(n, display.AltPartBody))
	case display.AltKindPhantom:
		// Nothing shows of it.
		return ""
	case display.AltKindStyle, display.AltKindOverlay, display.AltKindRaise, display.AltKindColor:
		return s.part(n, display.AltPartBody)
	case display.AltKindError:
		return m.Error(n.Text)
	case display.AltKindPlaceholder:
		return m.Missing()
	}
	// Spaces, breaks and alignment points read as nothing.
	return ""
}

// scripts says a base with scripts or limits.
func (s speaker) scripts(n display.AltNode) string {
	m := s.m
	base := s.part(n, display.AltPartBase)
	top, hasTop := part(n, display.AltPartTop)
	_, hasBottom := part(n, display.AltPartBottom)
	topSpeech, bottomSpeech := s.part(n, display.AltPartTop), s.part(n, display.AltPartBottom)
	out := base
	switch n.ScriptRole {
	case display.AltScriptLimits:
		switch {
		case hasTop && hasBottom:
			out = m.Limits(base, bottomSpeech, topSpeech)
		case hasBottom:
			out = m.LowerLimit(base, bottomSpeech)
		case hasTop:
			out = m.UpperLimit(base, topSpeech)
		}
	case display.AltScriptOverlay:
		switch {
		case hasTop && hasBottom:
			out = m.AboveAndBelow(base, topSpeech, bottomSpeech)
		case hasTop:
			out = m.Above(base, topSpeech)
		case hasBottom:
			out = m.Below(base, bottomSpeech)
		}
	case display.AltScriptBraceLabel:
		if hasTop {
			out = m.Labeled(base, topSpeech)
		}
		if hasBottom {
			out = m.Labeled(out, bottomSpeech)
		}
	default:
		// Scripts over and under an ordinary base are its superscript and subscript.
		if hasBottom {
			out = m.Subscript(out, bottomSpeech)
		}
		if hasTop {
			out = s.power(out, top)
		}
	}
	if left := s.part(n, display.AltPartBottomLeft); left != "" {
		out = m.PreSubscript(out, left)
	}
	if left := s.part(n, display.AltPartTopLeft); left != "" {
		out = m.PreSuperscript(out, left)
	}
	if sub := s.part(n, display.AltPartBottomRight); sub != "" {
		out = m.Subscript(out, sub)
	}
	if sup, ok := part(n, display.AltPartTopRight); ok {
		out = s.power(out, sup)
	}
	return out
}

// power says base raised to exponent.
func (s speaker) power(base string, exponent display.AltNode) string {
	switch literal(exponent) {
	case "2":
		return s.m.Squared(base)
	case "3":
		return s.m.Cubed(base)
	}
	return s.m.Power(base, s.node(exponent))
}

// rows says a table's or a multiline formula's rows, cells apart where cells.
func (s speaker) rows(rows []display.AltRow, cells bool) string {
	m := s.m
	said := make([]string, len(rows))
	for i, row := range rows {
		var words []string
		empty := 0
		for _, item := range row.Items {
			w := s.node(item)
			if w == "" && cells {
				w = m.EmptyCell()
				empty++
			}
			if w != "" {
				words = append(words, w)
			}
		}
		sep := " "
		if cells {
			sep = m.CellSeparator()
		}
		text := strings.Join(words, sep)
		if text == "" || cells && empty == len(row.Items) {
			text = m.EmptyRow()
		}
		said[i] = m.Row(i+1, text)
	}
	return strings.Join(said, m.RowSeparator())
}

// glyph says a character: by its name, as itself when it is letters and digits, else as a symbol.
func (s speaker) glyph(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	// A variation selector says nothing of its own.
	if r := []rune(text); len(r) == 2 && r[1] >= '︀' && r[1] <= '️' {
		text = string(r[0])
	}
	if f, ok := symbolNames[text]; ok {
		return f(s.m)
	}
	for _, r := range text {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return s.m.Symbol(text)
		}
	}
	return text
}

// variant says a letter in a math alphabet that tells it apart, as ℝ's double-struck R.
func (s speaker) variant(v display.AltVariant, letter string) string {
	if letter == "" {
		return ""
	}
	switch v {
	case display.AltVariantDoubleStruck:
		return s.m.DoubleStruck(letter)
	case display.AltVariantBold, display.AltVariantBoldItalic, display.AltVariantBoldSans:
		return s.m.Bold(letter)
	case display.AltVariantScript, display.AltVariantBoldScript:
		return s.m.Script(letter)
	case display.AltVariantFraktur, display.AltVariantBoldFraktur:
		return s.m.Fraktur(letter)
	}
	return letter
}

// numberAt returns the number whose digits, one a part, start at parts[i]: digits with at most one decimal point between them; "" when parts[i] is no digit.
func numberAt(parts []display.AltPart, i int) string {
	digit := func(k int) bool {
		if k >= len(parts) {
			return false
		}
		n := parts[k].Node
		return n.Kind == display.AltKindGlyph && len(n.Text) == 1 && n.Text[0] >= '0' && n.Text[0] <= '9' && n.Variant == ""
	}
	if !digit(i) {
		return ""
	}
	var b strings.Builder
	point := false
	for k := i; k < len(parts); k++ {
		switch {
		case digit(k):
			b.WriteString(parts[k].Node.Text)
		case !point && parts[k].Node.Kind == display.AltKindGlyph && parts[k].Node.Text == "." && digit(k+1):
			point = true
			b.WriteByte('.')
		default:
			return b.String()
		}
	}
	return b.String()
}

func hasFlag(n display.AltNode, f display.AltFlag) bool {
	for _, g := range n.Flags {
		if g == f {
			return true
		}
	}
	return false
}

// underMark reports whether mark is one that is under what it marks by name, as an underbrace.
func underMark(mark string) bool {
	return strings.ContainsAny(mark, "⏟⎵⏝⏡")
}

// accentName names an accent by its mark, "" for one without a name.
func accentName(m locales.Messages, mark string) string {
	for _, r := range mark {
		switch r {
		case '̂', '^', 'ˆ', '⌃':
			return m.AccentHat()
		case '̄', '¯', '-', '−', '―':
			return m.AccentBar()
		case '̃', '~', '˜', '∼':
			return m.AccentTilde()
		case '̇', '.', '⋅':
			return m.AccentDot()
		case '̈', '¨':
			return m.AccentDoubleDot()
		case '⃗', '→':
			return m.AccentVector()
		case '́', '´':
			return m.AccentAcute()
		case '̀', '`':
			return m.AccentGrave()
		case '̆', '˘':
			return m.AccentBreve()
		case '̌', 'ˇ':
			return m.AccentCaron()
		case '̊', '∘', '○':
			return m.AccentRing()
		case '̧', '¸':
			return m.AccentCedilla()
		case '̨', '˛':
			return m.AccentOgonek()
		case '̋', '˝':
			return m.AccentDoubleAcute()
		case '⏞':
			return m.Overbrace()
		case '⏟':
			return m.Underbrace()
		case '⎴':
			return m.Overbracket()
		case '⎵':
			return m.Underbracket()
		case '⏜':
			return m.Overparen()
		case '⏝':
			return m.Underparen()
		case '⏠':
			return m.Overshell()
		case '⏡':
			return m.Undershell()
		}
	}
	return ""
}

// symbolNames name the characters a formula says by name.
var symbolNames = map[string]func(locales.Messages) string{
	"+": locales.Messages.SymPlus,
	"-": locales.Messages.SymMinus,
	"−": locales.Messages.SymMinus,
	"*": locales.Messages.SymAsterisk,
	"/": locales.Messages.SymSlash,
	"=": locales.Messages.SymEquals,
	"≠": locales.Messages.SymNotEqual,
	"<": locales.Messages.SymLessThan,
	">": locales.Messages.SymGreaterThan,
	"≤": locales.Messages.SymLessOrEqual,
	"≥": locales.Messages.SymGreaterOrEqual,
	"≦": locales.Messages.SymLessOrEqual,
	"≧": locales.Messages.SymGreaterOrEqual,
	"≈": locales.Messages.SymApproximately,
	"≉": locales.Messages.SymNotApproximately,
	"≡": locales.Messages.SymEquivalent,
	"≢": locales.Messages.SymNotEquivalent,
	"≅": locales.Messages.SymCongruent,
	"∼": locales.Messages.SymSimilar,
	"≁": locales.Messages.SymNotSimilar,
	"±": locales.Messages.SymPlusMinus,
	"∓": locales.Messages.SymMinusPlus,
	"×": locales.Messages.SymTimes,
	"·": locales.Messages.SymDot,
	"⋅": locales.Messages.SymDot,
	"÷": locales.Messages.SymDividedSign,
	"∞": locales.Messages.SymInfinity,
	"∑": locales.Messages.SymSum,
	"∏": locales.Messages.SymProduct,
	"∐": locales.Messages.SymCoproduct,
	"∫": locales.Messages.SymIntegral,
	"∬": locales.Messages.SymDoubleIntegral,
	"∭": locales.Messages.SymTripleIntegral,
	"∮": locales.Messages.SymContourIntegral,
	"∂": locales.Messages.SymPartial,
	"∇": locales.Messages.SymNabla,
	"√": locales.Messages.SymSquareRootSign,
	"∀": locales.Messages.SymForAll,
	"∃": locales.Messages.SymExists,
	"∄": locales.Messages.SymNotExists,
	"∈": locales.Messages.SymElementOf,
	"∉": locales.Messages.SymNotElementOf,
	"∋": locales.Messages.SymContains,
	"∌": locales.Messages.SymNotContains,
	"∩": locales.Messages.SymIntersection,
	"∪": locales.Messages.SymUnion,
	"∧": locales.Messages.SymAnd,
	"∨": locales.Messages.SymOr,
	"¬": locales.Messages.SymNot,
	"→": locales.Messages.SymRightArrow,
	"←": locales.Messages.SymLeftArrow,
	"↔": locales.Messages.SymLeftRightArrow,
	"⇒": locales.Messages.SymImplies,
	"⇐": locales.Messages.SymImpliedBy,
	"⇔": locales.Messages.SymIfAndOnlyIf,
	"↦": locales.Messages.SymMapsTo,
	"↑": locales.Messages.SymUpArrow,
	"↓": locales.Messages.SymDownArrow,
	"⊂": locales.Messages.SymSubset,
	"⊆": locales.Messages.SymSubsetOrEqual,
	"⊃": locales.Messages.SymSuperset,
	"⊇": locales.Messages.SymSupersetOrEqual,
	"⊄": locales.Messages.SymNotSubset,
	"⊈": locales.Messages.SymNotSubsetOrEqual,
	"⊥": locales.Messages.SymPerpendicular,
	"∥": locales.Messages.SymParallel,
	"∠": locales.Messages.SymAngle,
	"∘": locales.Messages.SymComposed,
	"′": locales.Messages.SymPrime,
	"″": locales.Messages.SymDoublePrime,
	"‴": locales.Messages.SymTriplePrime,
	"(": locales.Messages.SymLeftParenthesis,
	")": locales.Messages.SymRightParenthesis,
	"[": locales.Messages.SymLeftBracket,
	"]": locales.Messages.SymRightBracket,
	"{": locales.Messages.SymLeftBrace,
	"}": locales.Messages.SymRightBrace,
	"|": locales.Messages.SymVerticalBar,
	"‖": locales.Messages.SymDoubleVerticalBar,
	"⟨": locales.Messages.SymLeftAngle,
	"⟩": locales.Messages.SymRightAngle,
	"⌈": locales.Messages.SymLeftCeiling,
	"⌉": locales.Messages.SymRightCeiling,
	"⌊": locales.Messages.SymLeftFloor,
	"⌋": locales.Messages.SymRightFloor,
	",": locales.Messages.SymComma,
	".": locales.Messages.SymPoint,
	":": locales.Messages.SymColon,
	";": locales.Messages.SymSemicolon,
	"!": locales.Messages.SymFactorial,
	"?": locales.Messages.SymQuestionMark,
	"%": locales.Messages.SymPercent,
	"…": locales.Messages.SymEllipsis,
	"⋯": locales.Messages.SymEllipsis,
	"∅": locales.Messages.SymEmptySet,
}

// operatorNames name the operators a formula says by name, as sin's sine.
var operatorNames = map[string]func(locales.Messages) string{
	"sin": locales.Messages.OpSin,
	"cos": locales.Messages.OpCos,
	"tan": locales.Messages.OpTan,
	"cot": locales.Messages.OpCot,
	"sec": locales.Messages.OpSec,
	"csc": locales.Messages.OpCsc,
	"sinh": locales.Messages.OpSinh,
	"cosh": locales.Messages.OpCosh,
	"tanh": locales.Messages.OpTanh,
	"log": locales.Messages.OpLog,
	"ln": locales.Messages.OpLn,
	"exp": locales.Messages.OpExp,
	"lim": locales.Messages.OpLim,
	"max": locales.Messages.OpMax,
	"min": locales.Messages.OpMin,
	"sup": locales.Messages.OpSup,
	"inf": locales.Messages.OpInf,
	"arg": locales.Messages.OpArg,
	"det": locales.Messages.OpDet,
	"gcd": locales.Messages.OpGcd,
	"lcm": locales.Messages.OpLcm,
	"mod": locales.Messages.OpMod,
	"ker": locales.Messages.OpKer,
	"dim": locales.Messages.OpDim,
	"rank": locales.Messages.OpRank,
	"argmax": locales.Messages.OpArgmax,
	"argmin": locales.Messages.OpArgmin,
}
