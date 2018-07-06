// +build linux

package platform

import (
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/platform/baremetal"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/platform/cloud/aws"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
)

// Platform is an interface describing a platform.
type Platform interface {
	Name() string
	UserData() (userdata.UserData, error)
	Prepare(userdata.UserData) error
}

// NewPlatform is a helper func for discovering the current platform.
func NewPlatform() (p Platform, err error) {
	if aws.IsEC2() {
		p = &aws.AWS{}
	} else {
		p = &baremetal.BareMetal{}
	}

	return p, nil
}
