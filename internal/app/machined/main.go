// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
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

	"github.com/talos-systems/go-cmd/pkg/cmd/proc"
	"github.com/talos-systems/go-cmd/pkg/cmd/proc/reaper"
	debug "github.com/talos-systems/go-debug"
	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/apid"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
	"github.com/talos-systems/talos/internal/app/trustd"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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

func revertBootloader() {
	if meta, err := bootloader.NewMeta(); err == nil {
		if err = meta.Revert(); err != nil {
			log.Printf("failed to revert upgrade: %v", err)
		}

		//nolint:errcheck
		meta.Close()
	} else {
		log.Printf("failed to open meta: %v", err)
	}
}

// syncNonVolatileStorageBuffers invokes unix.Sync and waits up to 30 seconds
// for it to finish.
//
// See http://man7.org/linux/man-pages/man2/reboot.2.html.
func syncNonVolatileStorageBuffers() {
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

//nolint:gocyclo
func handle(err error) {
	rebootCmd := unix.LINUX_REBOOT_CMD_RESTART

	var rebootErr runtime.RebootError

	if errors.As(err, &rebootErr) {
		// not a failure, but wrapped reboot command
		rebootCmd = rebootErr.Cmd

		err = nil
	}

	if err != nil {
		log.Print(err)
		revertBootloader()

		if p := procfs.ProcCmdline().Get(constants.KernelParamPanic).First(); p != nil {
			if *p == "0" {
				log.Printf("panic=0 kernel flag found, sleeping forever")

				rebootCmd = 0
			}
		}
	}

	if rebootCmd == unix.LINUX_REBOOT_CMD_RESTART {
		for i := 10; i >= 0; i-- {
			log.Printf("rebooting in %d seconds\n", i)
			time.Sleep(1 * time.Second)
		}
	}

	if err = proc.KillAll(); err != nil {
		log.Printf("error killing all procs: %s", err)
	}

	if err = mount.UnmountAll(); err != nil {
		log.Printf("error unmounting: %s", err)
	}

	syncNonVolatileStorageBuffers()

	if rebootCmd == 0 {
		exitSignal := make(chan os.Signal, 1)

		signal.Notify(exitSignal, syscall.SIGINT, syscall.SIGTERM)

		<-exitSignal
	} else if unix.Reboot(rebootCmd) == nil {
		// Wait forever.
		select {}
	}
}

func runDebugServer(ctx context.Context) {
	const debugAddr = ":9982"

	debugLogFunc := func(msg string) {
		log.Print(msg)
	}

	if err := debug.ListenAndServe(ctx, debugAddr, debugLogFunc); err != nil {
		log.Fatalf("failed to start debug server: %s", err)
	}
}

//nolint:gocyclo
func run() error {
	errCh := make(chan error)

	// Ensure RNG is seeded.
	if err := startup.RandSeed(); err != nil {
		return err
	}

	// Set the PATH env var.
	if err := os.Setenv("PATH", constants.PATH); err != nil {
		return errors.New("error setting PATH")
	}

	// Initialize the controller without a config.
	c, err := v1alpha1runtime.NewController()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	drainer := runtime.NewDrainer()
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), time.Second*10)

		defer cancel()

		if e := drainer.Drain(c); e != nil {
			log.Printf("WARNING: failed to drain controllers: %s", e)
		}
	}()

	go runDebugServer(ctx)

	// Schedule service shutdown on any return.
	defer system.Services(c.Runtime()).Shutdown(ctx)

	// Start signal and ACPI listeners.
	go func() {
		if e := c.ListenForEvents(ctx); e != nil {
			log.Printf("WARNING: signals and ACPI events will be ignored: %s", e)
		}
	}()

	// Start v2 controller runtime.
	go func() {
		if e := c.V1Alpha2().Run(ctx, drainer); e != nil {
			errCh <- fmt.Errorf("fatal controller runtime error: %s", e)
		}

		log.Printf("controller runtime finished")
	}()

	// Initialize the machine.
	if err = c.Run(ctx, runtime.SequenceInitialize, nil); err != nil {
		return err
	}

	// Perform an installation if required.
	if err = c.Run(ctx, runtime.SequenceInstall, nil); err != nil {
		return err
	}

	// Start the machine API.
	system.Services(c.Runtime()).LoadAndStart(&services.Machined{Controller: c})

	// Boot the machine.
	if err = c.Run(ctx, runtime.SequenceBoot, nil); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	// Watch and handle runtime events.
	_ = c.Runtime().Events().Watch(func(events <-chan runtime.EventInfo) { //nolint:errcheck
		for {
			for event := range events {
				switch msg := event.Payload.(type) {
				case *machine.SequenceEvent:
					if msg.Error != nil {
						if msg.Error.GetCode() == common.Code_LOCKED {
							// ignore sequence lock errors, they're not fatal
							continue
						}

						errCh <- fmt.Errorf("fatal sequencer error in %q sequence: %v", msg.GetSequence(), msg.GetError().String())
					}
				case *machine.RestartEvent:
					errCh <- runtime.RebootError{Cmd: int(msg.Cmd)}
				}
			}
		}
	})

	return <-errCh
}

func main() {
	switch os.Args[0] {
	case "/apid":
		apid.Main()

		return
	case "/trustd":
		trustd.Main()

		return
	default:
	}

	// Setup panic handler.
	defer recovery()

	// Initialize the process reaper.
	reaper.Run()
	defer reaper.Shutdown()

	handle(run())
}
