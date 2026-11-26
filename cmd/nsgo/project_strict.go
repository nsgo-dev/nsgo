package main

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// decodeProject decodes an nsgo.yaml into p and refuses a key p has no
// field for. yaml drops such a key silently, so a misspelt permission or
// entitlement would ship the app without it.
func decodeProject(data []byte, p *project) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(p); err != nil {
		return nsgoYAMLDecodeError(data, err)
	}
	return nil
}

// unknownField matches yaml's error for a key its target has no field for,
// such as "line 3: field permisions not found in type main.android".
var unknownField = regexp.MustCompile(`^line (\d+): field (.+) not found in type `)

// nsgoYAMLDecodeError names the path of every unknown key and the closest
// known one, and prefixes each error with the file's line.
func nsgoYAMLDecodeError(data []byte, err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return fmt.Errorf("nsgo.yaml: %w", err)
	}
	var doc yaml.Node
	if perr := yaml.Unmarshal(data, &doc); perr != nil {
		return fmt.Errorf("nsgo.yaml: %w", err)
	}
	parts := make([]string, 0, len(typeErr.Errors))
	for _, e := range typeErr.Errors {
		m := unknownField.FindStringSubmatch(e)
		if m == nil {
			parts = append(parts, withoutLineWord(e))
			continue
		}
		line, _ := strconv.Atoi(m[1])
		parts = append(parts, unknownKeyMessage(&doc, line, m[2]))
	}
	return fmt.Errorf("nsgo.yaml:%s", strings.Join(parts, "; "))
}

// unknownKeyMessage says which mapping has no key and, when one is near,
// which key it probably meant.
func unknownKeyMessage(doc *yaml.Node, line int, key string) string {
	path := keyPath(doc, line, key)
	if len(path) == 0 {
		return fmt.Sprintf("%d: field %s is not a known key", line, key)
	}
	parent := strings.ReplaceAll(strings.Join(path[:len(path)-1], "."), ".[]", "[]")
	var msg string
	if parent == "" {
		msg = fmt.Sprintf("the top level has no key %s", key)
	} else {
		msg = fmt.Sprintf("%s has no key %s", parent, key)
	}
	if near := closestKey(yamlKeys(yamlTypeAt(reflect.TypeFor[project](), path[:len(path)-1])), key); near != "" {
		msg += "; the closest is " + near
	}
	return fmt.Sprintf("%d: %s", line, msg)
}

// withoutLineWord drops yaml's "line" before a line number, so that the
// file prefix reads nsgo.yaml:12.
func withoutLineWord(s string) string {
	return lineWord.ReplaceAllString(s, "$1: ")
}

var lineWord = regexp.MustCompile(`^line (\d+): `)

// keyPath returns the mapping keys from the document's root to the key on
// line, with "[]" for each sequence an element sits in. It returns nil when
// the document has no such key.
func keyPath(n *yaml.Node, line int, key string) []string {
	switch n.Kind {
	case yaml.DocumentNode:
		for _, c := range n.Content {
			if p := keyPath(c, line, key); p != nil {
				return p
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			if k.Line == line && k.Value == key {
				return []string{key}
			}
			if p := keyPath(v, line, key); p != nil {
				return append([]string{k.Value}, p...)
			}
		}
	case yaml.SequenceNode:
		for _, c := range n.Content {
			if p := keyPath(c, line, key); p != nil {
				return append([]string{"[]"}, p...)
			}
		}
	}
	return nil
}

// yamlTypeAt returns the type the value at path decodes to, nil when path
// names no field or the value takes free-form keys.
func yamlTypeAt(t reflect.Type, path []string) reflect.Type {
	for _, name := range path {
		if name == "[]" {
			for t.Kind() == reflect.Pointer {
				t = t.Elem()
			}
			if t.Kind() != reflect.Slice && t.Kind() != reflect.Array {
				return nil
			}
			t = t.Elem()
			continue
		}
		if t = yamlStruct(t); t == nil {
			return nil
		}
		f, ok := yamlField(t, name)
		if !ok {
			return nil
		}
		t = f.Type
	}
	return t
}

// yamlStruct returns t without its pointers, nil when t is not a struct of
// keys of its own. yaml.Node takes whatever its key holds, so it is not one.
func yamlStruct(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || t == reflect.TypeFor[yaml.Node]() {
		return nil
	}
	return t
}

// yamlField returns the field of t that name decodes into, inline fields
// searched too.
func yamlField(t reflect.Type, name string) (reflect.StructField, bool) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("yaml")
		if tag == "-" {
			continue
		}
		tagName, opts, _ := strings.Cut(tag, ",")
		if strings.Contains(opts, "inline") {
			if sub := yamlStruct(f.Type); sub != nil {
				if sf, ok := yamlField(sub, name); ok {
					return sf, true
				}
			}
			continue
		}
		if tagName == "" {
			tagName = strings.ToLower(f.Name)
		}
		if tagName == name {
			return f, true
		}
	}
	return reflect.StructField{}, false
}

// yamlKeys lists the keys t decodes, inline fields' included.
func yamlKeys(t reflect.Type) []string {
	if t = yamlStruct(t); t == nil {
		return nil
	}
	var keys []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("yaml")
		if tag == "-" {
			continue
		}
		tagName, opts, _ := strings.Cut(tag, ",")
		if strings.Contains(opts, "inline") {
			keys = append(keys, yamlKeys(f.Type)...)
			continue
		}
		if tagName == "" {
			tagName = strings.ToLower(f.Name)
		}
		keys = append(keys, tagName)
	}
	return keys
}

// closestKey returns the key in keys nearest to key when one is close
// enough to be its misspelling.
func closestKey(keys []string, key string) string {
	best, bestDistance := "", 0
	for _, k := range keys {
		d := editDistance(k, key)
		if d == 0 || d > 2 || d*3 > len(key) {
			continue
		}
		if best == "" || d < bestDistance {
			best, bestDistance = k, d
		}
	}
	return best
}

// editDistance returns the number of insertions, deletions and
// substitutions that turn a into b.
func editDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(br)]
}
