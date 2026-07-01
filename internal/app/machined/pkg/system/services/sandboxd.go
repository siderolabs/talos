// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"errors"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/sandboxd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/pkg/conditions"
)

var _ system.HealthcheckedService = (*Sandboxd)(nil)

// Sandboxd implements the Service interface. It owns the sandbox PID+mount
// namespace that CRI (and the pods it runs) execute inside, isolating them from
// machined (PID 1) and its file descriptors.
//
// It is a restart-forever service: if sandboxd dies the kernel tears down the
// namespace, and this service recreates it instead of rebooting the node;
// dependent services (e.g. cri) re-launch into the new namespace.
type Sandboxd struct{}

// ID implements the Service interface.
func (s *Sandboxd) ID(runtime.Runtime) string {
	return sandboxd.ServiceID
}

// PreFunc implements the Service interface.
func (s *Sandboxd) PreFunc(context.Context, runtime.Runtime) error {
	return nil
}

// PostFunc implements the Service interface.
func (s *Sandboxd) PostFunc(runtime.Runtime, events.ServiceState) error {
	return nil
}

// Condition implements the Service interface.
func (s *Sandboxd) Condition(runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (s *Sandboxd) DependsOn(runtime.Runtime) []string {
	return nil
}

// Volumes implements the Service interface.
func (s *Sandboxd) Volumes(runtime.Runtime) []string {
	return nil
}

// HealthFunc implements the HealthcheckedService interface. The namespace is
// healthy while the launcher is published (sandboxd running); it reports
// unhealthy during the window after an unexpected exit until the namespace is
// recreated.
func (s *Sandboxd) HealthFunc(r runtime.Runtime) health.Check {
	return func(context.Context) error {
		if r.Sandbox() == nil {
			return errors.New("sandbox namespace not available")
		}

		return nil
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (s *Sandboxd) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}

// Runner implements the Service interface.
func (s *Sandboxd) Runner(r runtime.Runtime) (runner.Runner, error) {
	return restart.New(
		sandboxd.NewRunner(r),
		restart.WithType(restart.Forever),
	), nil
}
