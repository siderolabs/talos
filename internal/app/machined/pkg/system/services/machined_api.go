// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"

	"github.com/talos-systems/talos/internal/app/machined/internal/api"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/talos-systems/talos/internal/pkg/conditions"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// MachinedAPI implements the Service interface. It serves as the concrete type with
// the required methods.
type MachinedAPI struct{}

// ID implements the Service interface.
func (c *MachinedAPI) ID(config runtime.Configurator) string {
	return "machined-api"
}

// PreFunc implements the Service interface.
func (c *MachinedAPI) PreFunc(ctx context.Context, config runtime.Configurator) error {
	return nil
}

// PostFunc implements the Service interface.
func (c *MachinedAPI) PostFunc(config runtime.Configurator, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (c *MachinedAPI) Condition(config runtime.Configurator) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (c *MachinedAPI) DependsOn(config runtime.Configurator) []string {
	return nil
}

// Runner implements the Service interface.
func (c *MachinedAPI) Runner(config runtime.Configurator) (runner.Runner, error) {
	return goroutine.NewRunner(config, "machined-api", api.NewService().Main), nil
}
