// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// Check interfaces.
var (
	_ config.VirtualMachineNetworkingConfig = &VirtualMachineNetworking{}
	_ config.VirtualMachineInterfaceConfig  = &VirtualMachineInterface{}
	_ yaml.IsZeroer                         = VirtualMachineNetworking{}
)

// VirtualMachineNetworking describes the networking of a virtual machine.
type VirtualMachineNetworking struct {
	//   description: |
	//     Network interfaces presented to the guest.
	//
	//     Removing an interface from this list detaches it from the virtual machine.
	//
	//     A configuration patch replaces this list as a whole rather than appending to it.
	InterfacesConfig []VirtualMachineInterface `yaml:"interfaces,omitempty" merge:"replace"`
}

// VirtualMachineInterface describes a single network interface of a virtual machine.
type VirtualMachineInterface struct {
	//   description: |
	//     Name of the interface, unique within the virtual machine.
	//
	//     Must be between 1 and 63 characters long, and can only contain ASCII letters,
	//     digits and hyphens. This is how the interface is addressed over the API; it is not the
	//     device name inside the guest, which the guest kernel chooses for itself.
	//   examples:
	//     - value: '"net0"'
	//   schemaRequired: true
	InterfaceName string `yaml:"name"`
	//   description: |
	//     Kernel name (or alias) of the host link the interface is attached to.
	//
	//     The link must already exist on the host: it is attached to as is, and neither Talos nor
	//     the hypervisor configures networking for it.
	//   examples:
	//     - value: '"eth0"'
	//   schemaRequired: true
	InterfaceLink string `yaml:"link"`
}

// IsZero implements yaml.IsZeroer.
func (n VirtualMachineNetworking) IsZero() bool {
	return len(n.InterfacesConfig) == 0
}

// Interfaces implements config.VirtualMachineNetworkingConfig interface.
func (n *VirtualMachineNetworking) Interfaces() []config.VirtualMachineInterfaceConfig {
	out := make([]config.VirtualMachineInterfaceConfig, 0, len(n.InterfacesConfig))

	for i := range n.InterfacesConfig {
		out = append(out, &n.InterfacesConfig[i])
	}

	return out
}

// Name implements config.VirtualMachineInterfaceConfig interface.
func (i *VirtualMachineInterface) Name() string {
	return i.InterfaceName
}

// Link implements config.VirtualMachineInterfaceConfig interface.
func (i *VirtualMachineInterface) Link() string {
	return i.InterfaceLink
}

// Validate checks the interface and returns its name.
func (i *VirtualMachineInterface) Validate(index int) (string, error) {
	var validationErrors error

	if err := validateName(i.InterfaceName); err != nil {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("networking.interfaces[%d]: %w", index, err))
	}

	validationErrors = errors.Join(validationErrors, validateLink(index, i.InterfaceLink))

	return i.InterfaceName, validationErrors
}

// validateLink checks the shape of the host link name an interface is attached to.
//
// The rules are the kernel's own for a link name; whether the link exists is only known on the
// host, once the virtual machine starts.
func validateLink(index int, link string) error {
	switch {
	case link == "":
		return fmt.Errorf("networking.interfaces[%d]: link is required", index)
	case len(link) > 15:
		return fmt.Errorf("networking.interfaces[%d]: link %q must not exceed 15 bytes", index, link)
	case link == "." || link == "..":
		return fmt.Errorf("networking.interfaces[%d]: link must not be %q", index, link)
	case strings.ContainsAny(link, "/:"):
		return fmt.Errorf("networking.interfaces[%d]: link %q must not contain '/' or ':'", index, link)
	}

	for _, r := range link {
		if !unicode.IsPrint(r) || unicode.IsSpace(r) {
			return fmt.Errorf("networking.interfaces[%d]: link %q must not contain whitespace or non-printable characters", index, link)
		}
	}

	return nil
}
