// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// Check interfaces.
var (
	_ config.VirtualMachineGuestConfig     = &VirtualMachineGuest{}
	_ config.VirtualMachineCloudInitConfig = &VirtualMachineCloudInit{}
	_ config.VirtualMachineAgentConfig     = &VirtualMachineAgent{}
	_ yaml.IsZeroer                        = VirtualMachineGuest{}
)

// VirtualMachineGuest describes the settings which apply inside the guest.
type VirtualMachineGuest struct {
	//   description: |
	//     Seed handed to the guest on first boot.
	CloudInitConfig *VirtualMachineCloudInit `yaml:"cloudInit,omitempty"`
	//   description: |
	//     qemu-guest-agent settings.
	//
	//     Optional; the agent channel is not attached when this section is omitted.
	AgentConfig *VirtualMachineAgent `yaml:"agent,omitempty"`
}

// VirtualMachineAgent describes the qemu-guest-agent settings for a virtual machine.
type VirtualMachineAgent struct {
	//   description: |
	//     Attach the qemu-guest-agent virtio channel.
	//
	//     Without the agent, stopping a virtual machine is ACPI-or-destroy, and status cannot
	// 	   report the addresses the guest holds. The agent has to be installed and running inside
	//     the guest for the channel to be of any use.
	//
	//     Optional; defaults to disabled.
	AgentEnabled *bool `yaml:"enabled,omitempty"`
}

// VirtualMachineCloudInit describes the NoCloud seed handed to the guest.
type VirtualMachineCloudInit struct {
	//   description: |
	//     Contents of the seed's `meta-data` file, carrying the guest's identity.
	//
	//     `instance-id` is what decides whether a boot is a reboot or a new instance. An unchanged
	//     id means edits to `userData` are inert; a changed id re-runs provisioning, which
	//     regenerates the SSH host keys in most images.
	//
	//     Must be valid YAML.
	//   examples:
	//     - value: '"instance-id: vm1-001\nlocal-hostname: vm1\n"'
	MetaDataConfig string `yaml:"metaData,omitempty"`
	//   description: |
	//     Contents of the seed's `user-data` file, carrying what the operator wants done.
	//
	//     For a distro image this is a cloud-init document, usually starting with `#cloud-config`,
	//     though a script or a MIME archive is equally valid -- it is not parsed here. For a Talos
	//     guest this is the guest's own machine configuration.
	//
	//     Carries SSH keys, passwords and tokens in practice, and is redacted from the
	//     configuration as read back over the API.
	UserDataConfig string `yaml:"userData,omitempty"`
	//   description: |
	//     Contents of the seed's `network-config` file, carrying the guest's network settings.
	//
	//     Needed by guests which cannot configure themselves over DHCP. Unset leaves the guest to
	//     its own defaults.
	//
	//     Must be valid YAML.
	NetworkConfigConfig string `yaml:"networkConfig,omitempty"`
}

// IsZero implements yaml.IsZeroer.
func (g VirtualMachineGuest) IsZero() bool {
	return g.CloudInitConfig == nil && g.AgentConfig == nil
}

// CloudInit implements config.VirtualMachineGuestConfig interface.
func (g *VirtualMachineGuest) CloudInit() optional.Optional[config.VirtualMachineCloudInitConfig] {
	if g.CloudInitConfig == nil {
		return optional.None[config.VirtualMachineCloudInitConfig]()
	}

	return optional.Some[config.VirtualMachineCloudInitConfig](g.CloudInitConfig)
}

// Agent implements config.VirtualMachineGuestConfig interface.
func (g *VirtualMachineGuest) Agent() config.VirtualMachineAgentConfig {
	if g.AgentConfig == nil {
		return &VirtualMachineAgent{}
	}

	return g.AgentConfig
}

// Enabled implements config.VirtualMachineAgentConfig interface.
func (a *VirtualMachineAgent) Enabled() bool {
	return pointer.SafeDeref(a.AgentEnabled)
}

// MetaData implements config.VirtualMachineCloudInitConfig interface.
func (c *VirtualMachineCloudInit) MetaData() string {
	return c.MetaDataConfig
}

// UserData implements config.VirtualMachineCloudInitConfig interface.
func (c *VirtualMachineCloudInit) UserData() string {
	return c.UserDataConfig
}

// NetworkConfig implements config.VirtualMachineCloudInitConfig interface.
func (c *VirtualMachineCloudInit) NetworkConfig() string {
	return c.NetworkConfigConfig
}

// Redact replaces the secrets carried by the guest seed.
func (g *VirtualMachineGuest) Redact(replacement string) {
	if g.CloudInitConfig == nil {
		return
	}

	if g.CloudInitConfig.UserDataConfig != "" {
		g.CloudInitConfig.UserDataConfig = replacement
	}
}

// validate checks the guest settings, returning any warnings alongside the errors.
func (g *VirtualMachineGuest) validate() ([]string, error) {
	if g.CloudInitConfig == nil {
		return nil, nil
	}

	cloudInit := g.CloudInitConfig

	if cloudInit.MetaDataConfig == "" && cloudInit.UserDataConfig == "" && cloudInit.NetworkConfigConfig == "" {
		return nil, errors.New("guest.cloudInit: at least one of metaData, userData or networkConfig must be set")
	}

	var (
		validationErrors error
		warnings         []string
	)

	hasInstanceID, err := validateMetaData(cloudInit.MetaDataConfig)
	validationErrors = errors.Join(validationErrors, err)

	// `user-data` is deliberately not parsed: a script or a MIME archive is as valid there as a
	// cloud-config document, and only the latter is YAML.
	validationErrors = errors.Join(validationErrors, validateSeedYAML("guest.cloudInit.networkConfig", cloudInit.NetworkConfigConfig))

	if err == nil && !hasInstanceID {
		warnings = append(warnings,
			"guest.cloudInit: no instance-id, so cloud-init cannot tell a reboot from a new instance and edits to userData may not take effect")
	}

	return warnings, validationErrors
}

// validateMetaData checks the seed's `meta-data` file and reports whether it carries an
// instance-id.
//
// cloud-init reads the instance-id from nowhere else, so an absent `meta-data` is an absent
// instance-id.
func validateMetaData(contents string) (bool, error) {
	if contents == "" {
		return false, nil
	}

	var parsed any

	if err := yaml.Unmarshal([]byte(contents), &parsed); err != nil {
		return false, fmt.Errorf("guest.cloudInit.metaData: must be valid YAML: %w", err)
	}

	mapping, ok := parsed.(map[string]any)
	if !ok {
		return false, errors.New("guest.cloudInit.metaData: must be a YAML mapping")
	}

	_, hasInstanceID := mapping["instance-id"]

	return hasInstanceID, nil
}

// validateSeedYAML checks that a seed file which has to be YAML parses as YAML.
func validateSeedYAML(path, contents string) error {
	if contents == "" {
		return nil
	}

	var parsed any

	if err := yaml.Unmarshal([]byte(contents), &parsed); err != nil {
		return fmt.Errorf("%s: must be valid YAML: %w", path, err)
	}

	return nil
}
