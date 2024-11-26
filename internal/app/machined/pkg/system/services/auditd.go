// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"

	"github.com/siderolabs/talos/internal/app/auditd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/siderolabs/talos/pkg/conditions"
)

const auditdServiceID = "auditd"

var _ system.HealthcheckedService = (*Auditd)(nil)

// Auditd implements the Service interface. It serves as the concrete type with
// the required methods.
type Auditd struct{}

// ID implements the Service interface.
func (s *Auditd) ID(runtime.Runtime) string {
	return auditdServiceID
}

// PreFunc implements the Service interface.
func (s *Auditd) PreFunc(context.Context, runtime.Runtime) error {
	return nil
}

// PostFunc implements the Service interface.
func (s *Auditd) PostFunc(runtime.Runtime, events.ServiceState) error {
	return nil
}

// Condition implements the Service interface.
func (s *Auditd) Condition(runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (s *Auditd) DependsOn(runtime.Runtime) []string {
	return nil
}

// Runner implements the Service interface.
func (s *Auditd) Runner(r runtime.Runtime) (runner.Runner, error) {
	return goroutine.NewRunner(r, auditdServiceID, auditd.Main, runner.WithLoggingManager(r.Logging())), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (s *Auditd) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		return nil
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (s *Auditd) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
