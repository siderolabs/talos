/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package system

import (
	"context"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/pkg/config"
)

// Service is an interface describing a system service.
type Service interface {
	// ID is the service id.
	ID(config.Configurator) string
	// PreFunc is invoked before a runner is created
	PreFunc(context.Context, config.Configurator) error
	// Runner creates runner for the service
	Runner(config.Configurator) (runner.Runner, error)
	// PostFunc is invoked after a runner is closed.
	PostFunc(config.Configurator) error
	// Condition describes the conditions under which a service should
	// start.
	Condition(config.Configurator) conditions.Condition
	// DependsOn returns list of service IDs this service depends on.
	DependsOn(config.Configurator) []string
}

// HealthcheckedService is a service which provides health check
type HealthcheckedService interface {
	// HealtFunc provides function that checks health of the service
	HealthFunc(config.Configurator) health.Check
	// HealthSettings returns settings for the health check
	HealthSettings(config.Configurator) *health.Settings
}

// APIStartableService is a service which allows to be started via API
type APIStartableService interface {
	APIStartAllowed(config.Configurator) bool
}

// APIStoppableService is a service which allows to be stopped via API
type APIStoppableService interface {
	APIStopAllowed(config.Configurator) bool
}

// APIRestartableService is a service which allows to be restarted via API
type APIRestartableService interface {
	APIRestartAllowed(config.Configurator) bool
}
