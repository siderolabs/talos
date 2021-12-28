// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !amd64
// +build !amd64

package vmware

import (
	"context"
	"fmt"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

// VMware is the concrete type that implements the platform.Platform interface.
type VMware struct{}

// Name implements the platform.Platform interface.
func (v *VMware) Name() string {
	return "vmware"
}

// Configuration implements the platform.Platform interface.
func (v *VMware) Configuration(context.Context) ([]byte, error) {
	return nil, fmt.Errorf("arch not supported")
}

// Mode implements the platform.Platform interface.
func (v *VMware) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (v *VMware) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (v *VMware) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	return nil
}
