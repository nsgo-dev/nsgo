//go:build ios

// The iOS share extension is Go's code as a static library in an executable that enters NSExtensionMain, with a runtime-registered UIKit principal class. It calls only extension-safe public APIs, through purego.
package shareextension

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/nsgo-dev/nsgo/internal/platform/objcblock"
	"github.com/nsgo-dev/nsgo/internal/platform/shareinbox"
)

var sel = objc.RegisterName
var ns = shareinbox.String
var text = shareinbox.GoString
var loadTypes = sync.OnceFunc(func() {
	if _, err := purego.Dlopen("/System/Library/Frameworks/UniformTypeIdentifiers.framework/UniformTypeIdentifiers", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		panic(err)
	}
})

// Register registers the extension's principal class, NSGoShareController. The extension's executable is this code as a static library linked with _NSExtensionMain as its entry point, as App Store Connect requires (#2718): Go's runtime starts from the library's initializer on a thread of its own while the main thread enters NSExtensionMain, and the generated main package's init calls Register, before UIKit looks the class up for the host's first request.
func Register() {
	uikit()
	loadTypes()
	if _, err := objc.RegisterClass("NSGoShareController", objc.GetClass("UIViewController"), []*objc.Protocol{}, []objc.FieldDef{}, []objc.MethodDef{
		{Cmd: sel("viewDidLoad"), Fn: loaded},
		{Cmd: sel("viewDidAppear:"), Fn: appeared},
		{Cmd: sel("viewDidDisappear:"), Fn: disappeared},
		{Cmd: sel("nsgoClose:"), Fn: closePressed},
	}); err != nil {
		panic(err)
	}
}

// controllers holds each controller's request, from viewDidLoad until they complete.
var controllers sync.Map // objc.ID → *request

func loaded(self objc.ID, _ objc.SEL) {
	objc.SendSuper[objc.ID](self, sel("viewDidLoad"))
	sheet := newSheet(self)
	context := self.Send(sel("extensionContext"))
	context.Send(sel("retain"))
	r := newRequest(sheet.show, func(err error) {
		controllers.Delete(self)
		finish(context, err)
	})
	controllers.Store(self, r)
	receive(context, r.done, r.stopped)
}

func appeared(self objc.ID, _ objc.SEL, animated bool) {
	objc.SendSuper[objc.ID](self, sel("viewDidAppear:"), animated)
	if r, ok := controllers.Load(self); ok {
		r.(*request).appear()
	}
}

// disappeared completes a request whose view the host took away before it did, so the share sheet does not wait on it.
func disappeared(self objc.ID, _ objc.SEL, animated bool) {
	objc.SendSuper[objc.ID](self, sel("viewDidDisappear:"), animated)
	if r, ok := controllers.Load(self); ok {
		r.(*request).close()
	}
}

func closePressed(self objc.ID, _ objc.SEL, _ objc.ID) {
	if r, ok := controllers.Load(self); ok {
		r.(*request).close()
	}
}

func receive(context objc.ID, done func(error), stopped func() bool) {
	receiveInto(context, shareinbox.GroupPath(), shareinbox.Info(shareinbox.TypesKey), done, stopped)
}

