package hot

import (
	"bufio"
	"log"
	"os"
	"plugin"
	"strings"
)

// ReloaderLoop starts a loop that loads new plugins
// and applies patches to existing functions.
// Suggested usage: `go hot.ReloaderLoop()`
func ReloaderLoop() {
	r := bufio.NewReader(os.Stdin)

	for {
		ln, err := r.ReadString('\n')
		if err != nil {
			log.Fatalf("hot.Reloader failed reading from stdin: %v", err)
		}

		plugPath := strings.TrimRight(ln, "\n")

		log.Printf("Opening plugin %s", plugPath)
		plug, err := plugin.Open(plugPath)
		if err != nil {
			log.Fatalf("Couldn't open the plugin: %v", err)
		}
		sym, err := plug.Lookup("Mock")
		if err != nil {
			log.Fatalf("Couldn't open the symbol Mock: %v", err)
		}

		log.Printf("Calling Mock() from a plugin")
		sym.(func())()
		log.Printf("Hot reload was successful")
	}
}
