// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubelet

import (
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// memoryLimitArgs maps the kubelet flags which override the managed memory accounting to their configuration field.
//
// Flags override the configuration file, so they are rejected even when the field itself would be accepted.
// An empty field marks a flag whose configuration field is owned by Talos.
var memoryLimitArgs = map[string]string{
	"system-reserved":          "systemReserved",
	"kube-reserved":            "kubeReserved",
	"system-reserved-cgroup":   "systemReservedCgroup",
	"kube-reserved-cgroup":     "kubeReservedCgroup",
	"enforce-node-allocatable": "enforceNodeAllocatable",
	"cgroups-per-qos":          "cgroupsPerQOS",
	"cgroup-root":              "",
	"cgroup-driver":            "cgroupDriver",
	"memory-manager-policy":    "memoryManagerPolicy",
	"reserved-memory":          "reservedMemory",
	"eviction-hard":            "evictionHard",
}

const configDirArg = "config-dir"

// ValidateMemoryLimitConfiguration rejects kubelet settings which conflict with a Talos-managed kubepods memory limit.
//
// Talos derives `systemReserved.memory` from the limit and relies on the kubelet enforcing it on the
// `pods` level only; the checks below need no host information and are safe to run at configuration time.
//
//nolint:gocyclo,cyclop
func ValidateMemoryLimitConfiguration(extraConfig map[string]any, extraArgs map[string][]string) error {
	var errs error

	for _, flag := range slices.Sorted(maps.Keys(extraArgs)) {
		normalized := normalizeFlag(flag)

		if normalized == configDirArg {
			errs = errors.Join(errs, fmt.Errorf("kubelet argument %q conflicts with the kubepods memory limit: drop-in files are merged over the derived configuration", flag))

			continue
		}

		field, conflicts := memoryLimitArgs[normalized]
		if !conflicts {
			continue
		}

		if field != "" {
			errs = errors.Join(errs, fmt.Errorf("kubelet argument %q conflicts with the kubepods memory limit: use the %q configuration field instead", flag, field))
		} else {
			errs = errors.Join(errs, fmt.Errorf("kubelet argument %q conflicts with the kubepods memory limit", flag))
		}
	}

	if stringMapContains(extraConfig["systemReserved"], "memory") {
		errs = errors.Join(errs, errors.New(`kubelet configuration field "systemReserved.memory" conflicts with the kubepods memory limit: Talos derives it from the limit`))
	}

	if perQOS, ok := extraConfig["cgroupsPerQOS"].(bool); ok && !perQOS {
		errs = errors.Join(errs, errors.New(`kubelet configuration field "cgroupsPerQOS" must be enabled with the kubepods memory limit`))
	}

	if driver, ok := extraConfig["cgroupDriver"].(string); ok && driver != "cgroupfs" {
		errs = errors.Join(errs, fmt.Errorf(`kubelet configuration field "cgroupDriver" %q conflicts with the kubepods memory limit: only "cgroupfs" is supported`, driver))
	}

	if policy, ok := extraConfig["memoryManagerPolicy"].(string); ok && policy == "Static" {
		errs = errors.Join(errs, errors.New(`kubelet configuration field "memoryManagerPolicy" "Static" is not supported with the kubepods memory limit`))
	}

	enforcement := []string{"pods"}

	if raw, present := extraConfig["enforceNodeAllocatable"]; present {
		enforcement = stringSlice(raw)
	}

	if !slices.Contains(enforcement, "pods") {
		errs = errors.Join(errs, errors.New(`kubelet configuration field "enforceNodeAllocatable" must include "pods" with the kubepods memory limit`))
	}

	if slices.Contains(enforcement, "system-reserved") {
		errs = errors.Join(errs, errors.New(`kubelet configuration field "enforceNodeAllocatable" must not include "system-reserved" with the kubepods memory limit: `+
			`the derived "systemReserved.memory" is not a system daemon budget`))
	}

	if slices.Contains(enforcement, "kube-reserved") {
		if cgroupName, ok := extraConfig["kubeReservedCgroup"].(string); ok && isManagedRoot(cgroupName) {
			errs = errors.Join(errs, fmt.Errorf(`kubelet configuration field "kubeReservedCgroup" %q is a Talos-managed cgroup: `+
				`it cannot be enforced with "kube-reserved" together with the kubepods memory limit`, cgroupName))
		}
	}

	return errs
}

// ValidateWorkloadRootConfiguration also rejects opaque drop-ins while workload limits are active.
func ValidateWorkloadRootConfiguration(extraConfig map[string]any, extraArgs map[string][]string) error {
	errs := ValidateWorkloadRootReservations(extraConfig, extraArgs)

	for _, flag := range slices.Sorted(maps.Keys(extraArgs)) {
		if normalizeFlag(flag) == configDirArg {
			errs = errors.Join(errs, fmt.Errorf("kubelet argument %q conflicts with WorkloadResourceConfig: drop-in files can enforce reservations on the Talos-managed cgroups unseen", flag))
		}
	}

	return errs
}

// ValidateWorkloadRootReservations rejects memory reservations enforced on Talos-managed workload roots.
//
// The kubelet enforces a full `system-reserved`/`kube-reserved` reservation by writing every resource of the
// reservation, memory included, to the named cgroup; a memory reservation enforced on a Talos-managed root
// competes with the limit Talos keeps on it. The compressible variants only write CPU shares and stay allowed.
// Flags override the configuration file, so the effective values are derived from both; an absent or empty
// `systemReserved` is filled in by Talos with a memory reservation, so it counts as reserving memory.
func ValidateWorkloadRootReservations(extraConfig map[string]any, extraArgs map[string][]string) error {
	var errs error

	args := map[string][]string{}

	for _, flag := range slices.Sorted(maps.Keys(extraArgs)) {
		normalized := normalizeFlag(flag)

		args[normalized] = append(args[normalized], extraArgs[flag]...)
	}

	enforcement := effectiveStringSlice(extraConfig, args, "enforceNodeAllocatable", "enforce-node-allocatable")

	for _, reservation := range []struct {
		enforcementKey string
		compressible   string
		field          string
		cgroupField    string
		flag           string
		cgroupFlag     string
		talosDefaulted bool
	}{
		{"system-reserved", "system-reserved-compressible", "systemReserved", "systemReservedCgroup", "system-reserved", "system-reserved-cgroup", true},
		{"kube-reserved", "kube-reserved-compressible", "kubeReserved", "kubeReservedCgroup", "kube-reserved", "kube-reserved-cgroup", false},
	} {
		if !slices.Contains(enforcement, reservation.enforcementKey) {
			continue
		}

		cgroupName, ok := effectiveString(extraConfig, args, reservation.cgroupField, reservation.cgroupFlag)
		if !ok || !isTalosOwnedRoot(cgroupName) {
			continue
		}

		if !effectiveReservesMemory(extraConfig, args, reservation.field, reservation.flag, reservation.talosDefaulted) {
			continue
		}

		errs = errors.Join(errs, fmt.Errorf(`kubelet enforces the %q reservation on the Talos-managed cgroup %q through "enforceNodeAllocatable" %q: `+
			`use %q or a cgroup outside the Talos-managed workload roots`,
			reservation.field+".memory", cgroupName, reservation.enforcementKey, reservation.compressible))
	}

	return errs
}

// normalizeFlag applies the kubelet's flag normalization, which treats '_' and '-' as the same separator.
func normalizeFlag(flag string) string {
	return strings.ReplaceAll(flag, "_", "-")
}

// effectiveString resolves a string setting, with the flag taking precedence over the configuration field.
func effectiveString(extraConfig map[string]any, args map[string][]string, field, flag string) (string, bool) {
	if values, ok := args[flag]; ok && len(values) > 0 {
		return values[len(values)-1], true
	}

	value, ok := extraConfig[field].(string)

	return value, ok
}

// effectiveStringSlice resolves a list setting, with the flag taking precedence over the configuration field.
//
// The kubelet accepts the list flag both repeated and comma-separated.
func effectiveStringSlice(extraConfig map[string]any, args map[string][]string, field, flag string) []string {
	if values, ok := args[flag]; ok {
		var result []string

		for _, value := range values {
			for item := range strings.SplitSeq(value, ",") {
				result = append(result, strings.TrimSpace(item))
			}
		}

		return result
	}

	if raw, present := extraConfig[field]; present {
		return stringSlice(raw)
	}

	return []string{"pods"}
}

// effectiveReservesMemory reports whether a reservation setting carries a memory quantity, with the flag
// taking precedence over the configuration field.
//
// The kubelet accepts the reservation flag as comma-separated `resource=quantity` pairs, and the flag
// replaces the whole field. Talos fills an absent or empty field with its defaults when talosDefaulted
// is set, and those defaults include memory; a non-empty field without memory is left as is.
func effectiveReservesMemory(extraConfig map[string]any, args map[string][]string, field, flag string, talosDefaulted bool) bool {
	if values, ok := args[flag]; ok {
		for _, value := range values {
			for pair := range strings.SplitSeq(value, ",") {
				if name, _, _ := strings.Cut(pair, "="); strings.TrimSpace(name) == "memory" {
					return true
				}
			}
		}

		return false
	}

	if talosDefaulted && stringMapLen(extraConfig[field]) == 0 {
		return true
	}

	exists := stringMapContains(extraConfig[field], "memory")

	return exists
}

// stringMapLen returns the size of a configuration map which may be decoded as map[string]any or built as map[string]string.
func stringMapLen(raw any) int {
	switch m := raw.(type) {
	case map[string]any:
		return len(m)
	case map[string]string:
		return len(m)
	default:
		return 0
	}
}

func stringMapContains(raw any, key string) bool {
	switch m := raw.(type) {
	case map[string]any:
		_, exists := m[key]

		return exists
	case map[string]string:
		_, exists := m[key]

		return exists
	default:
		return false
	}
}

// stringSlice flattens a configuration list which may be decoded as []any or built as []string.
func stringSlice(raw any) []string {
	switch items := raw.(type) {
	case []any:
		var result []string

		for _, item := range items {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}

		return result
	case []string:
		return items
	default:
		return nil
	}
}

// isManagedRoot reports whether the kubelet cgroup name denotes a root whose memory limit Talos manages,
// either directly or through the kubepods limit.
func isManagedRoot(cgroupName string) bool {
	return path.Clean("/"+cgroupName) == "/"+constants.CgroupKubepods || isTalosOwnedRoot(cgroupName)
}

// isTalosOwnedRoot reports whether the kubelet cgroup name denotes a root whose memory.max Talos writes itself.
func isTalosOwnedRoot(cgroupName string) bool {
	return slices.Contains([]string{
		"/" + constants.CgroupTalosContainersRoot,
		"/" + constants.CgroupVirtualMachines,
	}, path.Clean("/"+cgroupName))
}
