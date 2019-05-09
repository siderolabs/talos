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
	"github.com/talos-systems/talos/internal/app/init/internal/platform/iso"
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
	if platform, ok := kernel.GetParameter(constants.KernelParamPlatform); ok {
		switch platform {
		case "aws":
			p = &aws.AWS{}
		case "googlecloud":
			p = &googlecloud.GoogleCloud{}
		case "vmware":
			p = &vmware.VMware{}
		case "bare-metal":
			p = &baremetal.BareMetal{}
		case "packet":
			p = &packet.Packet{}
		case "iso":
			p = &iso.ISO{}
		default:
			return nil, fmt.Errorf("platform not supported: %s", platform)
		}
	}
	return p, nil
}
