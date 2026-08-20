// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"slices"

	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/containerd/containerd/v2/pkg/oci"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// capabilityAll is the set-wide value accepted in capabilities.drop.
const capabilityAll = "ALL"

// ContainerSecuritySpec adapter provides translation to OCI spec options.
//
//nolint:revive
func ContainerSecuritySpec(r *containers.ContainerSecuritySpec) containerSecuritySpec {
	return containerSecuritySpec{
		ContainerSecuritySpec: r,
	}
}

type containerSecuritySpec struct {
	*containers.ContainerSecuritySpec
}

// OCISpecOpts builds OCI spec opts for the security spec.
func (a containerSecuritySpec) OCISpecOpts(grantableCapabilities []string) []oci.SpecOpts {
	spec := a.ContainerSecuritySpec

	var opts []oci.SpecOpts

	if spec.Privileged {
		// Extension-service-level permissions: all grantable capabilities and all devices.
		opts = append(opts,
			oci.WithCapabilities(grantableCapabilities),
			oci.WithAllDevicesAllowed,
		)
	} else {
		// Restricted default: no capabilities, read-only rootfs.
		opts = append(opts,
			oci.WithCapabilities(nil),
			oci.WithRootFSReadonly(),
		)
	}

	// ALL is a set operation rather than a name, so it cannot go through WithDroppedCapabilities,
	// which only removes exact matches and would silently drop nothing at all. Clearing the set
	// outright is what makes "drop ALL plus add X" mean "only X", which is how the configuration
	// documents expressing exactly one capability.
	if slices.Contains(spec.CapabilitiesDrop, capabilityAll) {
		opts = append(opts, oci.WithCapabilities(nil))
	} else if len(spec.CapabilitiesDrop) > 0 {
		opts = append(opts, oci.WithDroppedCapabilities(PrefixCapabilities(spec.CapabilitiesDrop)))
	}

	if len(spec.CapabilitiesAdd) > 0 {
		opts = append(opts, oci.WithAddedCapabilities(PrefixCapabilities(spec.CapabilitiesAdd)))
	}

	return opts
}

// PrefixCapabilities restores the CAP_ prefix the configuration deliberately omits.
func PrefixCapabilities(capabilities []string) []string {
	out := make([]string, 0, len(capabilities))

	for _, c := range capabilities {
		out = append(out, "CAP_"+c)
	}

	return out
}

// ContainerResourcesSpec adapter provides translation to cgroup v2 resources.
//
//nolint:revive
func ContainerResourcesSpec(r *containers.ContainerResourcesSpec) containerResourcesSpec {
	return containerResourcesSpec{
		ContainerResourcesSpec: r,
	}
}

type containerResourcesSpec struct {
	*containers.ContainerResourcesSpec
}

// CgroupResources translates the resolved limits into cgroup v2 resources.
func (a containerResourcesSpec) CgroupResources() *cgroup2.Resources {
	spec := a.ContainerResourcesSpec

	cgroupResources := &cgroup2.Resources{}

	if spec.MemoryLimit > 0 {
		cgroupResources.Memory = &cgroup2.Memory{
			Max: new(int64(spec.MemoryLimit)), //nolint:gosec
		}
	}

	if spec.CPULimit > 0 {
		// cpu.max is a quota over a period, unlike the weight used elsewhere in Talos: this is a
		// ceiling, not a share.
		const period = 100000

		quota := int64(spec.CPULimit) * period / 1000 //nolint:gosec

		cgroupResources.CPU = &cgroup2.CPU{
			Max: cgroup2.NewCPUMax(&quota, new(uint64(period))),
		}
	}

	return cgroupResources
}
