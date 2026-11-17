package core

import (
	"slices"
	"strconv"
	"strings"

	"github.com/nsgo-dev/nsgo/internal/driver"
)

// Class selectors match the engine's element tree, never a backend's implementation views. Regions are transparent and svg means Icon/SVGView.
type classSelector struct {
	choices [][]classSelectorStep
	watch   uint16
}

type classSelectorStep struct {
	relation byte // relation to the preceding step: space, >, + or ~
	part     classSelectorPart
}

type classSelectorPart struct {
	tag               string
	anchor            bool
	classes           []string
	attrs             []classSelectorAttr
	states, notStates uint16
	position          uint8
	nth               int
	functions         []classSelectorFunc
}

type classSelectorAttr struct {
	name, value string
	hasValue    bool
}
type classSelectorFunc struct {
	name     string
	selector *classSelector
}

// parseClassSelector parses native selectors. An arbitrary variant has one &, while :has takes relative selectors, as CSS does.
func parseClassSelector(s string, relative bool) (*classSelector, bool) {
	return parseClassSelectorAt(s, relative, 0)
}

func parseClassSelectorAt(s string, relative bool, depth int) (*classSelector, bool) {
	if depth > 8 || len(s) > 2048 || (relative || depth > 0) && strings.Contains(s, "&") {
		return nil, false
	}
	parts, ok := selectorSplit(s, ',')
	if !ok {
		return nil, false
	}
	out := &classSelector{}
	for _, text := range parts {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil, false
		}
		if relative {
			text = "& " + text
		}
		var steps []classSelectorStep
		var relation byte
		for len(text) > 0 {
			trimmed := strings.TrimLeft(text, " \t\r\n")
			if len(trimmed) != len(text) && len(steps) > 0 && relation == 0 {
				relation = ' '
			}
			text = trimmed
			if text == "" {
				break
			}
			if strings.ContainsRune(">+~", rune(text[0])) {
				if len(steps) == 0 || relation != 0 && relation != ' ' {
					return nil, false
				}
				relation, text = text[0], text[1:]
				continue
			}
			if len(steps) > 0 && relation == 0 {
				return nil, false
			}
			end := selectorPartEnd(text)
			if end == 0 {
				return nil, false
			}
			part, watch, ok := parseSelectorPart(text[:end], depth)
			if !ok {
				return nil, false
			}
			steps = append(steps, classSelectorStep{relation, part})
			out.watch |= watch
			relation, text = 0, text[end:]
		}
		if len(steps) == 0 || relation != 0 && relation != ' ' {
			return nil, false
		}
		out.choices = append(out.choices, steps)
	}
	return out, true
}

// selectorSplit and selectorPartEnd keep punctuation inside attributes and functional pseudo-classes out of the selector's combinators.
func selectorSplit(s string, delimiter byte) ([]string, bool) {
	var parts []string
	start, brackets, parens := 0, 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' {
			return nil, false
		}
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '[':
			brackets++
		case ']':
			brackets--
		case '(':
			parens++
		case ')':
			parens--
		default:
			if c == delimiter && brackets == 0 && parens == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
		if brackets < 0 || parens < 0 {
			return nil, false
		}
	}
	if quote != 0 || brackets != 0 || parens != 0 {
		return nil, false
	}
	return append(parts, s[start:]), true
}

func selectorPartEnd(s string) int {
	brackets, parens := 0, 0
	var quote byte
	for i := range len(s) {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case '[':
			brackets++
		case ']':
			brackets--
		case '(':
			parens++
		case ')':
			parens--
		default:
			if brackets == 0 && parens == 0 && strings.ContainsRune(" \t\r\n>+~", rune(c)) {
				return i
			}
		}
	}
	return len(s)
}

// selectorClosing ignores delimiters inside quoted attribute values.
func selectorClosing(s string, open, close byte) int {
	level := 0
	var quote byte
	for i := range len(s) {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
		} else if c == open {
			level++
		} else if c == close {
			level--
			if level == 0 {
				return i
			}
		}
	}
	return -1
}

func selectorName(s string) (string, string) {
	i := 0
	for i < len(s) && (s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z' || s[i] >= '0' && s[i] <= '9' || s[i] == '-' || s[i] == '_') {
		i++
	}
	return s[:i], s[i:]
}

