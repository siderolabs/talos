// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/talos-systems/talos/internal/pkg/inmemhttp"
)

// ReadConfig loads configuration from stdin.
func ReadConfig(config interface{}) error {
	d := json.NewDecoder(os.Stdin)
	if err := d.Decode(config); err != nil {
		return fmt.Errorf("error decoding config from stdin: %w", err)
	}

	if d.More() {
		return fmt.Errorf("extra unexpected input on stdin")
	}

	if err := os.Stdin.Close(); err != nil {
		return err
	}

	return nil
}

// ConfigureSignals configures signal handling for the process.
func ConfigureSignals() chan os.Signal {
	signal.Ignore(syscall.SIGHUP)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)

	return c
}

// NewConfigServer creates new inmemhttp.Server and mounts config file into it.
func NewConfigServer(gatewayAddr net.IP, config []byte) (inmemhttp.Server, error) {
	httpServer, err := inmemhttp.NewServer(fmt.Sprintf("%s:0", gatewayAddr))
	if err != nil {
		return nil, fmt.Errorf("error launching in-memory HTTP server: %w", err)
	}

	if err = httpServer.AddFile("config.yaml", config); err != nil {
		return nil, err
	}

	return httpServer, nil
}
