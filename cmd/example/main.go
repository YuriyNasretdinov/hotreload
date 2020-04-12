package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"plugin"

	"github.com/YuriyNasretdinov/hotreload/cmd/example/fp"
)

var plugPath = flag.String("plug", "", "Path to a plugin")

func main() {
	flag.Parse()

	devNull, _ := os.Open("/dev/null")

	plug, err := plugin.Open(*plugPath)
	if err != nil {
		log.Fatalf("Couldn't open the plugin: %v", err)
	}
	sym, err := plug.Lookup("Mock")
	if err != nil {
		log.Fatalf("Couldn't open the symbol Mock: %v", err)
	}

	log.Printf("Calling Mock() from a plugin")
	sym.(func())()

	fmt.Printf("Hello, world: %v!\n", fp.Close(devNull))
}
