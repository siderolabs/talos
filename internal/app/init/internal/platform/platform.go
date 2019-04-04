/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package platform

import (
	"fmt"

	"github.com/talos-systems/talos/internal/app/init/internal/platform/baremetal"
	"github.com/talos-systems/talos/internal/app/init/internal/platform/cloud/aws"
	"github.com/talos-systems/talos/internal/app/init/internal/platform/cloud/googlecloud"
	"github.com/talos-systems/talos/internal/app/init/internal/platform/cloud/packet"
	"github.com/talos-systems/talos/internal/app/init/internal/platform/cloud/vmware"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Platform is an interface describing a platform.
type Platform interface {
	Name() string
	UserData() (*userdata.UserData, error)
	Prepare(*userdata.UserData) error
	Install(*userdata.UserData) error
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
		case "googlecloud":
			p = &googlecloud.GoogleCloud{}
		case "vmware":
			p = &vmware.VMware{}
		case "bare-metal":
			p = &baremetal.BareMetal{}
		case "packet":
			p = &packet.Packet{}
		default:
			return nil, fmt.Errorf("platform not supported: %s", platform)
		}
	}
	return p, nil
}
