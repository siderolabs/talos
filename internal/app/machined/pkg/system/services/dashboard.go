// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:golint
package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-procfs/procfs"

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

// getCustomConsole returns the custom console parameter value if specified, empty string otherwise.
func (d *Dashboard) getCustomConsole() string {
	consoleParam := procfs.ProcCmdline().Get(constants.KernelParamDashboardConsole).First()

	return pointer.SafeDeref(consoleParam)
}

// hasCustomConsole checks if a custom console is specified via kernel parameter.
func (d *Dashboard) hasCustomConsole() bool {
	return d.getCustomConsole() != ""
}

// getConsoleDevice returns the console device path to use for the dashboard.
func (d *Dashboard) getConsoleDevice() (string, error) {
	consoleName := d.getCustomConsole()

	if consoleName != "" {
		// Validate that the console name starts with "tty"
		if !strings.HasPrefix(consoleName, "tty") || strings.Contains(consoleName, "/") {
			return "", fmt.Errorf("invalid console name %q: must start with 'tty'", consoleName)
		}

		return fmt.Sprintf("/dev/%s", consoleName), nil
	}

	// Default to the standard dashboard TTY
	return fmt.Sprintf("/dev/tty%d", constants.DashboardTTY), nil
}

// ID implements the Service interface.
func (d *Dashboard) ID(_ runtime.Runtime) string {
	return "dashboard"
}

// PreFunc implements the Service interface.
func (d *Dashboard) PreFunc(_ context.Context, _ runtime.Runtime) error {
	// Skip TTY switching if a custom console is specified
	if d.hasCustomConsole() {
		return nil
	}

	return console.Switch(constants.DashboardTTY)
}

// PostFunc implements the Service interface.
func (d *Dashboard) PostFunc(_ runtime.Runtime, _ events.ServiceState) error {
	// Skip TTY switching if a custom console is specified
	if d.hasCustomConsole() {
		return nil
	}

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

// Volumes implements the Service interface.
func (d *Dashboard) Volumes(runtime.Runtime) []string {
	return nil
}

// Runner implements the Service interface.
func (d *Dashboard) Runner(r runtime.Runtime) (runner.Runner, error) {
	tty, err := d.getConsoleDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to determine console device: %w", err)
	}

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
		runner.WithCtty(0),
		runner.WithOOMScoreAdj(-400),
		runner.WithDroppedCapabilities(capability.AllCapabilitiesSetLowercase()),
		runner.WithSelinuxLabel(constants.SelinuxLabelDashboard),
		runner.WithCgroupPath(constants.CgroupDashboard),
		runner.WithUID(constants.DashboardUserID),
		runner.WithPriority(constants.DashboardPriority),
	),
		restart.WithType(restart.Forever),
	), nil
}
