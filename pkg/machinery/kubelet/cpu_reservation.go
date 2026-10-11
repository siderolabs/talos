// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubelet

import (
	"fmt"
	"maps"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// ValidateCPUReservation rejects overrides which can undo a staged pod CPU boundary.
// Callers must not apply these ownership rules to unmanaged kubelet configuration.
func ValidateCPUReservation(extra map[string]any, args map[string][]string, mounts []specs.Mount) error {
	normalized, err := normalizedCPUArguments(args)
	if err != nil {
		return err
	}

	effective := maps.Clone(extra)
	if len(normalized["cpu-manager-policy"]) > 0 {
		delete(effective, "cpuManagerPolicy")
	}

	if len(normalized["cpu-manager-policy-options"]) > 0 {
		delete(effective, "cpuManagerPolicyOptions")
	}

	if err := validateCPUConfiguration(effective); err != nil {
		return err
	}

	if err := validateCPUArguments(args); err != nil {
		return err
	}

	for _, mount := range mounts {
		dest := filepath.Clean(mount.Destination)

		config := "/etc/kubernetes/kubelet.yaml"
		if dest == config || dest == "/" || strings.HasPrefix(config, dest+"/") {
			return fmt.Errorf("kubelet mount %q hides CPU reservation configuration", mount.Destination)
		}
	}

	return nil
}

func validateCPUConfiguration(extra map[string]any) error {
	if _, present := extra["reservedSystemCPUs"]; present {
		return fmt.Errorf("reservedSystemCPUs is owned by CPUPartitionConfig")
	}

	switch extra["cpuManagerPolicy"] {
	case nil, "", "static":
	default:
		return fmt.Errorf("CPUPartitionConfig requires cpuManagerPolicy static")
	}

	if options, present := extra["cpuManagerPolicyOptions"]; present {
		var strict any

		switch options := options.(type) {
		case nil:
			return nil
		case map[string]any:
			strict = options["strict-cpu-reservation"]
		case map[string]string:
			strict = options["strict-cpu-reservation"]
		default:
			return fmt.Errorf("invalid cpuManagerPolicyOptions")
		}

		if strict != nil && !strictReservation(strict) {
			return fmt.Errorf("CPUPartitionConfig requires strict-cpu-reservation=true")
		}
	}

	return nil
}

func normalizedCPUArguments(args map[string][]string) (map[string][]string, error) {
	normalized := make(map[string][]string, len(args))
	for key, values := range args {
		name := strings.ReplaceAll(key, "_", "-")
		switch name {
		case "reserved-cpus", "config", "cgroup-root", "kubelet-cgroups", "config-dir", "cpu-manager-policy", "cpu-manager-policy-options", "container-runtime-endpoint", "feature-gates":
		default:
			continue
		}

		if _, present := normalized[name]; present {
			return nil, fmt.Errorf("duplicate kubelet argument %q", name)
		}

		normalized[name] = values
	}

	return normalized, nil
}

func validateCPUArguments(args map[string][]string) error {
	normalized, err := normalizedCPUArguments(args)
	if err != nil {
		return err
	}

	for _, name := range []string{"reserved-cpus", "config", "cgroup-root", "kubelet-cgroups", "container-runtime-endpoint"} {
		if _, present := normalized[name]; present {
			return fmt.Errorf("kubelet argument %q is owned by CPUPartitionConfig", name)
		}
	}

	if values := normalized["config-dir"]; len(values) > 0 && values[len(values)-1] != "" {
		return fmt.Errorf("kubelet config-dir can override CPUPartitionConfig")
	}

	if values := normalized["cpu-manager-policy"]; len(values) > 0 && values[len(values)-1] != "static" {
		return fmt.Errorf("CPUPartitionConfig requires cpu-manager-policy=static")
	}

	if values, present := normalized["cpu-manager-policy-options"]; present {
		return validateCPUOptionArguments(values)
	}

	return nil
}

func validateCPUOptionArguments(values []string) error {
	// Kubelet's MapStringStringNoSplit clears config-file options on first use.
	options := map[string]string{}

	for _, value := range values {
		key, value, ok := strings.Cut(value, "=")
		if !ok {
			return fmt.Errorf("invalid cpu-manager-policy-options argument")
		}

		options[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	if strict, present := options["strict-cpu-reservation"]; present && !strictReservation(strict) {
		return fmt.Errorf("cpu-manager-policy-options must preserve strict-cpu-reservation=true")
	}

	return nil
}

// CPUReservationOptions applies kubelet's first-CLI-occurrence replacement semantics.
func CPUReservationOptions(config map[string]string, args map[string][]string) (map[string]string, error) {
	normalized, err := normalizedCPUArguments(args)
	if err != nil {
		return nil, err
	}

	options := maps.Clone(config)
	if options == nil {
		options = map[string]string{}
	}

	if values := normalized["cpu-manager-policy-options"]; len(values) > 0 {
		options = map[string]string{}

		for _, value := range values {
			key, value, ok := strings.Cut(value, "=")
			if !ok {
				return nil, fmt.Errorf("invalid cpu-manager-policy-options argument")
			}

			options[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}

	if strict, present := options["strict-cpu-reservation"]; present && !strictReservation(strict) {
		return nil, fmt.Errorf("CPUPartitionConfig requires strict-cpu-reservation=true")
	}

	options["strict-cpu-reservation"] = "true"

	return options, nil
}

// ValidateCPUReservationGates checks the gate required by strict reservation before v1.35.
func ValidateCPUReservationGates(minor uint64, gates map[string]bool, args map[string][]string) error {
	if minor >= 35 {
		return nil
	}

	if minor < 33 {
		return fmt.Errorf("managed CPU reservation requires Kubernetes v1.33 or newer")
	}

	normalized, err := normalizedCPUArguments(args)
	if err != nil {
		return err
	}

	effective, err := cpuFeatureGates(gates, normalized["feature-gates"])
	if err != nil {
		return err
	}

	if enabled, present := effective["CPUManagerPolicyBetaOptions"]; present && !enabled {
		return fmt.Errorf("strict-cpu-reservation requires CPUManagerPolicyBetaOptions before Kubernetes v1.35")
	}

	return nil
}

func cpuFeatureGates(config map[string]bool, values []string) (map[string]bool, error) {
	if len(values) == 0 {
		return config, nil
	}

	gates := maps.Clone(config)
	if gates == nil {
		gates = map[string]bool{}
	}

	for _, value := range values {
		for entry := range strings.SplitSeq(value, ",") {
			if entry == "" {
				continue
			}

			key, value, ok := strings.Cut(entry, "=")
			if !ok {
				return nil, fmt.Errorf("invalid kubelet feature-gates argument")
			}

			enabled, err := strconv.ParseBool(strings.TrimSpace(value))
			if err != nil {
				return nil, err
			}

			gates[strings.TrimSpace(key)] = enabled
		}
	}

	return gates, nil
}

func strictReservation(value any) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}

	enabled, err := strconv.ParseBool(text)

	return err == nil && enabled
}
