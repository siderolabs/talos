// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/http/httpproxy"
	"golang.org/x/sys/unix"

	v1alpha1server "github.com/talos-systems/talos/internal/app/machined/internal/server/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/acpi"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/universe"
)

func init() {
	// Explicitly set the default http client transport to work around proxy.Do
	// once. This is the http.DefaultTransport with the Proxy func overridden so
	// that the environment variables with be reread/initialized each time the
	// http call is made.
	http.DefaultClient.Transport = &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
		},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// See http://man7.org/linux/man-pages/man2/reboot.2.html.
func sync() {
	syncdone := make(chan struct{})

	go func() {
		defer close(syncdone)

		unix.Sync()
	}()

	log.Printf("waiting for sync...")

	for i := 29; i >= 0; i-- {
		select {
		case <-syncdone:
			log.Printf("sync done")
			return
		case <-time.After(time.Second):
		}

		if i != 0 {
			log.Printf("waiting %d more seconds for sync to finish", i)
		}
	}

	log.Printf("sync hasn't completed in time, aborting...")
}

func reboot(err error) {
	log.Print(err)

	for i := 10; i >= 0; i-- {
		log.Printf("rebooting in %d seconds\n", i)
		time.Sleep(1 * time.Second)
	}

	sync()

	// nolint: errcheck
	unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)
}

func main() {
	p, err := platform.CurrentPlatform()
	if err != nil {
		reboot(err)
	}

	r := runtime.NewRuntime(p, nil, runtime.Initialize)

	s := &v1alpha1runtime.Sequencer{}

	controller := &runtime.Controller{
		Runtime:   r,
		Sequencer: s,
	}

	if p.Mode() != runtime.Container {
		go func() {
			if e := acpi.StartACPIListener(); e != nil {
				log.Printf("WARNING: ACPI events will be ignored: %+v", err)

				return
			}

			log.Printf("shutdown via ACPI received")

			if e := controller.Run(runtime.Shutdown, nil); e != nil {
				reboot(e)
			}
		}()
	} else {
		termCh := make(chan os.Signal, 1)
		signal.Notify(termCh, syscall.SIGTERM)

		go func() {
			<-termCh
			signal.Stop(termCh)

			log.Printf("shutdown via SIGTERM received")

			if e := controller.Run(runtime.Shutdown, nil); e != nil {
				reboot(e)
			}
		}()
	}

	if err = controller.Run(runtime.Initialize, nil); err != nil {
		reboot(err)
	}

	go func() {
		server := &v1alpha1server.Server{
			Controller: controller,
		}

		if e := factory.ListenAndServe(server, factory.Network("unix"), factory.SocketPath(universe.MachineSocketPath)); e != nil {
			reboot(e)
		}
	}()

	cfg, err := config.NewFromFile(constants.ConfigPath)
	if err != nil {
		reboot(fmt.Errorf("failed to parse config: %w", err))
	}

	r = runtime.NewRuntime(p, cfg, runtime.Initialize)
	controller.Runtime = r

	if err = controller.Run(runtime.Boot, nil); err != nil {
		reboot(err)
	}

	select {}
}
