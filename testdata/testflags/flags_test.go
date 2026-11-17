// Package testflags links a native host without starting a UI. Its process counter verifies the tool's isolation of host-linked test repetitions.
package testflags

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"testing"

	_ "github.com/nsgo-dev/nsgo"
)

func TestMain(m *testing.M) { os.Exit(m.Run()) }

var calls int

func TestIsolated(t *testing.T) {
	calls++
	if calls != 1 {
		t.Fatal("a native test repetition reused the process")
	}
	f, err := os.OpenFile(os.Getenv("NSGO_FLAGS_MARK"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	// Exercise screenshot transport without starting a native UI.
	fmt.Println("nsgo-screenshot repeated 1/1 c2hvdA==")
	if _, err := f.WriteString(strconv.Itoa(runtime.GOMAXPROCS(0)) + "\n"); err != nil {
		t.Fatal(err)
	}
}
