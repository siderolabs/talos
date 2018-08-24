package main

import (
	"log"

	"github.com/autonomy/dianemo/src/initramfs/cmd/proxyd/pkg/frontend"
)

func main() {
	r, err := frontend.NewReverseProxy()
	if err != nil {
		log.Fatalf("failed to initialize the reverse proxy: %v", err)
	}

	// nolint: errcheck
	go r.Watch()

	// nolint: errcheck
	r.Listen(":443")
}

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
}
