// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
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

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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

	if meta, err := bootloader.NewMeta(); err == nil {
		if err = meta.Revert(); err != nil {
			log.Printf("failed to revert upgrade: %v", err)
		}

		//nolint: errcheck
		meta.Close()
	} else {
		log.Printf("failed to open meta: %v", err)
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

	// Ensure RNG is seeded.
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

	// Start signal and ACPI listeners.
	go func() {
		if e := c.ListenForEvents(); e != nil {
			log.Printf("WARNING: signals and ACPI events will be ignored: %+v", e)
		}
	}()

	// Initialize the machine.
	if err = c.Run(runtime.SequenceInitialize, nil); err != nil {
		handle(err)
	}

	// Perform an installation if required.
	if err = c.Run(runtime.SequenceInstall, nil); err != nil {
		handle(err)
	}

	// Start the machine API.
	system.Services(c.Runtime()).LoadAndStart(&services.Machined{Controller: c})

	// Boot the machine.
	if err = c.Run(runtime.SequenceBoot, nil); err != nil {
		handle(err)
	}

	// Watch and handle runtime events.
	_ = c.Runtime().Events().Watch(func(events <-chan runtime.Event) { //nolint: errcheck
		for {
			for event := range events {
				if msg, ok := event.Payload.(*machine.SequenceEvent); ok {
					if msg.Error != nil {
						if msg.Error.GetCode() == common.Code_LOCKED {
							// ignore sequence lock errors, they're not fatal
							continue
						}

						handle(fmt.Errorf("fatal sequencer error in %q sequence: %v", msg.GetSequence(), msg.GetError().String()))
					}
				}
			}
		}
	})

	select {}
}
