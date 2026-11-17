//go:build darwin

package objcblock

import (
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

func nativeAddress(block objc.Block) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&block))
}

func TestBlockCopyRetainsItsCallbackAcrossGC(t *testing.T) {
	offset := new(int64)
	*offset = 7
	block := New(func(_ objc.Block, a, b int64) int64 { return a + b + *offset })
	retained := block.Copy()
	block.Release()
	defer retained.Release()
	runtime.GC()
	debug.FreeOSMemory()

	lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_NOW)
	if err != nil {
		t.Fatal(err)
	}
	var mallocSize func(unsafe.Pointer) uintptr
	purego.RegisterLibFunc(&mallocSize, lib, "malloc_size")
	if size := mallocSize(nativeAddress(retained)); size < unsafe.Sizeof(literal{}) {
		t.Fatalf("block has no native allocation: malloc_size=%d", size)
	}
	header := (*literal)(nativeAddress(retained))
	var invoke func(objc.Block, int64, int64) int64
	purego.RegisterFunc(&invoke, header.invoke)
	if got := invoke(retained, 11, 13); got != 31 {
		t.Fatalf("native block invocation = %d, want 31", got)
	}
}

func TestBlockReleaseDropsItsClosure(t *testing.T) {
	block := New(func(objc.Block) {})
	retained := block.Copy()
	block.Release()
	if _, ok := functions.Load(retained); !ok {
		t.Fatal("releasing the first reference dropped a retained callback")
	}
	retained.Release()
	if _, ok := functions.Load(retained); ok {
		t.Fatal("last native release kept the callback alive")
	}
}

func TestFoundationInvokesTheBlockWithAnObjectAndStopPointer(t *testing.T) {
	if _, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	text := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithUTF8String:"), "first\nsecond\n")
	defer text.Send(objc.RegisterName("release"))
	heard := 0
	block := New(func(_ objc.Block, line objc.ID, stop *bool) {
		if line == 0 {
			t.Error("Foundation passed no line")
		}
		heard++
		*stop = true
	})
	defer block.Release()
	runtime.GC()
	text.Send(objc.RegisterName("enumerateLinesUsingBlock:"), block)
	if heard != 1 {
		t.Errorf("Foundation invoked the block %d times, want one before stop", heard)
	}
}

func TestBlockConcurrentCreationUnderGC(t *testing.T) {
	previous := debug.SetGCPercent(1)
	defer debug.SetGCPercent(previous)
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			for i := range 256 {
				done := make(chan struct{})
				go func() {
					defer close(done)
					want := int64(worker*256 + i)
					// Start each creation on a fresh small stack, while the other workers allocate, collect and reuse old stacks.
					block := New(func(_ objc.Block, value int64) int64 { return value + want })
					header := (*literal)(nativeAddress(block))
					var invoke func(objc.Block, int64) int64
					purego.RegisterFunc(&invoke, header.invoke)
					if i%64 == 0 {
						runtime.GC()
					}
					if got := invoke(block, 5); got != want+5 {
						t.Errorf("worker %d iteration %d: got %d, want %d", worker, i, got, want+5)
					}
					block.Release()
				}()
				<-done
			}
		})
	}
	workers.Wait()
}

func TestBlockRejectsNilCopyBeforeItCanReachFoundation(t *testing.T) {
	setup.Do(initialize)
	defer func() {
		message, ok := recover().(string)
		if !ok || !strings.Contains(message, "_Block_copy failed") {
			t.Errorf("allocation failure panic = %v", message)
		}
	}()
	newBlock(func(objc.Block) {}, func(*literal) objc.Block { return 0 })
	t.Fatal("nil block escaped allocation")
}

func TestBlockRejectsInvalidCallbacks(t *testing.T) {
	for _, callback := range []any{nil, 1, (func(objc.Block))(nil), func() {}, func(objc.Block, string) {}} {
		t.Run("invalid", func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("invalid callback was accepted")
				}
			}()
			New(callback)
		})
	}
}
