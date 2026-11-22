// Command cmd runs the feed stand-in as a program, for trying the whole system
// before the feed service (T-052) is merged. It is never part of the product.
package main

import (
	"log"
	"net/http"

	"contexia/tests/guardrails/rebuild/feedstandin"
)

func main() { log.Fatal(http.ListenAndServe(":8080", feedstandin.New().Routes())) }
