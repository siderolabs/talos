// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package iso

import (
	"net"

	"gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
)

// ISO is a platform for installing Talos via an ISO image.
type ISO struct{}

// Name implements the platform.Platform interface.
func (i *ISO) Name() string {
	return "iso"
}

// Configuration implements the platform.Platform interface.
func (i *ISO) Configuration() ([]byte, error) {
	config := v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineInstall: &v1alpha1.InstallConfig{
				InstallForce:      true,
				InstallDisk:       "/dev/sda",
				InstallBootloader: true,
			},
		},
	}

	b, err := yaml.Marshal(&config)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// Hostname implements the platform.Platform interface.
func (i *ISO) Hostname() (hostname []byte, err error) {
	return nil, nil
}

// Mode implements the platform.Platform interface.
func (i *ISO) Mode() runtime.Mode {
	return runtime.Interactive
}

// ExternalIPs implements the runtime.Platform interface.
func (i *ISO) ExternalIPs() (addrs []net.IP, err error) {
	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (i *ISO) KernelArgs() kernel.Parameters {
	return nil
}