// receiveInto separates provider import from discovering the containing app's shared container. The same importer can be tested with isolated storage. It calls done once with the import's result, unless stopped reports the request has ended, when it saves nothing.
func receiveInto(context objc.ID, group string, accepted objc.ID, done func(error), stopped func() bool) {
	loadTypes()
	if group == "" {
		done(errors.New("no app group"))
		return
	}
	batch, err := shareinbox.Begin(group)
	if err != nil {
		done(err)
		return
	}
	items := context.Send(sel("inputItems"))
	type itemLoad struct {
		provider objc.ID
		uti      string
		file     bool
	}
	var loads []itemLoad
	for i := uint(0); i < objc.Send[uint](items, sel("count")); i++ {
		attachments := items.Send(sel("objectAtIndex:"), i).Send(sel("attachments"))
		for j := uint(0); j < objc.Send[uint](attachments, sel("count")); j++ {
			provider := attachments.Send(sel("objectAtIndex:"), j)
			for k := uint(0); k < objc.Send[uint](accepted, sel("count")); k++ {
				uti := accepted.Send(sel("objectAtIndex:"), k)
				if objc.Send[bool](provider, sel("hasItemConformingToTypeIdentifier:"), uti) {
					provider.Send(sel("retain"))
					loads = append(loads, itemLoad{provider: provider, uti: representationType(provider, uti), file: !inlineType(uti) || objc.Send[int](provider, sel("preferredPresentationStyle")) == 2})
					break
				}
			}
		}
	}
	// Load sequentially so provider callbacks preserve the sender's item order.
	var load func(int)
	load = func(index int) {
		if index == len(loads) {
			if stopped() {
				batch.Abort()
				return
			}
			err := batch.Commit()
			batch.Abort()
			if err == nil {
				shareinbox.Notify()
			}
			done(err)
			return
		}
		item := loads[index]
		// fail ends the import at this item: with err, or saving nothing once the request has stopped.
		fail := func(err error) {
			for _, remaining := range loads[index+1:] {
				remaining.provider.Send(sel("release"))
			}
			batch.Abort()
			if !stopped() {
				done(err)
			}
		}
		var once sync.Once
		timeout := time.AfterFunc(loadTimeout, func() {
			// The provider keeps its own references while it loads; a late answer finds once done and is dropped.
			once.Do(func() {
				item.provider.Send(sel("release"))
				fail(fmt.Errorf("%w: %s within %v", errTimedOut, item.uti, loadTimeout))
			})
		})
		callback := objcblock.New(func(_ objc.Block, value, failure objc.ID) {
			once.Do(func() {
				timeout.Stop()
				var err error
				switch {
				case stopped():
					err = errCancelled
				case failure != 0 || value == 0:
					err = fmt.Errorf("provider could not load %s: %s", item.uti, text(failure.Send(sel("description"))))
				default:
					err = copyItem(batch, value, item.uti)
				}
				item.provider.Send(sel("release"))
				if err != nil {
					fail(err)
				} else {
					load(index + 1)
				}
			})
		})
		uti := objc.ID(objc.GetClass("UTType")).Send(sel("typeWithIdentifier:"), ns(item.uti))
		url := objc.ID(objc.GetClass("UTType")).Send(sel("typeWithIdentifier:"), ns("public.url"))
		isURL := objc.Send[bool](uti, sel("conformsToType:"), url)
		// A file-URL representation describes the source file. Decode the NSURL itself, rather than writing its URL descriptor to a file.
		if item.file && !isURL {
			item.provider.Send(sel("loadFileRepresentationForTypeIdentifier:completionHandler:"), ns(item.uti), callback)
		} else {
			class := objc.GetClass("NSString")
			if isURL {
				class = objc.GetClass("NSURL")
			}
			// Ask for the public object class when available. loadItem's generic NSData form may be an archive of a URL, rather than its UTF-8 bytes; loadObject performs the provider's decoding.
			if objc.Send[bool](item.provider, sel("canLoadObjectOfClass:"), class) {
				item.provider.Send(sel("loadObjectOfClass:completionHandler:"), class, callback)
			} else {
				item.provider.Send(sel("loadItemForTypeIdentifier:options:completionHandler:"), ns(item.uti), objc.ID(0), callback)
			}
		}
		callback.Release()
	}
	load(0)
}

func inlineType(identifier objc.ID) bool {
	class := objc.ID(objc.GetClass("UTType"))
	uti := class.Send(sel("typeWithIdentifier:"), identifier)
	for _, kind := range []string{"public.url", "public.text"} {
		if objc.Send[bool](uti, sel("conformsToType:"), class.Send(sel("typeWithIdentifier:"), ns(kind))) {
			return true
		}
	}
	return false
}

// Prefer a registered concrete representation over a broad accepted UTI so file providers retain their real media type and preferred extension.
func representationType(provider objc.ID, accepted objc.ID) string {
	class := objc.ID(objc.GetClass("UTType"))
	wanted := class.Send(sel("typeWithIdentifier:"), accepted)
	registered := provider.Send(sel("registeredTypeIdentifiers"))
	for i := uint(0); i < objc.Send[uint](registered, sel("count")); i++ {
		identifier := registered.Send(sel("objectAtIndex:"), i)
		candidate := class.Send(sel("typeWithIdentifier:"), identifier)
		if objc.Send[bool](candidate, sel("conformsToType:"), wanted) {
			return text(identifier)
		}
	}
	return text(accepted)
}

