//go:build nsgodev || nsgotest || nsgocheck

package nsgotest

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nsgo-dev/nsgo/core"
)

func TestSignalOwnershipNamesPost(t *testing.T) {
	var cx *core.Scope
	var signal *core.Signal[string]
	app := Start(t, func(scope *core.Scope) core.Node {
		cx, signal = scope, core.NewSignal(scope, "before")
		return core.Text{ContentFunc: signal.Get}
	})
	result := make(chan any, 1)
	go func() {
		defer func() { result <- recover() }()
		signal.Set("wrong goroutine")
	}()
	if r := <-result; r == nil || !strings.Contains(fmt.Sprint(r), "Scope.Post") {
		t.Fatalf("wrong-goroutine Set: %v", r)
	}
	done := make(chan struct{})
	go func() { cx.Post(func() { signal.Set("posted") }); close(done) }()
	<-done
	app.WaitFor("text:posted", time.Second)
}
