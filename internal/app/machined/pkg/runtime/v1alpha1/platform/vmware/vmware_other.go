// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !(amd64 || arm64)

package vmware

import (
	"context"
	"errors"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// Configuration implements the platform.Platform interface.
func (v *VMware) Configuration(context.Context, state.State) ([]byte, error) {
	return nil, errors.New("arch not supported")
}

// NetworkConfiguration implements the runtime.Platform interface.
func (v *VMware) NetworkConfiguration(ctx context.Context, _ state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	return nil
}
