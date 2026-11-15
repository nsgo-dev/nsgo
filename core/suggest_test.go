package core

import "testing"

func TestSuggestionRange(t *testing.T) {
	for _, tc := range []struct {
		name, value, want string
		caret, from, to   int
		ok                bool
	}{
		{name: "start", value: "@re", caret: 3, from: 0, to: 3, want: "re", ok: true},
		{name: "after emoji", value: "😀 @re", caret: 6, from: 3, to: 6, want: "re", ok: true},
		{name: "word prefix", value: "mail@re", caret: 7, ok: false},
		{name: "space closes", value: "@read me", caret: 8, ok: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			from, to, query, ok := suggestionRange(tc.value, tc.caret, SuggestOptions{})
			if from != tc.from || to != tc.to || query != tc.want || ok != tc.ok {
				t.Fatalf("got (%d, %d, %q, %v), want (%d, %d, %q, %v)", from, to, query, ok, tc.from, tc.to, tc.want, tc.ok)
			}
		})
	}
}

func TestSuggestionRangeAllowsSpaces(t *testing.T) {
	from, to, query, ok := suggestionRange("@release notes", UTF16Len("@release notes"), SuggestOptions{AllowSpaces: true})
	if !ok || from != 0 || to != UTF16Len("@release notes") || query != "release notes" {
		t.Fatalf("got (%d, %d, %q, %v)", from, to, query, ok)
	}
}

func TestMapOffset(t *testing.T) {
	for _, tc := range []struct {
		name, old, next string
		at, want        int
	}{
		{name: "edit after", old: "hi @al", next: "hi @alx", at: 3, want: 3},
		{name: "insert before", old: "hi @al", next: "oh hi @al", at: 3, want: 6},
		{name: "insert at", old: "@al", next: "x@al", at: 0, want: 1},
		{name: "trigger deleted", old: "hi @al", next: "hi al", at: 3, want: -1},
		{name: "emoji before", old: "😀 @al", next: "😀😀 @al", at: 3, want: 5},
		{name: "emoji changed before", old: "😀 @al", next: "😁 @al", at: 3, want: 3},
		{name: "none", old: "@al", next: "x@al", at: -1, want: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := mapOffset(tc.at, tc.old, tc.next); got != tc.want {
				t.Fatalf("mapOffset(%d, %q, %q) = %d, want %d", tc.at, tc.old, tc.next, got, tc.want)
			}
		})
	}
}

func TestSuggestionRangeAllowedPrefixes(t *testing.T) {
	o := SuggestOptions{AllowedPrefixes: []string{"("}}
	if _, _, query, ok := suggestionRange("(@re", 4, o); !ok || query != "re" {
		t.Fatalf("after (: %q, %v", query, ok)
	}
	if _, _, _, ok := suggestionRange("hi @re", 6, o); ok {
		t.Fatal("a space opened a query its prefixes leave out")
	}
	for _, value := range []string{"@re", "hi\n@re"} {
		if _, _, query, ok := suggestionRange(value, UTF16Len(value), o); !ok || query != "re" {
			t.Fatalf("at a line's start of %q: %q, %v", value, query, ok)
		}
	}
}
