package main

import (
	"os"
	"testing"

	"github.com/nsgo-dev/nsgo/nsgotest"
)

// TestMain writes each test's name and then, deferred after m.Run, the teardown's to the file NSGO_TEARDOWN_MARK names, and with NSGO_TEARDOWN_CODE set ends the process with 3 though the tests pass.
func TestMain(m *testing.M) {
	nsgotest.MainFunc(func() int {
		defer mark("teardown")
		code := m.Run()
		if os.Getenv("NSGO_TEARDOWN_CODE") != "" {
			return 3
		}
		return code
	})
}

// mark writes nothing without NSGO_TEARDOWN_MARK, as when the tool lists the tests with go test -list, which runs TestMain too.
func mark(s string) {
	path := os.Getenv("NSGO_TEARDOWN_MARK")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s + "\n"); err != nil {
		panic(err)
	}
}

func TestFirst(t *testing.T) {
	nsgotest.Start(t, Root).Find("text:Hello")
	mark("TestFirst")
}

func TestSecond(t *testing.T) {
	nsgotest.Start(t, Root).Find("text:Hello")
	mark("TestSecond")
}
