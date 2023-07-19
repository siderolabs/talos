// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2

import (
	"context"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/config"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
)

// platformConfigurator adapts a runtime.Platform to the config.PlatformConfigurator interface.
type platformConfigurator struct {
	platform runtime.Platform
	state    state.State
}

// Check interfaces.
var (
	_ config.PlatformConfigurator = &platformConfigurator{}
)

func (p *platformConfigurator) Name() string {
	return p.platform.Name()
}

func (p *platformConfigurator) Configuration(ctx context.Context) ([]byte, error) {
	return p.platform.Configuration(ctx, p.state)
}

// platformEventer adapts a runtime.Platform to the config.PlatformEventer interface.
type platformEventer struct {
	platform runtime.Platform
}

// Check interfaces.
var (
	_ config.PlatformEventer = &platformEventer{}
)

func (p *platformEventer) FireEvent(ctx context.Context, event platform.Event) {
	platform.FireEvent(ctx, p.platform, event)
}
