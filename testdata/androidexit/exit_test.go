package androidexit_test

import (
	"os"
	"testing"

	_ "github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) {
	if os.Getenv("NSGO_EXIT_BEFORE_HOST") == "1" {
		os.Exit(0)
	}
	nsgotest.Main(m)
}

func TestPass(t *testing.T)  { t.Log("normal test finished") }
func TestCrash(t *testing.T) { panic("android exit fixture") }
func TestExit(t *testing.T)  { os.Exit(1) }
