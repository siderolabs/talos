// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// capabilityAll is the wildcard accepted in the drop list.
const capabilityAll = "ALL"

// ContainerSecurity configures the container's security posture.
type ContainerSecurity struct {
	//   description: |
	//     Security profile.
	//
	//     `restricted` drops all capabilities, allows no device access, and mounts the rootfs
	//     and sysfs read-only. `privileged` grants all grantable capabilities and all devices,
	//     which is what extension services get implicitly.
	//   values:
	//     - restricted
	//     - privileged
	//   schema:
	//     type: string
	SecurityProfile config.ContainerSecurityProfile `yaml:"profile,omitempty"`
	//   description: |
	//     Linux capabilities to add or drop on top of the profile.
	SecurityCapabilities *ContainerCapabilities `yaml:"capabilities,omitempty"`
	//   description: |
	//     Publishes the container's PID so machined's API can recognize it, and bind-mounts the
	//     machined API socket into the container.
	//
	//     This alone does not grant DAC access to the socket, which is owned by the `apid` user:
	//     reaching it in practice still requires `profile: privileged` or an equivalent capability/
	//     `runAs` grant. Once connected, the container may request any role, same as extension
	//     services; the RPC's own role requirements are what actually gate access.
	SecurityMachinedAccess bool `yaml:"machinedAccess,omitempty"`
}

// ContainerCapabilities adjusts the container's Linux capabilities.
type ContainerCapabilities struct {
	//   description: |
	//     Capabilities to grant, without the `CAP_` prefix.
	//   examples:
	//     - value: '[]string{"NET_ADMIN"}'
	CapabilitiesAddConfig []string `yaml:"add,omitempty"`
	//   description: |
	//     Capabilities to remove, without the `CAP_` prefix. `ALL` removes every capability.
	//   examples:
	//     - value: '[]string{"ALL"}'
	CapabilitiesDropConfig []string `yaml:"drop,omitempty"`
}

// Check interfaces.
var _ config.ContainerSecurityConfig = &ContainerSecurity{}

// Profile implements config.ContainerSecurityConfig interface.
func (s *ContainerSecurity) Profile() config.ContainerSecurityProfile {
	if s.SecurityProfile == "" {
		return config.ContainerSecurityProfileRestricted
	}

	return s.SecurityProfile
}

// CapabilitiesAdd implements config.ContainerSecurityConfig interface.
func (s *ContainerSecurity) CapabilitiesAdd() []string {
	if s.SecurityCapabilities == nil {
		return nil
	}

	return s.SecurityCapabilities.CapabilitiesAddConfig
}

// CapabilitiesDrop implements config.ContainerSecurityConfig interface.
func (s *ContainerSecurity) CapabilitiesDrop() []string {
	if s.SecurityCapabilities == nil {
		return nil
	}

	return s.SecurityCapabilities.CapabilitiesDropConfig
}

// MachinedAccess implements config.ContainerSecurityConfig interface.
func (s *ContainerSecurity) MachinedAccess() bool {
	return s.SecurityMachinedAccess
}

// Validate checks the security settings.
func (s *ContainerSecurity) Validate() error {
	var validationErrors error

	switch s.Profile() {
	case config.ContainerSecurityProfileRestricted, config.ContainerSecurityProfilePrivileged:
	default:
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("unsupported security profile %q, expected restricted or privileged", s.SecurityProfile))
	}

	if s.SecurityCapabilities == nil {
		return validationErrors
	}

	for _, capability := range s.SecurityCapabilities.CapabilitiesAddConfig {
		if capability == capabilityAll {
			validationErrors = errors.Join(validationErrors,
				errors.New("capabilities.add does not accept ALL, use profile: privileged instead"))

			continue
		}

		validationErrors = errors.Join(validationErrors, ValidateCapabilityName("capabilities.add", capability))
	}

	for _, capability := range s.SecurityCapabilities.CapabilitiesDropConfig {
		if capability == capabilityAll {
			continue
		}

		validationErrors = errors.Join(validationErrors, ValidateCapabilityName("capabilities.drop", capability))
	}

	// A capability in both lists is almost certainly a mistake, and the resolution order would be
	// arbitrary. Reject rather than guess. ALL in drop plus a specific add is the documented way
	// to express "only this one".
	for _, capability := range s.SecurityCapabilities.CapabilitiesAddConfig {
		if slices.Contains(s.SecurityCapabilities.CapabilitiesDropConfig, capability) {
			validationErrors = errors.Join(validationErrors,
				fmt.Errorf("capability %q appears in both add and drop", capability))
		}
	}

	return validationErrors
}

func ValidateCapabilityName(field, capability string) error {
	if capability == "" {
		return fmt.Errorf("%s: capability name must not be empty", field)
	}

	if strings.HasPrefix(capability, "CAP_") {
		return fmt.Errorf("%s: capability %q must be given without the CAP_ prefix", field, capability)
	}

	if strings.ToUpper(capability) != capability {
		return fmt.Errorf("%s: capability %q must be upper case", field, capability)
	}

	if strings.ContainsFunc(capability, func(r rune) bool {
		switch {
		case r >= 'A' && r <= 'Z':
			return false
		case r == '_':
			return false
		default:
			return true
		}
	}) {
		return fmt.Errorf("%s: capability %q may only contain upper case letters and underscores", field, capability)
	}

	return nil
}
