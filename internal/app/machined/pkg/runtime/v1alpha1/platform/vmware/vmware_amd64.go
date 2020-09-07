// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build amd64

package vmware

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/vmware/vmw-guestinfo/rpcvmx"
	"github.com/vmware/vmw-guestinfo/vmcheck"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// VMware is the concrete type that implements the platform.Platform interface.
type VMware struct{}

// Name implements the platform.Platform interface.
func (v *VMware) Name() string {
	return "vmware"
}

// Configuration implements the platform.Platform interface.
func (v *VMware) Configuration() ([]byte, error) {
	var option *string
	if option = procfs.ProcCmdline().Get(constants.KernelParamConfig).First(); option == nil {
		return nil, fmt.Errorf("no config option was found")
	}

	if *option == constants.ConfigGuestInfo {
		log.Printf("fetching machine config from: guestinfo key %q", constants.VMwareGuestInfoConfigKey)

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
			return nil, fmt.Errorf("config is required, no value found for guestinfo: %q", constants.VMwareGuestInfoConfigKey)
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
	return runtime.ModeCloud
}

// ExternalIPs implements the runtime.Platform interface.
func (v *VMware) ExternalIPs() (addrs []net.IP, err error) {
	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (v *VMware) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty0").Append("ttyS0"),
		procfs.NewParameter("earlyprintk").Append("ttyS0,115200"),
	}
}
