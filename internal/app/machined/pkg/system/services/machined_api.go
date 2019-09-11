/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"

	"github.com/talos-systems/talos/internal/app/machined/internal/api"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/talos-systems/talos/pkg/userdata"
)

// MachinedAPI implements the Service interface. It serves as the concrete type with
// the required methods.
type MachinedAPI struct{}

// ID implements the Service interface.
func (c *MachinedAPI) ID(data *userdata.UserData) string {
	return "machined-api"
}

// PreFunc implements the Service interface.
func (c *MachinedAPI) PreFunc(ctx context.Context, data *userdata.UserData) error {
	return nil
}

// PostFunc implements the Service interface.
func (c *MachinedAPI) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// Condition implements the Service interface.
func (c *MachinedAPI) Condition(data *userdata.UserData) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (c *MachinedAPI) DependsOn(data *userdata.UserData) []string {
	return nil
}

// Runner implements the Service interface.
func (c *MachinedAPI) Runner(data *userdata.UserData) (runner.Runner, error) {
	return goroutine.NewRunner(data, "machined-api", api.NewService().Main, runner.WithLogPath("/run")), nil
}
