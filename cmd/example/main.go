package main

import (
	"fmt"
	"os"

	"github.com/YuriyNasretdinov/hotreload"
)

func fpClose(f *os.File) error {
	return f.Close()
}

func main() {
	soft.Mock(fpClose, func(f *os.File) error {
		fmt.Printf("File is going to be closed: %s\n", f.Name())
		res, _ := soft.CallOriginal(fpClose, f)[0].(error)
		return res
	})
	fp, _ := os.Open("/dev/null")
	fmt.Printf("Hello, world: %v!\n", fpClose(fp))
}
