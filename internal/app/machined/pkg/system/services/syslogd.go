// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/siderolabs/talos/internal/app/syslogd"
	"github.com/siderolabs/talos/pkg/conditions"
)

const syslogServiceID = "syslogd"

var _ system.HealthcheckedService = (*Syslogd)(nil)

// Syslogd implements the Service interface. It serves as the concrete type with
// the required methods.
type Syslogd struct{}

// ID implements the Service interface.
func (s *Syslogd) ID(runtime.Runtime) string {
	return syslogServiceID
}

// PreFunc implements the Service interface.
func (s *Syslogd) PreFunc(context.Context, runtime.Runtime) error {
	return nil
}

// PostFunc implements the Service interface.
func (s *Syslogd) PostFunc(runtime.Runtime, events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (s *Syslogd) Condition(runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (s *Syslogd) DependsOn(runtime.Runtime) []string {
	return []string{machinedServiceID}
}

// Volumes implements the Service interface.
func (s *Syslogd) Volumes(runtime.Runtime) []string {
	return nil
}

// Runner implements the Service interface.
func (s *Syslogd) Runner(r runtime.Runtime) (runner.Runner, error) {
	return goroutine.NewRunner(r, syslogServiceID, syslogd.Main, runner.WithLoggingManager(r.Logging())), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (s *Syslogd) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		return nil
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (s *Syslogd) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
