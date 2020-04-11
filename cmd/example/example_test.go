package main

import (
	"errors"
	"os"
	"testing"

	"github.com/YuriyNasretdinov/hotreload"
)

func TestSoft(t *testing.T) {
	resetMethod(t, false)
}

func TestSoftAll(t *testing.T) {
	resetMethod(t, true)
}

func osOpen(filename string) (*os.File, error) {
	return os.Open(filename)
}

func resetMethod(t *testing.T, all bool) {
	soft.Mock(osOpen, func(filename string) (*os.File, error) {
		return nil, errors.New("Cannot open files!")
	})

	if _, err := osOpen(os.DevNull); err == nil {
		t.Fatalf("Must be error opening dev null!")
	}

	if all {
		soft.ResetAll()
	} else {
		soft.Reset(osOpen)
	}

	if _, err := osOpen(os.DevNull); err != nil {
		t.Fatalf("Must be no errors opening dev null after mock reset!")
	}
}
