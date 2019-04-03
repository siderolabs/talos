/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package vmware

import (
	"encoding/base64"
	"fmt"

	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/vmware/vmw-guestinfo/rpcvmx"
	"github.com/vmware/vmw-guestinfo/vmcheck"

	yaml "gopkg.in/yaml.v2"
)

// VMware is the concrete type that implements the platform.Platform interface.
type VMware struct{}

// Name implements the platform.Platform interface.
func (vmw *VMware) Name() string {
	return "VMware"
}

// UserData implements the platform.Platform interface.
func (vmw *VMware) UserData() (data *userdata.UserData, err error) {
	arguments, err := kernel.ParseProcCmdline()
	if err != nil {
		return
	}

	option, ok := arguments[constants.KernelParamUserData]
	if !ok {
		return data, fmt.Errorf("no user data option was found")
	}

	if option == constants.UserDataGuestInfo {
		ok, err := vmcheck.IsVirtualWorld()
		if err != nil {
			return data, err
		}

		if !ok {
			return data, fmt.Errorf("not a virtual world")
		}

		config := rpcvmx.NewConfig()
		val, err := config.String(constants.VMwareGuestInfoUserDataKey, "")
		if err != nil {
			return data, fmt.Errorf("failed to get guestinfo.%s: %v", constants.VMwareGuestInfoUserDataKey, err)
		}

		if val == "" {
			return data, fmt.Errorf("userdata is required, no value found for guestinfo.%s: %v", constants.VMwareGuestInfoUserDataKey, err)
		}

		b, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			return data, fmt.Errorf("failed to decode guestinfo.%s: %v", constants.VMwareGuestInfoUserDataKey, err)
		}

		if err = yaml.Unmarshal(b, &data); err != nil {
			return data, fmt.Errorf("unmarshal user data: %s", err.Error())
		}
	}

	return data, nil
}

// Prepare implements the platform.Platform interface and handles initial host preparation.
func (vmw *VMware) Prepare(data *userdata.UserData) (err error) {
	return nil
}

// Install implements the platform.Platform interface and handles additional system setup.
func (vmw *VMware) Install(data *userdata.UserData) (err error) {
	return nil
}
