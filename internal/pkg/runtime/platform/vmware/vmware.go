// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vmware

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"

	"github.com/vmware/vmw-guestinfo/rpcvmx"
	"github.com/vmware/vmw-guestinfo/vmcheck"

	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// VMware is the concrete type that implements the platform.Platform interface.
type VMware struct{}

// Name implements the platform.Platform interface.
func (v *VMware) Name() string {
	return "VMware"
}

// Configuration implements the platform.Platform interface.
func (v *VMware) Configuration() ([]byte, error) {
	var option *string
	if option = kernel.ProcCmdline().Get(constants.KernelParamConfig).First(); option == nil {
		return nil, fmt.Errorf("no config option was found")
	}

	if *option == constants.ConfigGuestInfo {
		ok, err := vmcheck.IsVirtualWorld()
		if err != nil {
			return nil, err
		}

		if !ok {
			return nil, errors.New("not a virtual world")
		}

		config := rpcvmx.NewConfig()

		val, err := config.String(constants.VMwareGuestInfoConfigKey, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get guestinfo.%s: %w", constants.VMwareGuestInfoConfigKey, err)
		}

		if val == "" {
			return nil, fmt.Errorf("config is required, no value found for guestinfo.%s: %w", constants.VMwareGuestInfoConfigKey, err)
		}

		b, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			return nil, fmt.Errorf("failed to decode guestinfo.%s: %w", constants.VMwareGuestInfoConfigKey, err)
		}

		return b, nil
	}

	return nil, nil
}

// Hostname implements the platform.Platform interface.
func (v *VMware) Hostname() (hostname []byte, err error) {
	return nil, nil
}

// Mode implements the platform.Platform interface.
func (v *VMware) Mode() runtime.Mode {
	return runtime.Cloud
}

// ExternalIPs provides any external addresses assigned to the instance
func (v *VMware) ExternalIPs() (addrs []net.IP, err error) {
	return addrs, err
}