func parseSelectorPart(s string, depth int) (p classSelectorPart, watch uint16, ok bool) {
	if s == "" {
		return p, 0, false
	}
	if s[0] == '&' {
		p.anchor, s = true, s[1:]
	} else if s[0] == '*' {
		p.tag, s = "*", s[1:]
	} else {
		p.tag, s = selectorName(s)
		if p.tag != "" && !selectorTagKnown(p.tag) {
			return p, 0, false
		}
	}
	for len(s) > 0 {
		switch s[0] {
		case '.':
			var name string
			name, s = selectorName(s[1:])
			if name == "" {
				return p, 0, false
			}
			p.classes = append(p.classes, name)
		case '[':
			i := selectorClosing(s, '[', ']')
			if i < 0 {
				return p, 0, false
			}
			name, value, has := strings.Cut(s[1:i], "=")
			name, value = strings.TrimSpace(name), strings.TrimSpace(value)
			if strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
				if len(value) < 2 || value[len(value)-1] != value[0] {
					return p, 0, false
				}
				value = value[1 : len(value)-1]
			}
			if !selectorAttrKnown(name) || has && value == "" {
				return p, 0, false
			}
			p.attrs = append(p.attrs, classSelectorAttr{name, value, has})
			s = s[i+1:]
		case ':':
			var name string
			name, s = selectorName(s[1:])
			if name == "" {
				return p, 0, false
			}
			if len(s) > 0 && s[0] == '(' {
				close := selectorClosing(s, '(', ')')
				if close < 0 {
					return p, 0, false
				}
				end := close + 1
				arg := strings.TrimSpace(s[1 : end-1])
				s = s[end:]
				if name == "nth-child" {
					switch arg {
					case "odd":
						p.position |= positionOdd
					case "even":
						p.position |= positionEven
					default:
						p.nth, _ = strconv.Atoi(arg)
						if p.nth <= 0 {
							return p, 0, false
						}
					}
				} else {
					if name != "is" && name != "where" && name != "not" && name != "has" {
						return p, 0, false
					}
					inner, valid := parseClassSelectorAt(arg, name == "has", depth+1)
					if !valid {
						return p, 0, false
					}
					watch |= inner.watch
					p.functions = append(p.functions, classSelectorFunc{name, inner})
				}
			} else if v, exists := classVariants[name]; exists {
				p.states |= v.bit
				watch |= v.bit
			} else {
				switch name {
				case "enabled":
					p.notStates |= classDisabled
				case "first-child":
					p.position |= positionFirst
				case "last-child":
					p.position |= positionLast
				case "only-child":
					p.position |= positionOnly
				default:
					return p, 0, false
				}
			}
		default:
			return p, 0, false
		}
	}
	return p, watch, true
}

func selectorTagKnown(tag string) bool {
	switch tag {
	case "div", "view", "text", "span", "scroll", "input", "textarea", "img", "image", "canvas", "svg", "video", "webview", "native":
		return true
	}
	return false
}

func selectorAttrKnown(name string) bool {
	if strings.HasPrefix(name, "data-") {
		_, ok := dataVariant(name)
		return ok
	}
	switch name {
	case "role", "disabled", "checked", "aria-checked", "aria-disabled", "aria-selected", "aria-expanded", "aria-busy", "aria-invalid", "aria-pressed":
		return true
	}
	return false
}

func (p *classSelectorPart) matches(t *Tree, n *node) bool {
	if n == nil || n.deleted || n.kind == 0 {
		return false
	}
	if p.tag != "" && p.tag != "*" {
		match := false
		switch p.tag {
		case "svg":
			match = n.icon != nil
		case "canvas":
			match = n.kind == driver.KindCanvas && n.icon == nil
		case "div", "view":
			match = n.kind == driver.KindView
		case "text", "span":
			match = n.kind == driver.KindText
		case "scroll":
			match = n.kind == driver.KindScroll
		case "input":
			match = n.kind == driver.KindInput
		case "textarea":
			match = n.input != nil && n.input.config.Multiline
		case "img", "image":
			match = n.kind == driver.KindImage
		case "video":
			match = n.kind == driver.KindVideo
		case "webview":
			match = n.kind == driver.KindWeb
		case "native":
			match = n.native != nil
		}
		if !match {
			return false
		}
	}
	for _, name := range p.classes {
		if n.cls == nil || !slices.Contains(strings.Fields(n.cls.list.class), name) {
			return false
		}
	}
	shown, bits := t.selectorStates(n)
	if bits&p.states != p.states || bits&p.notStates != 0 {
		return false
	}
	if p.position != 0 && n.position()&p.position != p.position {
		return false
	}
	if p.nth > 0 {
		parent := n.nativeParent()
		if parent == nil || slices.Index(parent.flatChildren(), n) != p.nth-1 {
			return false
		}
	}
	for _, a := range p.attrs {
		if !selectorAttr(n, shown, a) {
			return false
		}
	}
	for _, f := range p.functions {
		found := false
		if f.name == "has" {
			found = f.selector.has(t, n)
		} else {
			found = f.selector.matches(t, n, nil)
		}
		if found == (f.name == "not") {
			return false
		}
	}
	return true
}

