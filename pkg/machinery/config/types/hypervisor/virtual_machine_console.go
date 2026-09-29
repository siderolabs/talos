// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

//docgen:jsonschema

import (
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// Check interfaces.
var (
	_ config.VirtualMachineConsoleConfig = &VirtualMachineConsole{}
	_ config.VirtualMachineSerialConfig  = &VirtualMachineSerial{}
	_ config.VirtualMachineVNCConfig     = &VirtualMachineVNC{}
)

// VirtualMachineConsole describes the consoles attached to a virtual machine.
type VirtualMachineConsole struct {
	//   description: |
	//     Serial console settings.
	SerialConfig VirtualMachineSerial `yaml:"serial,omitempty"`
	//   description: |
	//     VNC console settings.
	VNCConfig VirtualMachineVNC `yaml:"vnc,omitempty"`
}

// VirtualMachineSerial describes the serial console of a virtual machine.
type VirtualMachineSerial struct {
	//   description: |
	//     Attach a serial console.
	//
	//     Optional; defaults to disabled.
	SerialEnabled *bool `yaml:"enabled,omitempty"`
}

// VirtualMachineVNC describes the VNC console of a virtual machine.
type VirtualMachineVNC struct {
	//   description: |
	//     Attach a VNC console.
	//
	//     Optional; defaults to disabled.
	VNCEnabled *bool `yaml:"enabled,omitempty"`
}

// Serial implements config.VirtualMachineConsoleConfig interface.
func (c *VirtualMachineConsole) Serial() config.VirtualMachineSerialConfig {
	return &c.SerialConfig
}

// VNC implements config.VirtualMachineConsoleConfig interface.
func (c *VirtualMachineConsole) VNC() config.VirtualMachineVNCConfig {
	return &c.VNCConfig
}

// Enabled implements config.VirtualMachineSerialConfig interface.
func (s *VirtualMachineSerial) Enabled() bool {
	return pointer.SafeDeref(s.SerialEnabled)
}

// Enabled implements config.VirtualMachineVNCConfig interface.
func (v *VirtualMachineVNC) Enabled() bool {
	return pointer.SafeDeref(v.VNCEnabled)
}
