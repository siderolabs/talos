// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package profile owns builtin VM hardware contracts and resource-level profile validation.
package profile

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// ErrNotReady marks a profile that has not been published or is being removed.
var ErrNotReady = errors.New("profile not ready")

// Validate checks a published profile, including the identity of builtin profiles.
func Validate(name string, spec hypervisor.VirtualMachineProfileSpecSpec) error {
	if !spec.Builtin {
		return ValidateCustom(name, spec)
	}

	for _, builtin := range Profiles() {
		if builtin.Metadata().ID() != name {
			continue
		}

		expected := builtin.TypedSpec()
		if spec.Machine != expected.Machine ||
			spec.CPU.Mode != expected.CPU.Mode ||
			spec.CPU.Model != expected.CPU.Model ||
			spec.CPU.Fallback != expected.CPU.Fallback ||
			!slices.Equal(spec.CPU.Features, expected.CPU.Features) {
			return fmt.Errorf("builtin profile %q does not match the builtin hardware contract", name)
		}

		return nil
	}

	return fmt.Errorf("unknown builtin profile %q", name)
}

// ValidateCustom checks a custom profile before publishing or rendering it.
// It deliberately does not probe host capabilities or promise migration compatibility.
func ValidateCustom(name string, spec hypervisor.VirtualMachineProfileSpecSpec) error {
	var errs error

	if err := hypervisorhelpers.ValidateName(name); err != nil {
		errs = errors.Join(errs, fmt.Errorf("profile name %q: %w", name, err))
	}

	if name == "host" || name == "legacy" || name == "passthrough" || strings.HasPrefix(name, "compat-") {
		errs = errors.Join(errs, fmt.Errorf("profile name %q is reserved", name))
	}

	if spec.Builtin {
		errs = errors.Join(errs, errors.New("custom profiles cannot be builtin"))
	}

	if !validMachine(spec.Machine) {
		errs = errors.Join(errs, fmt.Errorf("machine %q is unsupported on this architecture", spec.Machine))
	}

	features := make([]hypervisorhelpers.VirtualMachineProfileFeature, 0, len(spec.CPU.Features))
	for _, feature := range spec.CPU.Features {
		features = append(features, hypervisorhelpers.VirtualMachineProfileFeature{
			Name:   feature.Name,
			Policy: feature.Policy,
		})
	}

	return errors.Join(errs, hypervisorhelpers.ValidateVirtualMachineProfileCPU(
		spec.CPU.Mode,
		spec.CPU.Model,
		spec.CPU.Fallback,
		features,
	))
}
