// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
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

	"github.com/talos-systems/go-procfs/procfs"

	v1alpha1server "github.com/talos-systems/talos/internal/app/machined/internal/server/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/syslinux"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/proc/reaper"
	"github.com/talos-systems/talos/pkg/startup"
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

func recovery() {
	if r := recover(); r != nil {
		var (
			err error
			ok  bool
		)

		err, ok = r.(error)
		if ok {
			handle(err)
		}
	}
}

func handle(err error) {
	if err != nil {
		log.Print(err)
	}

	if err := syslinux.Revert(); err != nil {
		log.Printf("failed to revert upgrade: %v", err)
	}

	if p := procfs.ProcCmdline().Get(constants.KernelParamPanic).First(); p != nil {
		if *p == "0" {
			log.Printf("panic=0 kernel flag found, sleeping forever")

			exitSignal := make(chan os.Signal, 1)

			signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)

			<-exitSignal
		}
	}

	for i := 10; i >= 0; i-- {
		log.Printf("rebooting in %d seconds\n", i)
		time.Sleep(1 * time.Second)
	}

	v1alpha1runtime.SyncNonVolatileStorageBuffers()

	if unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART) == nil {
		// Wait forever.
		select {}
	}
}

// nolint: gocyclo
func main() {
	// Setup panic handler.
	defer recovery()

	// Initialize the process reaper.
	reaper.Run()
	defer reaper.Shutdown()

	// Ensure rng is seeded.
	if err := startup.RandSeed(); err != nil {
		handle(err)
	}

	// Set the PATH env var.
	if err := os.Setenv("PATH", constants.PATH); err != nil {
		handle(errors.New("error setting PATH"))
	}

	// Initialize the controller without a config.
	c, err := v1alpha1runtime.NewController(nil)
	if err != nil {
		handle(err)
	}

	// Initialize the machine.
	if err = c.Run(runtime.SequenceInitialize, nil); err != nil {
		handle(err)
	}

	// Start event listeners.
	go func() {
		if e := c.ListenForEvents(); e != nil {
			log.Printf("WARNING: signals and ACPI events will be ignored: %+v", e)
		}
	}()

	// Start the API server.
	go func() {
		server := &v1alpha1server.Server{
			Controller: c,
		}

		e := factory.ListenAndServe(server, factory.Network("unix"), factory.SocketPath(constants.MachineSocketPath))

		handle(e)
	}()

	// Perform an installation if required.
	if err = c.Run(runtime.SequenceInstall, nil); err != nil {
		handle(err)
	}

	// Boot the machine.
	if err = c.Run(runtime.SequenceBoot, nil); err != nil {
		handle(err)
	}

	// Wait forever.
	select {}
}
