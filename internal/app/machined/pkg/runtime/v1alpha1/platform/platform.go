// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package platform provides functions to get the [runtime.Platform].
package platform

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/akamai"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/aws"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/azure"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/cloudstack"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/container"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/digitalocean"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/equinixmetal"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/exoscale"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/gcp"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/hcloud"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/nocloud"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/opennebula"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/openstack"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/oracle"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/scaleway"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/upcloud"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/vmware"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/vultr"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Event is a struct used below in FireEvent
// in hopes that we can reuse some of this eventing in other platforms if possible.
type Event struct {
	Type    string
	Message string
	Error   error
}

// nb: these events currently map to those expected by
// equinix metal. if/when we do other platforms, we should
// maybe generalize this and map the events inside each platform.
const (
	// EventTypeActivate is the activate event string.
	EventTypeActivate = "activate"
	// EventTypeFailure is the failure event string.
	EventTypeFailure = "failure"
	// EventTypeInfo is the info event string.
	EventTypeInfo = "info"
	// EventTypeConfigLoaded is the config loaded event string.
	EventTypeConfigLoaded = "talos.prov.config.loaded"
	// EventTypeRebooted is the reboot event string.
	EventTypeRebooted = "talos.prov.host.rebooted"
	// EventTypeInstalled is the installation event string.
	EventTypeInstalled = "talos.prov.os.installed"
	// EventTypeUpgraded is the upgrade event string.
	EventTypeUpgraded = "talos.prov.os.upgraded"
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

//nolint:gocyclo,cyclop
func newPlatform(platform string) (p runtime.Platform, err error) {
	switch platform {
	case "akamai":
		p = &akamai.Akamai{}
	case "aws":
		return aws.NewAWS()
	case "azure":
		p = &azure.Azure{}
	case "cloudstack":
		p = &cloudstack.Cloudstack{}
	case "container":
		p = &container.Container{}
	case "digital-ocean":
		p = &digitalocean.DigitalOcean{}
	case "gcp":
		p = &gcp.GCP{}
	case "hcloud":
		p = &hcloud.Hcloud{}
	case constants.PlatformMetal:
		p = &metal.Metal{}
	case "opennebula":
		p = &opennebula.OpenNebula{}
	case "openstack":
		p = &openstack.OpenStack{}
	case "oracle":
		p = &oracle.Oracle{}
	case "nocloud":
		p = &nocloud.Nocloud{}
	// "packet" kept for backwards compatibility
	case "equinixMetal", "packet":
		p = &equinixmetal.EquinixMetal{}
	case "exoscale":
		p = &exoscale.Exoscale{}
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

// FireEvent will call the implemented platform's event function if we know it has one.
// Error logging is handled in this function and we don't return any error values to the sequencer.
func FireEvent(ctx context.Context, p runtime.Platform, e Event) {
	switch platType := p.(type) {
	case *equinixmetal.EquinixMetal:
		emEvent := equinixmetal.Event{
			Type:    e.Type,
			Message: e.Message,
		}

		if e.Error != nil {
			emEvent.Message = fmt.Sprintf("%s: %s", e.Message, e.Error)
		}

		eventErr := platType.FireEvent(ctx, emEvent)

		if eventErr != nil {
			log.Printf("failed sending event: %s", eventErr)
		}

	default:
		// Treat anything else as a no-op b/c we don't support event firing
		return
	}
}
