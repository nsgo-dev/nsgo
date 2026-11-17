package simulatornohost_test

import (
	"testing"

	_ "github.com/nsgo-dev/nsgo"
)

func TestFast(t *testing.T)  { t.Log("fast simulator test without the host finished") }
func TestCrash(t *testing.T) { panic("nsgo simulator crash fixture") }
