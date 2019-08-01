/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package api

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/api/reg"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/userdata"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

var (
	rebootCmd int
)

// API represents the API task.
type API struct{}

// NewAPITask initializes and returns an API task.
func NewAPITask() phase.Task {
	return &API{}
}

// RuntimeFunc returns the runtime function.
func (task *API) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Standard:
		return task.standard
	case runtime.Container:
		return task.container
	default:
		return nil
	}
}

func (task *API) standard(platform platform.Platform, data *userdata.UserData) (err error) {
	var (
		api    *reg.Registrator
		server *grpc.Server
	)
	if api, server, err = start(data); err != nil {
		return err
	}

	go func() {
		poweroffCh, err := listenForPowerButton()
		if err != nil {
			log.Printf("WARNING: power off events will be ignored: %+v", err)
		}

		termCh := make(chan os.Signal, 1)
		signal.Notify(termCh, syscall.SIGTERM)

		// NB: Defer is FILO.
		defer reboot()
		defer server.Stop()
		defer system.Services(data).Shutdown()

		select {
		case <-api.ShutdownCh:
			log.Printf("poweroff via API received")
			// poweroff, proceed to shutdown but mark as poweroff
			rebootCmd = unix.LINUX_REBOOT_CMD_POWER_OFF
		case <-poweroffCh:
			log.Printf("poweroff via ACPI received")
			// poweroff, proceed to shutdown but mark as poweroff
			rebootCmd = unix.LINUX_REBOOT_CMD_POWER_OFF
		case <-termCh:
			log.Printf("SIGTERM received, rebooting...")
		case <-api.RebootCh:
			log.Printf("reboot via API received, rebooting...")
			rebootCmd = unix.LINUX_REBOOT_CMD_RESTART
		}
	}()

	return nil
}

func (task *API) container(platform platform.Platform, data *userdata.UserData) (err error) {
	if _, _, err = start(data); err != nil {
		return err
	}

	return nil
}

func start(data *userdata.UserData) (api *reg.Registrator, server *grpc.Server, err error) {
	api = reg.NewRegistrator(data)
	server = factory.NewServer(api)
	listener, err := factory.NewListener(factory.Network("unix"), factory.SocketPath(constants.InitSocketPath))
	if err != nil {
		return nil, nil, err
	}

	go func() {
		// nolint: errcheck
		server.Serve(listener)
	}()

	return api, server, nil
}
