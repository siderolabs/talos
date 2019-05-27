/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"

	"github.com/talos-systems/talos/internal/app/init/pkg/network"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/goroutine"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Networkd implements the Service interface. It serves as the concrete type with
// the required methods.
type Networkd struct{}

// ID implements the Service interface.
func (c *Networkd) ID(data *userdata.UserData) string {
	return "networkd"
}

// PreFunc implements the Service interface.
func (c *Networkd) PreFunc(ctx context.Context, data *userdata.UserData) error {
	return nil
}

// PostFunc implements the Service interface.
func (c *Networkd) PostFunc(data *userdata.UserData) (err error) {
	return nil
}

// Condition implements the Service interface.
func (c *Networkd) Condition(data *userdata.UserData) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (c *Networkd) DependsOn(data *userdata.UserData) []string {
	return nil
}

// Runner implements the Service interface.
func (c *Networkd) Runner(data *userdata.UserData) (runner.Runner, error) {
	return goroutine.NewRunner(data, "networkd", network.NewService().Main), nil
}
