//go:build darwin && !ios

package macos

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"testing"
)

// mainCalls carries what tests run on the main thread, where init keeps
// TestMain and AppKit wants its views.
var mainCalls = make(chan func())

func TestMain(m *testing.M) {
	done := make(chan int)
	go func() { done <- m.Run() }()
	serveMain(done)
}

func serveMain(done <-chan int) {
	defer func() {
		if value := recover(); value != nil {
			panic(value)
		}
		// Goexit still runs defers. Keep serving on this same goroutine and locked OS thread: replacing it would lose AppKit's thread.
		serveMain(done)
	}()
	for {
		select {
		case fn := <-mainCalls:
			fn()
		case code := <-done:
			os.Exit(code)
		}
	}
}

// onMain runs fn on the main thread and waits for it.
func onMain(fn func()) {
	done := make(chan struct{})
	returned := false
	var failure any
	var stack []byte
	mainCalls <- func() {
		defer func() {
			failure = recover()
			if failure != nil {
				stack = debug.Stack()
			}
			close(done)
		}()
		fn()
		returned = true
	}
	<-done
	if failure != nil {
		fmt.Fprintf(os.Stderr, "panic on the main thread:\n%s", stack)
		panic(failure)
	}
	if !returned {
		// Fatal/FailNow/SkipNow exited the main call; finish the test on its own goroutine too, before it can execute another statement.
		runtime.Goexit()
	}
}