func (t *Tree) selectorStates(n *node) (shownStates, uint16) {
	shown := t.sharedStates(n)
	var bits uint16
	for i, on := range shown.on {
		if on {
			bits |= classStates[i]
		}
	}
	return shown, bits
}

func selectorAttr(n *node, shown shownStates, a classSelectorAttr) bool {
	if name, data := strings.CutPrefix(a.name, "data-"); data {
		for _, item := range shown.data {
			key, value, has := strings.Cut(item, "=")
			if key == name && (!a.hasValue || has && value == a.value) {
				return true
			}
		}
		return false
	}
	if a.name == "role" {
		return n.props.Role != 0 && (!a.hasValue || driver.RoleName(n.props.Role) == a.value)
	}
	if a.name == "aria-checked" && shown.on[1] {
		return !a.hasValue || a.value == "mixed"
	}
	var value bool
	switch a.name {
	case "disabled", "aria-disabled":
		value = shown.on[stateCount-1]
	case "checked", "aria-checked":
		value = shown.on[0]
	case "aria-selected":
		value = shown.on[2]
	case "aria-expanded":
		value = shown.on[10]
	case "aria-busy":
		value = shown.on[8]
	case "aria-invalid":
		value = shown.on[11]
	case "aria-pressed":
		value = shown.on[12]
	}
	if !a.hasValue {
		return value
	}
	if a.value == "true" {
		return value
	}
	if a.value == "false" {
		return !value
	}
	return false
}

// eachAnchor matches right to left and reports the element substituted for &, which lets stacked arbitrary variants compose without rescanning every possible intermediate element.
func (s *classSelector) eachAnchor(t *Tree, target, anchor *node, found func(*node) bool) bool {
	for _, steps := range s.choices {
		var match func(int, *node, *node) bool
		match = func(i int, n, bound *node) bool {
			if n == nil || !steps[i].part.matches(t, n) {
				return false
			}
			if steps[i].part.anchor {
				if anchor != nil && anchor != n || bound != nil && bound != n {
					return false
				}
				bound = n
			}
			if i == 0 {
				return found(bound)
			}
			switch steps[i].relation {
			case '>':
				return match(i-1, n.nativeParent(), bound)
			case ' ':
				for p := n.nativeParent(); p != nil; p = p.nativeParent() {
					if match(i-1, p, bound) {
						return true
					}
				}
			case '+', '~':
				p := n.nativeParent()
				if p == nil {
					return false
				}
				kids := p.flatChildren()
				pos := slices.Index(kids, n)
				for j := pos - 1; j >= 0; j-- {
					if match(i-1, kids[j], bound) {
						return true
					}
					if steps[i].relation == '+' {
						break
					}
				}
			}
			return false
		}
		if match(len(steps)-1, target, nil) {
			return true
		}
	}
	return false
}

func (s *classSelector) matches(t *Tree, target, anchor *node) bool {
	return s.eachAnchor(t, target, anchor, func(*node) bool { return true })
}

func (s *classSelector) has(t *Tree, anchor *node) bool {
	// Relative siblings can leave the anchor's subtree, but cannot leave its parent's. Descendants stay inside it, including through regions.
	root := anchor
	if s.scopeMode() > 0 && anchor.nativeParent() != nil {
		root = anchor.nativeParent()
	}
	var walk func(*node) bool
	walk = func(n *node) bool {
		if n.kind != 0 && s.matches(t, n, anchor) {
			return true
		}
		for _, k := range n.flatChildren() {
			if walk(k) {
				return true
			}
		}
		return false
	}
	return walk(root)
}
