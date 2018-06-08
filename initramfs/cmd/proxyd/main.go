package main

import (
	"log"
)

func main() {
	r, err := NewReverseProxy()
	if err != nil {
		log.Fatalf("failed to initialize the reverse proxy: %v", err)
	}

	// nolint: errcheck
	go r.Watch()

	// nolint: errcheck
	r.Listen(":443")
}
