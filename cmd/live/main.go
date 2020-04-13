package main

import (
	"log"
	"net/http"

	hot "github.com/YuriyNasretdinov/hotreload"
	"github.com/YuriyNasretdinov/hotreload/cmd/live/subpkg"
)

func main() {
	es := &subpkg.ExampleStruct{}

	http.HandleFunc("/increment", es.IncrementCounter)
	http.HandleFunc("/get", es.GetCounter)

	go hot.ReloaderLoop()

	log.Fatal(http.ListenAndServe(":8080", nil))
}
