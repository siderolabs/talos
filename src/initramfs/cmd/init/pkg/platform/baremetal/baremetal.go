// +build linux

package baremetal

import (
	"fmt"

	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/kernel"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
)

// BareMetal is a discoverer for non-cloud environments.
type BareMetal struct{}

// Name implements the platform.Platform interface.
func (b *BareMetal) Name() string {
	return "Bare Metal"
}

// UserData implements the platform.Platform interface.
func (b *BareMetal) UserData() (data userdata.UserData, err error) {
	arguments, err := kernel.ParseProcCmdline()
	if err != nil {
		return
	}

	endpoint, ok := arguments[constants.KernelParamUserData]
	if !ok {
		return data, fmt.Errorf("no user data endpoint was found")
	}

	return userdata.Download(endpoint)
}

// Prepare implements the platform.Platform interface.
func (b *BareMetal) Prepare(data userdata.UserData) (err error) {
	return nil
}
