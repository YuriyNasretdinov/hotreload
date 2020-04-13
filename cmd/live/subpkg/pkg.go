package subpkg

import (
	"net/http"
	"strconv"
	"sync"
)

// Counter is a type for demo purposes.
type Counter int64

// ExampleStruct is an example struct of some "web server"
type ExampleStruct struct {
	sync.Mutex
	Counter Counter
}

func (c Counter) String() string {
	return strconv.FormatInt(int64(c), 10)
}

func (e *ExampleStruct) incrementCounter(w http.ResponseWriter, r *http.Request) {
	e.Lock()
	defer e.Unlock()

	e.Counter++

	w.Write([]byte("Incremented!\n"))
	// fmt.Fprintf(w, "Counter: %s", e.Counter)
	// log.Printf("[Incr] Counter value: %s", e.Counter)
}

// IncrementCounter increments some counter but does not return the value.
func (e *ExampleStruct) IncrementCounter(w http.ResponseWriter, r *http.Request) {
	e.incrementCounter(w, r)
}

// GetCounter increments some counter but does not return the value.
func (e *ExampleStruct) GetCounter(w http.ResponseWriter, r *http.Request) {
	e.Lock()
	defer e.Unlock()

	// fmt.Fprintf(w, "Counter: %s", e.Counter)
	// log.Printf("[Get] Counter value: %d", e.Counter)
}
