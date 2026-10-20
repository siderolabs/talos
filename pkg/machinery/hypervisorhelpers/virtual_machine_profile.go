// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

import (
	"errors"
	"fmt"
	"strings"
)

// VirtualMachineProfileFeature is a named CPU flag and its libvirt policy.
type VirtualMachineProfileFeature struct {
	Name   string
	Policy VirtualMachineProfileCPUFeaturePolicy
}

// ValidVirtualMachineProfileMachine checks the amd64 QEMU machine families.
func ValidVirtualMachineProfileMachine(machine string) bool {
	if machine == "q35" || machine == "pc" {
		return true
	}

	for _, prefix := range []string{"pc-q35-", "pc-i440fx-"} {
		if suffix, ok := strings.CutPrefix(machine, prefix); ok && suffix != "" && !strings.ContainsAny(suffix, " /\\\t\n\r<>&\"'") {
			return true
		}
	}

	return false
}

// ValidateVirtualMachineProfileCPU checks structural constraints without probing a host.
func ValidateVirtualMachineProfileCPU(mode VirtualMachineProfileCPUMode, model, fallback string, features []VirtualMachineProfileFeature) error {
	return errors.Join(
		validateVirtualMachineProfileExposure(mode, model, fallback, features),
		validateVirtualMachineProfileFeatures(features),
	)
}

func validateVirtualMachineProfileExposure(mode VirtualMachineProfileCPUMode, model, fallback string, features []VirtualMachineProfileFeature) error {
	var errs error

	switch mode {
	case VirtualMachineProfileCPUModeHostModel, VirtualMachineProfileCPUModeHostPassthrough:
		if model != "" {
			errs = errors.Join(errs, errors.New("cpu.model is only allowed with custom mode"))
		}

		if fallback != "" {
			errs = errors.Join(errs, errors.New("cpu.fallback is only allowed with custom mode"))
		}

		if len(features) != 0 {
			errs = errors.Join(errs, errors.New("cpu.features are only allowed with custom mode"))
		}
	case VirtualMachineProfileCPUModeCustom:
		if model == "" || strings.ContainsAny(model, " /\\\t\n\r<>&\"'") {
			errs = errors.Join(errs, errors.New("cpu.model must be a nonempty model name"))
		}

		switch fallback {
		case "", "forbid", "allow":
		default:
			errs = errors.Join(errs, fmt.Errorf("cpu.fallback %q is invalid", fallback))
		}
	case VirtualMachineProfileCPUModeUnknown:
		errs = errors.Join(errs, fmt.Errorf("cpu.mode %q must be host-model, host-passthrough or custom", mode.String()))
	default:
		errs = errors.Join(errs, fmt.Errorf("cpu.mode %q must be host-model, host-passthrough or custom", mode.String()))
	}

	return errs
}

func validateVirtualMachineProfileFeatures(features []VirtualMachineProfileFeature) error {
	var errs error

	seen := map[string]struct{}{}

	for i, feature := range features {
		if feature.Name == "" || strings.ContainsAny(feature.Name, " /\\\t\n\r<>&\"'") {
			errs = errors.Join(errs, fmt.Errorf("cpu.features[%d].name is invalid", i))
		}

		if _, exists := seen[feature.Name]; exists {
			errs = errors.Join(errs, fmt.Errorf("cpu.features[%d].name is duplicated", i))
		}

		seen[feature.Name] = struct{}{}
		switch feature.Policy {
		case VirtualMachineProfileCPUFeaturePolicyRequire,
			VirtualMachineProfileCPUFeaturePolicyDisable,
			VirtualMachineProfileCPUFeaturePolicyOptional,
			VirtualMachineProfileCPUFeaturePolicyForce,
			VirtualMachineProfileCPUFeaturePolicyForbid:
		case VirtualMachineProfileCPUFeaturePolicyUnknown:
			errs = errors.Join(errs, fmt.Errorf("cpu.features[%d].policy %q is invalid", i, feature.Policy.String()))
		default:
			errs = errors.Join(errs, fmt.Errorf("cpu.features[%d].policy %q is invalid", i, feature.Policy.String()))
		}
	}

	return errs
}
