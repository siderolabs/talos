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
	//     `restricted` drops all capabilities, allows no device access, and mounts sysfs
	//     read-only. `privileged` grants all grantable capabilities and all devices, and mounts
	//     sysfs read-write.
	//
	//     The profile also sets the default of `readOnlyRootFS`.
	//   values:
	//     - restricted
	//     - privileged
	//   schema:
	//     type: string
	SecurityProfile config.ContainerSecurityProfile `yaml:"profile,omitempty"`
	//   description: |
	//     Linux capabilities to add or drop on top of the profile.
	//
	//     When the container runs as a non-root user (through `runAs` or the image's `USER`), the
	//     resulting capabilities (the profile's, minus `drop`, plus `add`) are also raised into the
	//     ambient set, so that they remain effective after the entrypoint is executed. Ambient
	//     capabilities are inherited by every program the container runs, not only its entrypoint,
	//     so grant only what the workload needs.
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
	//   description: |
	//     Seccomp profile applied to the container.
	//
	//     Unset means the containerd default seccomp profile, independently of the security
	//     profile: even a `privileged` container is filtered by it.
	//   examples:
	//     - value: exampleContainerSeccompLocalhost()
	//   schema:
	//     oneOf:
	//       - type: string
	//         enum:
	//           - default
	//           - unconfined
	//       - $ref: "#/$defs/container.ContainerSeccomp"
	SecuritySeccompProfile *ContainerSeccomp `yaml:"seccompProfile,omitempty" merge:"replace"`

	//   description: |
	//     Mounts the container's root filesystem read-only.
	//
	//     Defaults to `true` with `profile: restricted`, and to `false` with `profile: privileged`.
	//     Paths the workload writes to need a `tmpfs` or volume mount when the root filesystem is
	//     read-only.
	//
	//     This only affects the root filesystem: sysfs stays read-only unless `profile` is
	//     `privileged`, regardless of this setting.
	//   examples:
	//     - value: false
	SecurityReadOnlyRootFS *bool `yaml:"readOnlyRootFS,omitempty"`
}

// ContainerSeccomp selects the seccomp filter applied to the container.
//
// For compatibility, a bare profile type (e.g. `seccompProfile: unconfined`) is accepted as a
// shorthand for `seccompProfile: {type: unconfined}`.
type ContainerSeccomp struct {
	//   description: |
	//     Seccomp profile type.
	//
	//     `default` applies the containerd default seccomp profile. `unconfined` applies no
	//     seccomp filter at all, which some workloads need, e.g. a nested container runtime
	//     creating session keyrings. `localhost` applies the profile named by `localhostProfile`.
	//   values:
	//     - default
	//     - unconfined
	//     - localhost
	//   schema:
	//     type: string
	SeccompType config.ContainerSeccompProfile `yaml:"type,omitempty"`
	//   description: |
	//     Name of the seccomp profile to apply, declared by a `SeccompProfileConfig` document
	//     (or the deprecated `.machine.seccompProfiles` field).
	//
	//     Required with, and only allowed with, `type: localhost`. Updating the referenced
	//     profile restarts the container with the new profile.
	//   examples:
	//     - value: '"audit.json"'
	SeccompLocalhostProfile string `yaml:"localhostProfile,omitempty"`
}

// UnmarshalYAML implements yaml.Unmarshaler, accepting the bare profile type shorthand.
//
// The callback form keeps the strict decoding of the config loader, so unknown fields are still rejected.
func (s *ContainerSeccomp) UnmarshalYAML(unmarshal func(any) error) error {
	var profileType config.ContainerSeccompProfile

	if err := unmarshal(&profileType); err == nil {
		*s = ContainerSeccomp{SeccompType: profileType}

		return nil
	}

	type rawContainerSeccomp ContainerSeccomp

	return unmarshal((*rawContainerSeccomp)(s))
}

func exampleContainerSeccompLocalhost() *ContainerSeccomp {
	return &ContainerSeccomp{
		SeccompType:             config.ContainerSeccompProfileLocalhost,
		SeccompLocalhostProfile: "audit.json",
	}
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

// ReadOnlyRootFS implements config.ContainerSecurityConfig interface.
func (s *ContainerSecurity) ReadOnlyRootFS() bool {
	if s.SecurityReadOnlyRootFS == nil {
		return s.Profile() == config.ContainerSecurityProfileRestricted
	}

	return *s.SecurityReadOnlyRootFS
}

// SeccompProfile implements config.ContainerSecurityConfig interface.
func (s *ContainerSecurity) SeccompProfile() config.ContainerSeccompProfile {
	if s.SecuritySeccompProfile == nil || s.SecuritySeccompProfile.SeccompType == "" {
		return config.ContainerSeccompProfileDefault
	}

	return s.SecuritySeccompProfile.SeccompType
}

// SeccompLocalhostProfile implements config.ContainerSecurityConfig interface.
func (s *ContainerSecurity) SeccompLocalhostProfile() string {
	if s.SecuritySeccompProfile == nil {
		return ""
	}

	return s.SecuritySeccompProfile.SeccompLocalhostProfile
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

	validationErrors = errors.Join(validationErrors, s.validateSeccomp())

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

// validateSeccomp checks the seccomp profile settings.
func (s *ContainerSecurity) validateSeccomp() error {
	switch s.SeccompProfile() {
	case config.ContainerSeccompProfileDefault, config.ContainerSeccompProfileUnconfined:
		if s.SeccompLocalhostProfile() != "" {
			return fmt.Errorf("seccompProfile.localhostProfile is only allowed with seccompProfile.type: localhost, got %q", s.SeccompProfile())
		}
	case config.ContainerSeccompProfileLocalhost:
		// Whether the profile is declared is checked against the whole configuration.
		if s.SeccompLocalhostProfile() == "" {
			return errors.New("seccompProfile.localhostProfile is required with seccompProfile.type: localhost")
		}
	default:
		return fmt.Errorf("unsupported seccomp profile %q, expected default, unconfined or localhost", s.SeccompProfile())
	}

	return nil
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
