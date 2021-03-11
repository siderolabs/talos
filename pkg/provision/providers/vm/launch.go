// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/talos-systems/talos/pkg/provision/internal/inmemhttp"
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

	return os.Stdin.Close()
}

// ConfigureSignals configures signal handling for the process.
func ConfigureSignals() chan os.Signal {
	signal.Ignore(syscall.SIGHUP)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)

	return c
}

func httpPostWrapper(f func() error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Body != nil {
			_, _ = io.Copy(ioutil.Discard, req.Body) //nolint:errcheck
			req.Body.Close()                         //nolint:errcheck
		}

		if req.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotImplemented)

			return
		}

		err := f()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)

			fmt.Fprint(w, err.Error())

			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

func httpGetWrapper(f func(w io.Writer)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Body != nil {
			_, _ = io.Copy(ioutil.Discard, req.Body) //nolint:errcheck
			req.Body.Close()                         //nolint:errcheck
		}

		switch req.Method {
		case http.MethodHead:
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			f(w)
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	})
}

// NewHTTPServer creates new inmemhttp.Server and mounts config file into it.
func NewHTTPServer(gatewayAddr net.IP, port int, config []byte, controller Controller) (inmemhttp.Server, error) {
	httpServer, err := inmemhttp.NewServer(fmt.Sprintf("%s:%d", gatewayAddr, port))
	if err != nil {
		return nil, fmt.Errorf("error launching in-memory HTTP server: %w", err)
	}

	if err = httpServer.AddFile("config.yaml", config); err != nil {
		return nil, err
	}

	if controller != nil {
		for _, method := range []struct {
			pattern string
			f       func() error
		}{
			{
				pattern: "/poweron",
				f:       controller.PowerOn,
			},
			{
				pattern: "/poweroff",
				f:       controller.PowerOff,
			},
			{
				pattern: "/reboot",
				f:       controller.Reboot,
			},
			{
				pattern: "/pxeboot",
				f:       controller.PXEBootOnce,
			},
		} {
			httpServer.AddHandler(method.pattern, httpPostWrapper(method.f))
		}

		httpServer.AddHandler("/status", httpGetWrapper(func(w io.Writer) {
			json.NewEncoder(w).Encode(controller.Status()) //nolint:errcheck
		}))
	}

	return httpServer, nil
}
