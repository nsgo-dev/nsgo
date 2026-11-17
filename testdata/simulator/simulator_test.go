package simulator_test

import (
	"testing"

	_ "github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/nsgotest"
)

func TestMain(m *testing.M) { nsgotest.Main(m) }

func TestFast(t *testing.T)  { t.Log("fast simulator test finished") }
func TestCrash(t *testing.T) { panic("nsgo simulator crash fixture") }
