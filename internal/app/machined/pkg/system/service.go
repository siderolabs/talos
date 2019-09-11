/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package system

import (
	"context"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Service is an interface describing a system service.
type Service interface {
	// ID is the service id.
	ID(*userdata.UserData) string
	// PreFunc is invoked before a runner is created
	PreFunc(context.Context, *userdata.UserData) error
	// Runner creates runner for the service
	Runner(*userdata.UserData) (runner.Runner, error)
	// PostFunc is invoked after a runner is closed.
	PostFunc(*userdata.UserData) error
	// Condition describes the conditions under which a service should
	// start.
	Condition(*userdata.UserData) conditions.Condition
	// DependsOn returns list of service IDs this service depends on.
	DependsOn(*userdata.UserData) []string
}

// HealthcheckedService is a service which provides health check
type HealthcheckedService interface {
	// HealtFunc provides function that checks health of the service
	HealthFunc(*userdata.UserData) health.Check
	// HealthSettings returns settings for the health check
	HealthSettings(*userdata.UserData) *health.Settings
}

// APIStartableService is a service which allows to be started via API
type APIStartableService interface {
	APIStartAllowed(*userdata.UserData) bool
}

// APIStoppableService is a service which allows to be stopped via API
type APIStoppableService interface {
	APIStopAllowed(*userdata.UserData) bool
}

// APIRestartableService is a service which allows to be restarted via API
type APIRestartableService interface {
	APIRestartAllowed(*userdata.UserData) bool
}
