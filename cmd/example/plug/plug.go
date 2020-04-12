package main

import (
	"fmt"
	"os"

	hot "github.com/YuriyNasretdinov/hotreload"
)

// Mock replaces fp.Close with a new implementation.
func Mock() {
	name := "github.com/YuriyNasretdinov/hotreload/cmd/example/fp/Close"
	hot.MockByName(name, func(f *os.File) error {
		fmt.Printf("File is going to be closed: %s\n", f.Name())
		res, _ := hot.CallOriginalByName(name, f)[0].(error)
		return res
	})
}

func main() {
	// main is not required for a plugin
}
