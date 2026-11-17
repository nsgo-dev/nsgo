package core

import (
	"fmt"
	"slices"
	"testing"
)

func TestClassNamesBelongToTheirTheme(t *testing.T) {
	const class = "bg-engine-brand text-(--engine-brand)"
	firstTheme, secondTheme := DefaultTheme(), DefaultTheme()
	firstTheme.Light.SetColor("engine-brand", RGB(0x123456))
	firstTheme.Dark.SetColor("engine-brand", RGB(0xabcdef))
	var aScope, bScope *Scope
	a := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		aScope = cx
		cx.SetTheme(firstTheme)
		return View{TestID: "brand", Class: class, Children: []Node{Func(func(cx *Scope) Node {
			cx.SetTheme(secondTheme)
			return Text{TestID: "other-subtree", Spans: []Span{{Content: "brand", Class: class}}}
		})}}
	})
	b := newHost(t, Size{Width: 200, Height: 100}, func(cx *Scope) Node {
		bScope = cx
		cx.SetTheme(secondTheme)
		return View{TestID: "brand", Class: class}
	})
	a.commit()
	b.commit()
	if got := a.find("brand").style.Background; got != RGB(0x123456) {
		t.Fatalf("first engine's brand = %#x", got)
	}
	if got := a.find("other-subtree").text.spans[0].cls.list.unknown; len(got) != 2 {
		t.Fatalf("color leaked into a sibling theme: %v", got)
	}
	before := b.find("brand").cls.list
	if len(before.unknown) != 2 {
		t.Fatalf("color leaked into the second engine: %v", before.unknown)
	}
	if got := UnknownClasses(class); !slices.Equal(got, []string{"bg-engine-brand", "text-(--engine-brand)"}) {
		t.Fatalf("a theme changed the standalone parser: %v", got)
	}

	a.tree.SetEnvironment(Environment{Dark: true})
	a.commit()
	if got := a.find("brand").style.Background; got != RGB(0xabcdef) {
		t.Fatalf("dark brand = %#x", got)
	}
	// Removing a name reparses the live node, without touching B's cache.
	aScope.SetTheme(secondTheme)
	a.commit()
	if got := a.find("brand").style.Background; got != 0 {
		t.Errorf("removed theme's brand remains: %#x", got)
	}
	if got := b.root.rt.parseClasses(&classEnv{theme: secondTheme}, class); got != before {
		t.Error("setting A's theme discarded B's cached classes")
	}
	// A formerly unknown class can become known in that graph alone.
	bScope.SetTheme(firstTheme)
	b.commit()
	if got := b.find("brand").style.Background; got != RGB(0x123456) {
		t.Errorf("a new theme did not resolve the previously unknown color: %#x", got)
	}
}

func TestGraphParseCachesAreBounded(t *testing.T) {
	a, b := NewRoot(RootOptions{}), NewRoot(RootOptions{})
	defer a.Dispose()
	defer b.Dispose()
	theme := a.rt.defaultTheme()
	kept := b.rt.parseClasses(&classEnv{theme: theme}, "p-4")
	for i := range 4 * parseCacheSize {
		class := fmt.Sprintf("w-[%dpx]", i)
		l := a.rt.parseClasses(&classEnv{theme: theme}, class)
		if len(l.unknown) != 0 {
			t.Fatalf("generated class did not parse: %s", class)
		}
		if _, err := a.rt.parseTemplate(GridTracks(fmt.Sprintf("%dpx 1fr", i))); err != nil {
			t.Fatal(err)
		}
		a.rt.reported.put(class, true)
		for name, size := range map[string]int{
			"classes": len(a.rt.classCache.now) + len(a.rt.classCache.old),
			"grids":   len(a.rt.templates.now) + len(a.rt.templates.old),
			"reports": len(a.rt.reported.now) + len(a.rt.reported.old),
		} {
			if size > 2*parseCacheSize {
				t.Fatalf("%s cache grew to %d entries", name, size)
			}
		}
	}
	if b.rt.parseClasses(&classEnv{theme: theme}, "p-4") != kept {
		t.Error("another graph's eviction discarded the cached class")
	}
	if _, ok := a.rt.classCache.get(classSource{theme.tailwind(), theme, 0, "w-[0px]"}); ok {
		t.Error("the oldest generated class was never evicted")
	}
	if _, ok := a.rt.templates.get("0px 1fr"); ok {
		t.Error("the oldest grid template was never evicted")
	}
}

func TestParseCachePromotesOldEntries(t *testing.T) {
	var cache parseCache[int, int]
	cache.put(-1, 42)
	for i := range parseCacheSize {
		cache.put(i, i)
	}
	if v, ok := cache.get(-1); !ok || v != 42 {
		t.Fatal("an old entry was lost before promotion")
	}
	for i := parseCacheSize; i < 2*parseCacheSize; i++ {
		cache.put(i, i)
	}
	if v, ok := cache.get(-1); !ok || v != 42 {
		t.Fatal("a promoted entry did not survive generation rotation")
	}
}

// A host token's color name is a class name in that host's app alone, and only while the host gives it.
func TestHostTokenColorNamesBelongToTheirHost(t *testing.T) {
	const class = "bg-host-brand"
	app := func(*Scope) Node { return View{TestID: "brand", Class: class} }
	a := newEmbedded(t, NewHost(), app)
	b := newEmbedded(t, NewHost(), app)
	a.commit()
	b.commit()
	if got := a.find("brand").cls.list.unknown; len(got) != 1 {
		t.Fatalf("a name no theme or host defines parsed: %v", got)
	}

	a.tree.SetTokens(Tokens{Values: map[string]string{"host-brand": "#123456"}, Dark: map[string]string{"host-brand": "#abcdef"}})
	a.drain()
	a.commit()
	if got := a.find("brand").style.Background; got != RGB(0x123456) {
		t.Fatalf("host token color = %#x", got)
	}
	a.tree.SetEnvironment(Environment{Dark: true})
	a.drain()
	a.commit()
	if got := a.find("brand").style.Background; got != RGB(0xabcdef) {
		t.Fatalf("dark host token color = %#x", got)
	}
	b.commit()
	if got := b.find("brand").cls.list.unknown; len(got) != 1 {
		t.Fatalf("one host's token named a class in another app: %v", got)
	}

	a.tree.SetTokens(Tokens{})
	a.drain()
	a.commit()
	if got := a.find("brand"); got.style.Background != 0 || len(got.cls.list.unknown) != 1 {
		t.Errorf("a withdrawn token still names a class: %#x, unknown %v", got.style.Background, got.cls.list.unknown)
	}
}
