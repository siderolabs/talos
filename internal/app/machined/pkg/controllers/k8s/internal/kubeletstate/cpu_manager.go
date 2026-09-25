// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeletstate

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"

	"k8s.io/apimachinery/pkg/api/resource"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/cpuset"
)

// CPU manager policy names (see k8s.io/kubernetes/pkg/kubelet/cm/cpumanager).
const (
	CPUManagerPolicyNone   = "none"
	CPUManagerPolicyStatic = "static"

	strictCPUReservationOption = "strict-cpu-reservation"
)

// CPUManagerConfig is the subset of the kubelet configuration which affects the CPU manager state.
type CPUManagerConfig struct {
	// Policy is the CPU manager policy (normalized, "none" if not set).
	Policy string
	// StrictCPUReservation is the value of the "strict-cpu-reservation" policy option.
	StrictCPUReservation bool
	// ReservedCPUs is the explicit set of reserved CPUs (reservedSystemCPUs), might be empty.
	ReservedCPUs cpuset.CPUSet
	// NumReservedCPUs is the number of reserved CPUs.
	//
	// If ReservedCPUs is empty, kubelet reserves this many CPUs picked by the topology.
	NumReservedCPUs int
}

// CPUManagerConfigFromKubelet extracts the CPU manager configuration from the kubelet configuration.
func CPUManagerConfigFromKubelet(cfg *kubeletconfig.KubeletConfiguration) (CPUManagerConfig, error) {
	reservedCPUs, err := cpuset.Parse(cfg.ReservedSystemCPUs)
	if err != nil {
		return CPUManagerConfig{}, fmt.Errorf("failed to parse reservedSystemCPUs %q: %w", cfg.ReservedSystemCPUs, err)
	}

	result := CPUManagerConfig{
		Policy:       cmp.Or(cfg.CPUManagerPolicy, CPUManagerPolicyNone),
		ReservedCPUs: reservedCPUs,
	}

	if value, ok := cfg.CPUManagerPolicyOptions[strictCPUReservationOption]; ok {
		result.StrictCPUReservation, err = strconv.ParseBool(value)
		if err != nil {
			return CPUManagerConfig{}, fmt.Errorf("failed to parse %s policy option %q: %w", strictCPUReservationOption, value, err)
		}
	}

	if !reservedCPUs.IsEmpty() {
		// kubelet overrides the CPU reservation with the explicit set
		result.NumReservedCPUs = reservedCPUs.Size()

		return result, nil
	}

	// kubelet takes the ceiling of the sum of kube and system reserved CPU quantities
	var reservedQuantity resource.Quantity

	for _, reserved := range []map[string]string{cfg.KubeReserved, cfg.SystemReserved} {
		value, ok := reserved["cpu"]
		if !ok {
			continue
		}

		quantity, err := resource.ParseQuantity(value)
		if err != nil {
			return CPUManagerConfig{}, fmt.Errorf("failed to parse reserved CPU quantity %q: %w", value, err)
		}

		reservedQuantity.Add(quantity)
	}

	result.NumReservedCPUs = int(math.Ceil(float64(reservedQuantity.MilliValue()) / 1000))

	return result, nil
}

// cpuManagerCheckpoint is the CPU manager state file format (v2).
type cpuManagerCheckpoint struct {
	PolicyName    string                       `json:"policyName"`
	DefaultCPUSet string                       `json:"defaultCpuSet"`
	Entries       map[string]map[string]string `json:"entries,omitempty"`
}

func validateCPUManagerState(raw []byte, cfg *kubeletconfig.KubeletConfiguration, machine Machine) error {
	config, err := CPUManagerConfigFromKubelet(cfg)
	if err != nil {
		return err
	}

	return ValidateCPUManagerState(raw, config, machine)
}

// ValidateCPUManagerState checks whether the kubelet would load the CPU manager state with the given configuration.
//
// It mirrors the checks done by the kubelet on startup (see k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/policy_static.go).
//
//nolint:gocyclo,cyclop
func ValidateCPUManagerState(raw []byte, cfg CPUManagerConfig, machine Machine) error {
	var checkpoint cpuManagerCheckpoint

	if err := json.Unmarshal(raw, &checkpoint); err != nil {
		return fmt.Errorf("failed to parse the state: %w", err)
	}

	if checkpoint.PolicyName != cfg.Policy {
		return fmt.Errorf("policy changed from %q to %q", checkpoint.PolicyName, cfg.Policy)
	}

	if cfg.Policy != CPUManagerPolicyStatic {
		return nil // only the static policy validates the state
	}

	defaultCPUs, err := cpuset.Parse(checkpoint.DefaultCPUSet)
	if err != nil {
		return fmt.Errorf("failed to parse default cpuset %q: %w", checkpoint.DefaultCPUSet, err)
	}

	assignedCPUs := cpuset.New()

	for pod, containers := range checkpoint.Entries {
		for container, value := range containers {
			containerCPUs, err := cpuset.Parse(value)
			if err != nil {
				return fmt.Errorf("failed to parse cpuset %q for container %q in pod %q: %w", value, container, pod, err)
			}

			if !containerCPUs.Intersection(defaultCPUs).IsEmpty() {
				return fmt.Errorf("cpuset %q of container %q in pod %q overlaps with the default cpuset %q", value, container, pod, checkpoint.DefaultCPUSet)
			}

			assignedCPUs = assignedCPUs.Union(containerCPUs)
		}
	}

	if defaultCPUs.IsEmpty() {
		if len(checkpoint.Entries) > 0 {
			return errors.New("default cpuset is empty, but there are container assignments")
		}

		return nil // empty state, kubelet initializes it
	}

	knownCPUs := defaultCPUs.Union(assignedCPUs)
	availableCPUs := machine.OnlineCPUs

	switch {
	case !cfg.ReservedCPUs.IsEmpty():
		// explicit set of reserved CPUs
		if cfg.StrictCPUReservation {
			availableCPUs = availableCPUs.Difference(cfg.ReservedCPUs)

			if !cfg.ReservedCPUs.Intersection(defaultCPUs).IsEmpty() {
				return fmt.Errorf("strictly reserved CPUs %q are present in the default cpuset %q", cfg.ReservedCPUs, defaultCPUs)
			}
		} else if !cfg.ReservedCPUs.IsSubsetOf(defaultCPUs) {
			return fmt.Errorf("not all reserved CPUs %q are present in the default cpuset %q", cfg.ReservedCPUs, defaultCPUs)
		}
	case cfg.StrictCPUReservation:
		// the reserved CPUs are picked by the kubelet based on the topology, but they are excluded
		// from the state with strict reservation, so the previously reserved set can be recovered from the state
		previouslyReservedCPUs := availableCPUs.Difference(knownCPUs)

		if previouslyReservedCPUs.Size() != cfg.NumReservedCPUs {
			return fmt.Errorf("number of strictly reserved CPUs changed from %d to %d", previouslyReservedCPUs.Size(), cfg.NumReservedCPUs)
		}

		availableCPUs = availableCPUs.Difference(previouslyReservedCPUs)
	default:
		// the reserved CPUs are picked by the kubelet based on the topology, and they are part of the default cpuset,
		// so there is no way to tell which CPUs were reserved when the state was created
	}

	if !knownCPUs.Equals(availableCPUs) {
		return fmt.Errorf("set of available CPUs %q doesn't match the CPUs in the state %q", availableCPUs, knownCPUs)
	}

	return nil
}
