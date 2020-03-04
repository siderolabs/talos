// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"context"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// Service is an interface describing a system service.
type Service interface {
	// ID is the service id.
	ID(runtime.Configurator) string
	// PreFunc is invoked before a runner is created
	PreFunc(context.Context, runtime.Configurator) error
	// Runner creates runner for the service
	Runner(runtime.Configurator) (runner.Runner, error)
	// PostFunc is invoked after a runner is closed.
	PostFunc(runtime.Configurator, events.ServiceState) error
	// Condition describes the conditions under which a service should
	// start.
	Condition(runtime.Configurator) conditions.Condition
	// DependsOn returns list of service IDs this service depends on.
	DependsOn(runtime.Configurator) []string
}

// HealthcheckedService is a service which provides health check
type HealthcheckedService interface {
	// HealtFunc provides function that checks health of the service
	HealthFunc(runtime.Configurator) health.Check
	// HealthSettings returns settings for the health check
	HealthSettings(runtime.Configurator) *health.Settings
}

// APIStartableService is a service which allows to be started via API
type APIStartableService interface {
	APIStartAllowed(runtime.Configurator) bool
}

// APIStoppableService is a service which allows to be stopped via API
type APIStoppableService interface {
	APIStopAllowed(runtime.Configurator) bool
}

// APIRestartableService is a service which allows to be restarted via API
type APIRestartableService interface {
	APIRestartAllowed(runtime.Configurator) bool
}
