//go:build ios

package shareextension

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
	_ "github.com/nsgo-dev/nsgo" // Register the UIKit test host for this library package.
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/internal/platform/apple"
	"github.com/nsgo-dev/nsgo/internal/platform/objcblock"
	"github.com/nsgo-dev/nsgo/internal/platform/shareinbox"
	"github.com/nsgo-dev/nsgo/mobile"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func mainThread(fn func()) {
	done := make(chan struct{})
	apple.OnMain(func() { fn(); close(done) })
	<-done
}

type inputContext struct {
	items objc.ID
	done  chan error
}

var contexts = map[objc.ID]inputContext{}
var contextClass objc.Class
var contextOnce sync.Once

// TestNativeProvidersReachTheContainingApp uses Foundation's real item providers and App Group storage, including the Darwin wakeup and engine callback. It does not pretend to drive Safari's share sheet.
func TestNativeProvidersReachTheContainingApp(t *testing.T) {
	received := make(chan []mobile.SharedItem, 1)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		mobile.OnShared(cx, func(items []mobile.SharedItem) { received <- items })
		return core.View{}
	})
	_ = app
	file := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(file, []byte("shared file contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	importProviders(t, file, "")
	select {
	case items := <-received:
		if len(items) != 3 || items[0].Text != "https://example.com/shared" || items[1].Path == "" || items[2].Text != "https://example.com/plain-text" {
			t.Fatalf("shared items: %#v", items)
		}
		data, err := os.ReadFile(items[1].Path)
		if err != nil || string(data) != "shared file contents" {
			t.Fatalf("shared file = %q, %v", data, err)
		}
		if items[1].Name != "note.txt" {
			t.Errorf("name = %q", items[1].Name)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("containing app did not receive the share")
	}
}

func TestNativeURLAndFileCopies(t *testing.T) {
	group := t.TempDir()
	batch, err := shareinbox.Begin(group)
	if err != nil {
		t.Fatal(err)
	}
	defer batch.Abort()
	path := filepath.Join(t.TempDir(), "document.pdf")
	os.WriteFile(path, []byte("pdf bytes"), 0o600)
	var copyError error
	mainThread(func() {
		// UniformTypeIdentifiers is not needed until the extension imports items.
		_ = apple.UTType()
		url := objc.ID(objc.GetClass("NSURL")).Send(sel("URLWithString:"), ns("https://example.com"))
		if copyError = copyItem(batch, url, "public.url"); copyError != nil {
			return
		}
		file := objc.ID(objc.GetClass("NSURL")).Send(sel("fileURLWithPath:"), ns(path))
		copyError = copyItem(batch, file, "com.adobe.pdf")
	})
	if copyError != nil {
		t.Fatal(copyError)
	}
	if err := batch.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeUnscopedPrivateFilesAreRefused(t *testing.T) {
	cache, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(cache, "nsgo655-private-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "private.txt")
	if err := os.WriteFile(path, []byte("not shared"), 0o600); err != nil {
		t.Fatal(err)
	}
	batch, err := shareinbox.Begin(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer batch.Abort()
	var denied error
	mainThread(func() {
		_ = apple.UTType()
		file := objc.ID(objc.GetClass("NSURL")).Send(sel("fileURLWithPath:"), ns(path))
		denied = copyItem(batch, file, "public.plain-text")
	})
	if denied == nil {
		t.Fatal("a sender-chosen private file URL was read without a grant")
	}
}

func TestNativeUTF8DataIsDeliveredAsText(t *testing.T) {
	group, cache := t.TempDir(), t.TempDir()
	batch, err := shareinbox.Begin(group)
	if err != nil {
		t.Fatal(err)
	}
	defer batch.Abort()
	var failure error
	mainThread(func() {
		_ = apple.UTType()
		data := apple.NSData([]byte("shared\x00UTF-8 text"))
		failure = copyItem(batch, data, "public.utf8-plain-text")
	})
	if failure != nil {
		t.Fatal(failure)
	}
	if err := batch.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := shareinbox.NewReader(group, cache).Drain(func(items []mobile.SharedItem, ack func() error) {
		if len(items) != 1 || items[0].Text != "shared\x00UTF-8 text" {
			t.Fatalf("decoded text: %#v", items)
		}
		if err := ack(); err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
}

// cancelled is a request the extension cancelled, with its NSError's code and description.
type cancelled struct {
	code int
	text string
}

func (c cancelled) Error() string { return fmt.Sprintf("cancelled (%d): %s", c.code, c.text) }

func registerContext() {
	contextOnce.Do(func() {
		var err error
		contextClass, err = objc.RegisterClass("NSGoShareTestContext", objc.GetClass("NSObject"), []*objc.Protocol{}, []objc.FieldDef{}, []objc.MethodDef{
			{Cmd: sel("inputItems"), Fn: func(self objc.ID, _ objc.SEL) objc.ID { return contexts[self].items }},
			{Cmd: sel("completeRequestReturningItems:completionHandler:"), Fn: func(self objc.ID, _ objc.SEL, _, _ objc.ID) { contexts[self].done <- nil }},
			{Cmd: sel("cancelRequestWithError:"), Fn: func(self objc.ID, _ objc.SEL, failure objc.ID) {
				contexts[self].done <- cancelled{code: objc.Send[int](failure, sel("code")), text: text(failure.Send(sel("userInfo")).Send(sel("objectForKey:"), ns("NSDebugDescription")))}
			}},
		})
		if err != nil {
			panic(err)
		}
	})
}

// newContext is a fake NSExtensionContext holding providers, whose completion or cancellation it sends to done. It has one retain for finish to release, and t's cleanup releases the rest.
func newContext(t *testing.T, providers []objc.ID, done chan error) objc.ID {
	var context objc.ID
	mainThread(func() {
		registerContext()
		array := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
		for _, provider := range providers {
			array.Send(sel("addObject:"), provider)
		}
		item := objc.ID(objc.GetClass("NSExtensionItem")).Send(sel("new"))
		item.Send(sel("setAttachments:"), array)
		items := objc.ID(objc.GetClass("NSArray")).Send(sel("arrayWithObject:"), item)
		items.Send(sel("retain"))
		item.Send(sel("release"))
		context = objc.ID(contextClass).Send(sel("new"))
		contexts[context] = inputContext{items: items, done: done}
		context.Send(sel("retain"))
	})
	t.Cleanup(func() {
		mainThread(func() {
			contexts[context].items.Send(sel("release"))
			delete(contexts, context)
			context.Send(sel("release"))
		})
	})
	return context
}

// acceptedTypes are the test extension's apple.share_types.
func acceptedTypes() objc.ID {
	types := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
	for _, uti := range []string{"public.url", "public.plain-text", "public.data"} {
		types.Send(sel("addObject:"), ns(uti))
	}
	return types
}

// finishing completes or cancels context with the import's result, as a request does once its view has appeared.
func finishing(context objc.ID) func(error) { return func(err error) { finish(context, err) } }

func running() bool { return false }

func importProviders(t *testing.T, file, group string) {
	t.Helper()
	done := make(chan error, 1)
	var providers []objc.ID
	mainThread(func() {
		array := objc.ID(objc.GetClass("NSMutableArray")).Send(sel("array"))
		for _, url := range []objc.ID{
			objc.ID(objc.GetClass("NSURL")).Send(sel("URLWithString:"), ns("https://example.com/shared")),
			objc.ID(objc.GetClass("NSURL")).Send(sel("fileURLWithPath:"), ns(file)),
		} {
			var provider objc.ID
			if objc.Send[bool](url, sel("isFileURL")) {
				provider = objc.ID(objc.GetClass("NSItemProvider")).Send(sel("alloc")).Send(sel("initWithContentsOfURL:"), url)
				provider.Send(sel("setPreferredPresentationStyle:"), int(2))
			} else {
				provider = objc.ID(objc.GetClass("NSItemProvider")).Send(sel("alloc")).Send(sel("initWithObject:"), url)
			}
			array.Send(sel("addObject:"), provider)
			provider.Send(sel("release"))
		}
		plain := objc.ID(objc.GetClass("NSItemProvider")).Send(sel("alloc")).Send(sel("initWithObject:"), ns("https://example.com/plain-text"))
		array.Send(sel("addObject:"), plain)
		plain.Send(sel("release"))
		array.Send(sel("retain"))
		for i := range objc.Send[uint](array, sel("count")) {
			providers = append(providers, array.Send(sel("objectAtIndex:"), i))
		}
		t.Cleanup(func() { mainThread(func() { array.Send(sel("release")) }) })
	})
	context := newContext(t, providers, done)
	mainThread(func() {
		if group == "" {
			receive(context, finishing(context), running)
		} else {
			receiveInto(context, group, acceptedTypes(), finishing(context), running)
		}
	})
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("extension import failed: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("provider did not complete")
	}
}

// TestNativeProvidersStageURLAndFile uses real NSItemProvider callbacks with isolated storage; it requires no App Group profile and does not replace the separate containing-app integration test.
func TestNativeProvidersStageURLAndFile(t *testing.T) {
	group, cache := t.TempDir(), t.TempDir()
	file := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(file, []byte("shared file contents"), 0o600); err != nil {
		t.Fatal(err)
	}
	importProviders(t, file, group)
	called := false
	if err := shareinbox.NewReader(group, cache).Drain(func(items []mobile.SharedItem, ack func() error) {
		called = true
		if len(items) != 3 || items[0].Text != "https://example.com/shared" || items[1].Name != "note.txt" || items[2].Text != "https://example.com/plain-text" {
			t.Fatalf("provider items: %#v", items)
		}
		data, err := os.ReadFile(items[1].Path)
		if err != nil || string(data) != "shared file contents" {
			t.Fatalf("provider file: %q, %v", data, err)
		}
		if items[1].MIME != "text/plain" {
			t.Errorf("file MIME = %q", items[1].MIME)
		}
		if err := ack(); err != nil {
			t.Fatal(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("provider callback did not commit a share")
	}
}

func TestNativeReceiveShareSimulation(t *testing.T) {
	received := make(chan []mobile.SharedItem, 1)
	app := nsgotest.Start(t, func(cx *core.Scope) core.Node {
		mobile.OnShared(cx, func(items []mobile.SharedItem) { received <- items })
		return core.View{}
	})
	want := []mobile.SharedItem{{Text: "https://example.com/simulated", MIME: "text/plain"}, {Path: "/cache/simulated.pdf", Name: "simulated.pdf", MIME: "application/pdf"}}
	app.ReceiveShare(want...)
	select {
	case got := <-received:
		if !slices.Equal(got, want) {
			t.Fatalf("simulated items: %#v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("iOS test host did not deliver the simulated share")
	}
}

// TestNativeSilentProviderTimesOut gives the extension a provider that never answers, which before kept the share sheet on its spinner until the host was killed: the import fails after loadTimeout, the request is cancelled with an error, and nothing reaches the inbox.
func TestNativeSilentProviderTimesOut(t *testing.T) {
	defer func(d time.Duration) { loadTimeout = d }(loadTimeout)
	loadTimeout = 300 * time.Millisecond
	group, cache := t.TempDir(), t.TempDir()
	var provider objc.ID
	mainThread(func() {
		provider = objc.ID(objc.GetClass("NSItemProvider")).Send(sel("new"))
		silent := objcblock.New(func(_ objc.Block, _, _, _ objc.ID) {})
		provider.Send(sel("registerItemForTypeIdentifier:loadHandler:"), ns("public.plain-text"), silent)
		silent.Release()
	})
	done := make(chan error, 1)
	context := newContext(t, []objc.ID{provider}, done)
	mainThread(func() {
		provider.Send(sel("release"))
		receiveInto(context, group, acceptedTypes(), finishing(context), running)
	})
	select {
	case err := <-done:
		var c cancelled
		if !errors.As(err, &c) || c.code != 256 || !strings.Contains(c.text, errTimedOut.Error()) {
			t.Fatalf("request ended with %v, want a cancellation for the provider's timeout", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a provider that never answers left the request open")
	}
	if err := shareinbox.NewReader(group, cache).Drain(func([]mobile.SharedItem, func() error) { t.Fatal("a timed-out import was delivered") }); err != nil {
		t.Fatal(err)
	}
}

// TestNativeTooLargeShareIsCancelled shares text over the inbox's limit: the request is cancelled with the error, which the sheet shows as too large.
func TestNativeTooLargeShareIsCancelled(t *testing.T) {
	var provider objc.ID
	mainThread(func() {
		provider = objc.ID(objc.GetClass("NSItemProvider")).Send(sel("alloc")).Send(sel("initWithObject:"), ns(strings.Repeat("x", shareinbox.MaxTextBytes+1)))
	})
	done := make(chan error, 1)
	context := newContext(t, []objc.ID{provider}, done)
	var result error
	mainThread(func() {
		provider.Send(sel("release"))
		receiveInto(context, t.TempDir(), acceptedTypes(), func(err error) { result = err; finish(context, err) }, running)
	})
	select {
	case err := <-done:
		var c cancelled
		if !errors.As(err, &c) || c.code != 256 {
			t.Fatalf("request ended with %v, want a cancellation", err)
		}
		if !errors.Is(result, shareinbox.ErrTooLarge) {
			t.Fatalf("import failed with %v, want ErrTooLarge", result)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the request did not end")
	}
}

// TestNativeStoppedImportSavesNothing stops the request before its provider answers, as the user's Cancel does: the import neither reports nor commits.
func TestNativeStoppedImportSavesNothing(t *testing.T) {
	group, cache := t.TempDir(), t.TempDir()
	var provider objc.ID
	mainThread(func() {
		provider = objc.ID(objc.GetClass("NSItemProvider")).Send(sel("alloc")).Send(sel("initWithObject:"), ns("https://example.com/cancelled"))
	})
	done := make(chan error, 1)
	context := newContext(t, []objc.ID{provider}, done)
	reported := make(chan error, 1)
	mainThread(func() {
		provider.Send(sel("release"))
		receiveInto(context, group, acceptedTypes(), func(err error) { reported <- err }, func() bool { return true })
	})
	select {
	case err := <-reported:
		t.Fatalf("a stopped import reported %v", err)
	case <-time.After(time.Second):
	}
	if err := shareinbox.NewReader(group, cache).Drain(func([]mobile.SharedItem, func() error) { t.Fatal("a stopped import was delivered") }); err != nil {
		t.Fatal(err)
	}
	finish(context, errCancelled)
	select {
	case err := <-done:
		var c cancelled
		if !errors.As(err, &c) || c.code != 3072 {
			t.Fatalf("request ended with %v, want NSUserCancelledError", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the request did not end")
	}
}