func copyItem(batch *shareinbox.Batch, value objc.ID, uti string) error {
	typeInfo := objc.ID(objc.GetClass("UTType")).Send(sel("typeWithIdentifier:"), ns(uti))
	mime := text(typeInfo.Send(sel("preferredMIMEType")))
	if objc.Send[bool](value, sel("isKindOfClass:"), objc.GetClass("NSData")) {
		if objc.Send[uint](value, sel("length")) > shareinbox.MaxTextBytes {
			return fmt.Errorf("%w: shared text", shareinbox.ErrTooLarge)
		}
		isText := objc.Send[bool](typeInfo, sel("conformsToType:"), objc.ID(objc.GetClass("UTType")).Send(sel("typeWithIdentifier:"), ns("public.text")))
		isURL := objc.Send[bool](typeInfo, sel("conformsToType:"), objc.ID(objc.GetClass("UTType")).Send(sel("typeWithIdentifier:"), ns("public.url")))
		if !isText && !isURL {
			return errors.New("unsupported shared data representation")
		}
		decoded := objc.ID(objc.GetClass("NSString")).Send(sel("alloc")).Send(sel("initWithData:encoding:"), value, uint(4))
		if decoded == 0 {
			return errors.New("shared text is not UTF-8")
		}
		defer decoded.Send(sel("release"))
		return storeText(batch, decoded, mime)
	}
	if objc.Send[bool](value, sel("isKindOfClass:"), objc.GetClass("NSString")) {
		if mime == "" {
			mime = "text/plain"
		}
		return storeText(batch, value, mime)
	}
	if objc.Send[bool](value, sel("isKindOfClass:"), objc.GetClass("NSURL")) {
		if !objc.Send[bool](value, sel("isFileURL")) {
			return storeText(batch, value.Send(sel("absoluteString")), "text/plain")
		}
		scoped := objc.Send[bool](value, sel("startAccessingSecurityScopedResource"))
		if scoped {
			defer value.Send(sel("stopAccessingSecurityScopedResource"))
		}
		path := text(value.Send(sel("path")))
		// Unscoped provider representations must be temporary copies, never a sender-chosen URL into the extension's App Group or private storage.
		temporary, tmpErr := filepath.EvalSymlinks(os.TempDir())
		resolved, pathErr := filepath.EvalSymlinks(path)
		rel, err := filepath.Rel(temporary, resolved)
		if tmpErr != nil || pathErr != nil {
			return errors.New("shared file is unavailable")
		}
		if !scoped && (err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel)) {
			return errors.New("shared file has no read grant")
		}
		if ext := strings.TrimPrefix(filepath.Ext(path), "."); ext != "" {
			actual := objc.ID(objc.GetClass("UTType")).Send(sel("typeWithFilenameExtension:"), ns(ext))
			if preferred := text(actual.Send(sel("preferredMIMEType"))); preferred != "" {
				mime = preferred
			}
		}
		if mime == "" {
			mime = "application/octet-stream"
		}
		return batch.File(path, filepath.Base(path), mime)
	}
	return errors.New("unsupported shared item representation")
}

func storeText(batch *shareinbox.Batch, value objc.ID, mime string) error {
	if objc.Send[uint](value, sel("lengthOfBytesUsingEncoding:"), uint(4)) > shareinbox.MaxTextBytes {
		return fmt.Errorf("%w: shared text", shareinbox.ErrTooLarge)
	}
	return batch.Text(text(value), mime)
}

func finish(context objc.ID, err error) {
	// UIKit completion belongs on its main operation queue, including callbacks from providers that perform their work on a background queue.
	block := objcblock.New(func(objc.Block) {
		if err == nil {
			context.Send(sel("completeRequestReturningItems:completionHandler:"), objc.ID(0), objc.ID(0))
		} else {
			// NSUserCancelledError for a request the user closed, which hosts do not report as a failure; NSFileReadUnknownError otherwise.
			code := 256
			if errors.Is(err, errCancelled) {
				code = 3072
			}
			info := objc.ID(objc.GetClass("NSDictionary")).Send(sel("dictionaryWithObject:forKey:"), ns(err.Error()), ns("NSDebugDescription"))
			failure := objc.ID(objc.GetClass("NSError")).Send(sel("errorWithDomain:code:userInfo:"), ns("NSCocoaErrorDomain"), code, info)
			context.Send(sel("cancelRequestWithError:"), failure)
		}
		context.Send(sel("release"))
	})
	objc.ID(objc.GetClass("NSOperationQueue")).Send(sel("mainQueue")).Send(sel("addOperationWithBlock:"), block)
	block.Release()
}
