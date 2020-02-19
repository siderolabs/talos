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
	"time"

	"golang.org/x/net/http/httpproxy"
	"golang.org/x/sys/unix"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/app/machined/internal/sequencer"
	"github.com/talos-systems/talos/internal/pkg/event"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/proc/reaper"
	"github.com/talos-systems/talos/pkg/startup"
)

// EventBusObserver is used to subscribe to the event bus.
type EventBusObserver struct {
	*event.Embeddable
}

func recovery() {
	if r := recover(); r != nil {
		log.Printf("recovered from: %+v\n", r)

		for i := 10; i >= 0; i-- {
			log.Printf("rebooting in %d seconds\n", i)
			time.Sleep(1 * time.Second)
		}

		if unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART) == nil {
			select {}
		}
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

func init() {
	// Explicitly set the default http client transport
	// to work around our fun proxy.Do once bug.
	// This is the http.DefaultTransport with the Proxy
	// func overridden so that the environment variables
	// with be reread/initialized each time the http call
	// is made.
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

// nolint: gocyclo
func main() {
	var err error

	// This is main entrypoint into machined execution, control is passed here
	// from init after switch root.
	//
	// When machined terminates either on normal shutdown (reboot, poweroff), or
	// due to panic, control goes through recovery() and reboot() functions
	// below, which finalize node state - sync buffers, initiate poweroff or
	// reboot. Also on shutdown, other deferred function are called, for example
	// services are gracefully shutdown.

	// On any return from init.main(), initiate host reboot or shutdown handle
	// any panics in the main goroutine, and proceed to reboot() above
	defer recovery()

	// Subscribe to events.
	init := EventBusObserver{&event.Embeddable{}}
	defer close(init.Channel())

	event.Bus().Register(init)

	defer event.Bus().Unregister(init)

	// Initialize process reaper.
	reaper.Run()
	defer reaper.Shutdown()

	// Ensure rng is seeded.
	if err = startup.RandSeed(); err != nil {
		panic(err)
	}

	// Set the PATH env var.
	if err = os.Setenv("PATH", constants.PATH); err != nil {
		panic(errors.New("error setting PATH"))
	}

	immediateReboot := false

	// Boot the machine.
	seq := sequencer.New(sequencer.V1Alpha1)

	if err := seq.Boot(); err != nil {
		if errors.Is(err, runtime.ErrReboot) {
			immediateReboot = true
		} else {
			log.Println(err)
			panic(fmt.Errorf("boot failed: %w", err))
		}
	}

	// Wait for an event.

	flag := unix.LINUX_REBOOT_CMD_RESTART

	for !immediateReboot {
		switch e := <-init.Channel(); e.Type {
		case event.Shutdown:
			flag = unix.LINUX_REBOOT_CMD_POWER_OFF
			fallthrough
		case event.Reboot:
			immediateReboot = true
		case event.Upgrade:
			var (
				req *machineapi.UpgradeRequest
				ok  bool
			)

			if req, ok = e.Data.(*machineapi.UpgradeRequest); !ok {
				log.Println("cannot perform upgrade, unexpected data type")
				continue
			}

			if err := seq.Upgrade(req); err != nil {
				log.Println(err)
				panic(fmt.Errorf("upgrade failed: %w", err))
			}

			immediateReboot = true
		case event.Reset:
			var (
				req *machineapi.ResetRequest
				ok  bool
			)

			if req, ok = e.Data.(*machineapi.ResetRequest); !ok {
				log.Println("cannot perform reset, unexpected data type")
				continue
			}

			if err := seq.Reset(req); err != nil {
				log.Println(err)
				panic(fmt.Errorf("reset failed: %w", err))
			}

			if !req.GetReboot() {
				flag = unix.LINUX_REBOOT_CMD_POWER_OFF
			}

			immediateReboot = true
		}
	}

	if err := seq.Shutdown(); err != nil {
		log.Println(err)
		panic(fmt.Errorf("shutdown failed: %w", err))
	}

	sync()

	if err := unix.Reboot(flag); err != nil {
		log.Println(err)
		panic(err)
	}

	// Prevent a kernel panic.

	select {}
}
