/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package platform

import (
	"net"
	"os"

	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/platform/aws"

	"github.com/talos-systems/talos/internal/pkg/platform/azure"
	"github.com/talos-systems/talos/internal/pkg/platform/container"
	"github.com/talos-systems/talos/internal/pkg/platform/gcp"
	"github.com/talos-systems/talos/internal/pkg/platform/iso"
	"github.com/talos-systems/talos/internal/pkg/platform/metal"
	"github.com/talos-systems/talos/internal/pkg/platform/packet"
	"github.com/talos-systems/talos/internal/pkg/platform/vmware"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// Platform is an interface describing a platform.
type Platform interface {
	Name() string
	Configuration() ([]byte, error)
	Mode() runtime.Mode
	Hostname() ([]byte, error)
	ExternalIPs() ([]net.IP, error)
}

// NewPlatform is a helper func for discovering the current platform.
//
// nolint: gocyclo
func NewPlatform() (p Platform, err error) {
	var platform string
	if p := kernel.ProcCmdline().Get(constants.KernelParamPlatform).First(); p != nil {
		platform = *p
	}

	if p, ok := os.LookupEnv("PLATFORM"); ok {
		platform = p
	}

	if platform == "" {
		return nil, errors.New("failed to determine platform")
	}

	switch platform {
	case "aws":
		p = &aws.AWS{}
	case "azure":
		p = &azure.Azure{}
	case "metal":
		p = &metal.Metal{}
	case "container":
		p = &container.Container{}
	case "gcp":
		p = &gcp.GCP{}
	case "iso":
		p = &iso.ISO{}
	case "packet":
		p = &packet.Packet{}
	case "vmware":
		p = &vmware.VMware{}
	default:
		return nil, errors.Errorf("platform not supported: %s", platform)
	}

	return p, nil
}
