package main

import (
	"log"
	"net/http"

	"github.com/YuriyNasretdinov/hotreload/cmd/live/subpkg"
)

func main() {
	es := &subpkg.ExampleStruct{}

	http.HandleFunc("/increment", es.IncrementCounter)
	http.HandleFunc("/get", es.GetCounter)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
