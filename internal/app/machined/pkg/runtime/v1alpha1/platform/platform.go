// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package platform

import (
	"errors"
	"fmt"
	"os"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/aws"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/azure"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/container"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/digitalocean"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/gcp"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/openstack"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/packet"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/vmware"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// CurrentPlatform is a helper func for discovering the current platform.
func CurrentPlatform() (p runtime.Platform, err error) {
	var platform string

	if p := procfs.ProcCmdline().Get(constants.KernelParamPlatform).First(); p != nil {
		platform = *p
	}

	if p, ok := os.LookupEnv("PLATFORM"); ok {
		platform = p
	}

	if platform == "" {
		return nil, errors.New("failed to determine platform")
	}

	return newPlatform(platform)
}

// NewPlatform initializes and returns a runtime.Platform.
func NewPlatform(platform string) (p runtime.Platform, err error) {
	return newPlatform(platform)
}

func newPlatform(platform string) (p runtime.Platform, err error) {
	switch platform {
	case "aws":
		p = &aws.AWS{}
	case "azure":
		p = &azure.Azure{}
	case "container":
		p = &container.Container{}
	case "digital-ocean":
		p = &digitalocean.DigitalOcean{}
	case "gcp":
		p = &gcp.GCP{}
	case "metal":
		p = &metal.Metal{}
	case "openstack":
		p = &openstack.Openstack{}
	case "packet":
		p = &packet.Packet{}
	case "vmware":
		p = &vmware.VMware{}
	default:
		return nil, fmt.Errorf("unknown platform: %q", platform)
	}

	return p, nil
}
