// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package initializer

import (
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/initializer/cloud"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/initializer/container"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/initializer/interactive"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/initializer/metal"
)

// Initializer defines a process for initializing the system based on the
// environment it is in.
type Initializer interface {
	Initialize(runtime.Runtime) error
}

// New initializes and returns and Initializer based on the runtime mode.
func New(mode runtime.Mode) (Initializer, error) {
	switch mode {
	case runtime.Interactive:
		return &interactive.Interactive{}, nil
	case runtime.Metal:
		return &metal.Metal{}, nil
	case runtime.Cloud:
		return &cloud.Cloud{}, nil
	case runtime.Container:
		return &container.Container{}, nil
	default:
	}

	return nil, nil
}
