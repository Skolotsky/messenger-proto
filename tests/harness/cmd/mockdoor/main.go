// Command mockdoor serves the mock door for building the web app.
package main

import (
	"log"
	"net/http"

	"contexia/tests/harness/mockdoor"
)

func main() {
	d := mockdoor.New(1, 400)
	log.Print("mock door on :8090")
	log.Fatal(http.ListenAndServe(mockdoor.Addr(8090), d.Handler()))
}
