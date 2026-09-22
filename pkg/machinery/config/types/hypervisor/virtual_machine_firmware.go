// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/go-pointer"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// Check interfaces.
var (
	_ config.VirtualMachineFirmwareConfig           = &VirtualMachineFirmware{}
	_ config.VirtualMachineFirmwareSecureBootConfig = &VirtualMachineFirmwareSecureBoot{}
	_ yaml.IsZeroer                                 = VirtualMachineFirmwareSecureBoot{}
)

// VirtualMachineFirmware describes the firmware a virtual machine boots.
type VirtualMachineFirmware struct {
	//   description: |
	//     Firmware the guest boots.
	//
	//     Required, and unchangeable for the life of the guest: a guest installed under one
	//     firmware will not boot under the other. `uefi` is the only option on arm64, and the only
	//     one secure boot can be used with.
	//   values:
	//     - uefi
	//     - bios
	//   schema:
	//     type: string
	//   schemaRequired: true
	FirmwareType config.VirtualMachineFirmwareType `yaml:"type"`
	//   description: |
	//     Secure boot settings.
	//
	//     Optional; secure boot is disabled when this section is omitted.
	SecureBootConfig VirtualMachineFirmwareSecureBoot `yaml:"secureBoot,omitempty"`
}

// VirtualMachineFirmwareSecureBoot describes the secure boot settings of a virtual machine.
type VirtualMachineFirmwareSecureBoot struct {
	//   description: |
	//     Boot the guest with secure boot.
	//
	//     Requires `type: uefi`. Talos keeps a per-virtual-machine variable store alongside the
	//     rest of the machine's identity, seeded from a signed firmware template and preserved for
	//     the life of the guest, as it is where the enrolled keys live.
	//
	//     Optional; defaults to disabled.
	SecureBootEnabled *bool `yaml:"enabled,omitempty"`
}

// IsZero implements yaml.IsZeroer.
func (s VirtualMachineFirmwareSecureBoot) IsZero() bool {
	return s.SecureBootEnabled == nil
}

// Type implements config.VirtualMachineFirmwareConfig interface.
//
// The value is returned as written: it is required, so validation has already rejected an empty
// one, and there is no default to apply.
func (f *VirtualMachineFirmware) Type() config.VirtualMachineFirmwareType {
	return f.FirmwareType
}

// SecureBoot implements config.VirtualMachineFirmwareConfig interface.
func (f *VirtualMachineFirmware) SecureBoot() config.VirtualMachineFirmwareSecureBootConfig {
	return &f.SecureBootConfig
}

// Enabled implements config.VirtualMachineFirmwareSecureBootConfig interface.
func (s *VirtualMachineFirmwareSecureBoot) Enabled() bool {
	return pointer.SafeDeref(s.SecureBootEnabled)
}

// validate checks the firmware settings.
//
// Whether the firmware makes sense for the machine's architecture is deliberately not checked here:
// arm64 has no BIOS, but validation.RuntimeMode carries no architecture, and `talosctl validate`
// runs on a client whose architecture need not match the machine's. That belongs in the controller,
// against the machine the guest will actually run on.
func (f *VirtualMachineFirmware) validate() error {
	var validationErrors error

	switch f.FirmwareType {
	case "":
		validationErrors = errors.Join(validationErrors, errors.New("firmware.type is required"))
	case config.VirtualMachineFirmwareTypeUEFI, config.VirtualMachineFirmwareTypeBIOS:
	default:
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("unsupported firmware.type %q, expected uefi or bios", f.FirmwareType))
	}

	if f.SecureBoot().Enabled() && f.FirmwareType == config.VirtualMachineFirmwareTypeBIOS {
		validationErrors = errors.Join(validationErrors,
			errors.New("firmware.secureBoot.enabled: secure boot requires firmware type uefi"))
	}

	return validationErrors
}
