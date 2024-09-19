// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:golint
package services

import (
	"context"
	"fmt"
	"strconv"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/internal/pkg/capability"
	"github.com/siderolabs/talos/internal/pkg/console"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Dashboard implements the Service interface. It serves as the concrete type with
// the required methods.
type Dashboard struct{}

// ID implements the Service interface.
func (d *Dashboard) ID(_ runtime.Runtime) string {
	return "dashboard"
}

// PreFunc implements the Service interface.
func (d *Dashboard) PreFunc(_ context.Context, _ runtime.Runtime) error {
	return console.Switch(constants.DashboardTTY)
}

// PostFunc implements the Service interface.
func (d *Dashboard) PostFunc(_ runtime.Runtime, _ events.ServiceState) error {
	return console.Switch(constants.KernelLogsTTY)
}

// Condition implements the Service interface.
func (d *Dashboard) Condition(_ runtime.Runtime) conditions.Condition {
	return conditions.WaitForFileToExist(constants.MachineSocketPath)
}

// DependsOn implements the Service interface.
func (d *Dashboard) DependsOn(_ runtime.Runtime) []string {
	return []string{machinedServiceID}
}

// Runner implements the Service interface.
func (d *Dashboard) Runner(r runtime.Runtime) (runner.Runner, error) {
	tty := fmt.Sprintf("/dev/tty%d", constants.DashboardTTY)

	return restart.New(process.NewRunner(false, &runner.Args{
		ID:          d.ID(r),
		ProcessArgs: []string{"/sbin/dashboard"},
	},
		runner.WithLoggingManager(r.Logging()),
		runner.WithEnv([]string{
			"TERM=linux",
			constants.TcellMinimizeEnvironment,
			"GOMEMLIMIT=" + strconv.Itoa(constants.CgroupDashboardMaxMemory/5*4),
		}),
		runner.WithStdinFile(tty),
		runner.WithStdoutFile(tty),
		runner.WithCtty(1),
		runner.WithOOMScoreAdj(-400),
		runner.WithDroppedCapabilities(capability.AllCapabilitiesSetLowercase()),
		runner.WithCgroupPath(constants.CgroupDashboard),
		runner.WithUID(constants.DashboardUserID),
	),
		restart.WithType(restart.Forever),
	), nil
}
