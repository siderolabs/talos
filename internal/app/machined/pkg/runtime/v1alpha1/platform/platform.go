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
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/hcloud"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/nocloud"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/openstack"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/packet"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/scaleway"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/upcloud"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/vmware"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/vultr"
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

//nolint:gocyclo
func newPlatform(platform string) (p runtime.Platform, err error) {
	switch platform {
	case "aws":
		return aws.NewAWS()
	case "azure":
		p = &azure.Azure{}
	case "container":
		p = &container.Container{}
	case "digital-ocean":
		p = &digitalocean.DigitalOcean{}
	case "gcp":
		p = &gcp.GCP{}
	case "hcloud":
		p = &hcloud.Hcloud{}
	case "metal":
		p = &metal.Metal{}
	case "openstack":
		p = &openstack.Openstack{}
	case "nocloud":
		p = &nocloud.Nocloud{}
	case "packet":
		p = &packet.Packet{}
	case "scaleway":
		p = &scaleway.Scaleway{}
	case "upcloud":
		p = &upcloud.UpCloud{}
	case "vmware":
		p = &vmware.VMware{}
	case "vultr":
		p = &vultr.Vultr{}
	default:
		return nil, fmt.Errorf("unknown platform: %q", platform)
	}

	return p, nil
}
