// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/process"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var _ system.HealthcheckedService = (*WpaSupplicant)(nil)

// WpaSupplicant implements the Service interface, running a wpa_supplicant
// instance for a single wireless interface.
type WpaSupplicant struct {
	LinkName string
}

// WpaSupplicantServiceID returns the service ID for the given wireless link.
func WpaSupplicantServiceID(linkName string) string {
	return "wpa-supplicant-" + linkName
}

// WpaSupplicantConfigPath returns the wpa_supplicant configuration file path for the given wireless link.
func WpaSupplicantConfigPath(linkName string) string {
	return filepath.Join(constants.WifiSupplicantRunDir, linkName+".conf")
}

// ID implements the Service interface.
func (svc *WpaSupplicant) ID(runtime.Runtime) string {
	return WpaSupplicantServiceID(svc.LinkName)
}

// PreFunc implements the Service interface.
func (svc *WpaSupplicant) PreFunc(context.Context, runtime.Runtime) error {
	return nil
}

// PostFunc implements the Service interface.
func (svc *WpaSupplicant) PostFunc(runtime.Runtime, events.ServiceState) error {
	return nil
}

// Condition implements the Service interface.
func (svc *WpaSupplicant) Condition(runtime.Runtime) conditions.Condition {
	return conditions.WaitForFileToExist(WpaSupplicantConfigPath(svc.LinkName))
}

// DependsOn implements the Service interface.
func (svc *WpaSupplicant) DependsOn(runtime.Runtime) []string {
	return nil
}

// Volumes implements the Service interface.
func (svc *WpaSupplicant) Volumes(runtime.Runtime) []string {
	return nil
}

// Runner implements the Service interface.
func (svc *WpaSupplicant) Runner(r runtime.Runtime) (runner.Runner, error) {
	args := &runner.Args{
		ID: svc.ID(r),
		ProcessArgs: []string{
			"/usr/bin/wpa_supplicant",
			"-i", svc.LinkName,
			"-D", "nl80211",
			"-c", WpaSupplicantConfigPath(svc.LinkName),
		},
	}

	debug := false

	if r.Config() != nil {
		debug = r.Config().Debug()
	}

	return restart.New(
		process.NewRunner(
			debug,
			args,
			runner.WithLoggingManager(r.Logging()),
			runner.WithCgroupPath(constants.CgroupWpaSupplicant),
		),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (svc *WpaSupplicant) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		// wpa_supplicant creates a control socket per interface once it has
		// successfully initialized the driver.
		if err := conditions.WaitForFileToExist(filepath.Join(constants.WifiSupplicantRunDir, svc.LinkName)).Wait(ctx); err != nil {
			return fmt.Errorf("wpa_supplicant control socket is not available: %w", err)
		}

		return nil
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (svc *WpaSupplicant) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
