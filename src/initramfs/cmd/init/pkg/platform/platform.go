// +build linux

package platform

import (
	"fmt"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/kernel"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/platform/baremetal"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/platform/cloud/aws"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/platform/cloud/vmware"
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"
)

// Platform is an interface describing a platform.
type Platform interface {
	Name() string
	UserData() (userdata.UserData, error)
	Prepare(userdata.UserData) error
}

// NewPlatform is a helper func for discovering the current platform.
func NewPlatform() (p Platform, err error) {
	arguments, err := kernel.ParseProcCmdline()
	if err != nil {
		return
	}
	if platform, ok := arguments[constants.KernelParamPlatform]; ok {
		switch platform {
		case "aws":
			if aws.IsEC2() {
				p = &aws.AWS{}
			} else {
				return nil, fmt.Errorf("failed to verify EC2 PKCS7 signature")
			}
		case "vmware":
			p = &vmware.VMware{}
		case "bare-metal":
			p = &baremetal.BareMetal{}
		default:
			return nil, fmt.Errorf("platform not supported: %s", platform)
		}
	}
	return p, nil
}
