package subpkg

import (
	"fmt"
	"net/http"
	"sync"
)

// ExampleStruct is an example struct of some "web server"
type ExampleStruct struct {
	sync.Mutex
	Counter int64
}

// IncrementCounter increments some counter but does not return the value.
func (e *ExampleStruct) IncrementCounter(w http.ResponseWriter, r *http.Request) {
	e.Lock()
	defer e.Unlock()

	e.Counter++

	w.Write([]byte("Incremented!"))
}

// GetCounter increments some counter but does not return the value.
func (e *ExampleStruct) GetCounter(w http.ResponseWriter, r *http.Request) {
	e.Lock()
	defer e.Unlock()

	fmt.Fprintf(w, "Counter: %d", e.Counter)
}
