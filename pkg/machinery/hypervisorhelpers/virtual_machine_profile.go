// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// VirtualMachineProfileFeature is a named CPU flag and its libvirt policy.
type VirtualMachineProfileFeature struct {
	Name   string
	Policy VirtualMachineProfileCPUFeaturePolicy
}

// ValidVirtualMachineProfileMachine checks the QEMU machine families of the given GOARCH:
// q35 and i440fx on amd64, virt on arm64.
func ValidVirtualMachineProfileMachine(arch, machine string) bool {
	var aliases, prefixes []string

	switch arch {
	case constants.ArchAmd64:
		aliases, prefixes = []string{"q35", "pc"}, []string{"pc-q35-", "pc-i440fx-"}
	case constants.ArchArm64:
		aliases, prefixes = []string{"virt"}, []string{"virt-"}
	default:
		return false
	}

	if slices.Contains(aliases, machine) {
		return true
	}

	for _, prefix := range prefixes {
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

// Profile timer defaults, applied when a profile omits a timer setting and used by every builtin profile.
// They follow virt-install's x86 KVM defaults. Changing one changes the guest ABI of every such profile.
const (
	// DefaultVirtualMachineProfileHPET hides the emulated HPET: QEMU keeps its periodic timer running
	// after the guest stops using it, waking the main thread at the guest's tick rate.
	DefaultVirtualMachineProfileHPET = false
	// DefaultVirtualMachineProfileRTCTickPolicy catches up on missed RTC ticks: guests without an HPET,
	// such as Windows, keep time from the RTC and would otherwise drift.
	DefaultVirtualMachineProfileRTCTickPolicy = VirtualMachineProfileRTCTickPolicyCatchup
	// DefaultVirtualMachineProfilePITTickPolicy delays missed PIT ticks, the in-kernel KVM PIT's default.
	DefaultVirtualMachineProfilePITTickPolicy = VirtualMachineProfilePITTickPolicyDelay
)

// ValidateVirtualMachineProfileTimers checks resolved timer tick policies.
// Unknown is rejected: profile specs always carry an explicit policy.
func ValidateVirtualMachineProfileTimers(rtc VirtualMachineProfileRTCTickPolicy, pit VirtualMachineProfilePITTickPolicy) error {
	var errs error

	switch rtc {
	case VirtualMachineProfileRTCTickPolicyCatchup, VirtualMachineProfileRTCTickPolicyDelay:
	case VirtualMachineProfileRTCTickPolicyUnknown:
		fallthrough
	default:
		errs = errors.Join(errs, fmt.Errorf("timers.rtc.tickPolicy %q must be catchup or delay", rtc.String()))
	}

	switch pit {
	case VirtualMachineProfilePITTickPolicyDelay, VirtualMachineProfilePITTickPolicyDiscard:
	case VirtualMachineProfilePITTickPolicyUnknown:
		fallthrough
	default:
		errs = errors.Join(errs, fmt.Errorf("timers.pit.tickPolicy %q must be delay or discard", pit.String()))
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
