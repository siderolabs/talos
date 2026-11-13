// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/siderolabs/go-pointer"
	"k8s.io/apimachinery/pkg/api/resource"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/machinery/kubelet"
)

// KubeletMemory is the memory policy rendered into the kubelet configuration.
type KubeletMemory struct {
	// Reserved is systemReserved.memory without a kubepods limit.
	Reserved uint64
	// KubepodsLimit, when set, derives systemReserved.memory from the limit and Capacity instead.
	KubepodsLimit uint64
	Capacity      uint64
	PageSize      int
}

// kubepodsMemoryReservation derives systemReserved.memory (S) from the kubepods memory limit (C):
//
//	S = M - C
//
// M is the host memory capacity, so the kubelet sets kubepods memory.max = M - S = C; kubeReserved.memory is rejected
// with the limit. The kubelet refuses to start when S + E > M, E being the hard memory.available eviction threshold,
// so C >= E is required as well. Hugepage capacity only reduces the advertised allocatable on the kubelet side and is
// not part of S. C was resolved against the same M, so a memory hotplug updates both.
func kubepodsMemoryReservation(config *kubeletconfig.KubeletConfiguration, memory KubeletMemory) (uint64, error) {
	capacity := memory.Capacity

	if capacity == 0 || capacity > math.MaxInt64 {
		return 0, fmt.Errorf("host memory capacity %d bytes is outside the kubelet's range", capacity)
	}

	effectiveLimit, err := kernel.NormalizeMemoryLimit(memory.KubepodsLimit, memory.PageSize)
	if err != nil {
		return 0, fmt.Errorf("invalid kubepods memory limit: %w", err)
	}

	if effectiveLimit > capacity {
		return 0, fmt.Errorf("kubepods memory limit %d bytes exceeds the host memory capacity %d bytes", effectiveLimit, capacity)
	}

	eviction, err := hardMemoryEvictionThreshold(config, capacity)
	if err != nil {
		return 0, err
	}

	if eviction.CmpInt64(int64(effectiveLimit)) > 0 {
		return 0, fmt.Errorf(`kubepods memory limit %d bytes is below the hard eviction threshold "evictionHard.memory.available" of %s (%d bytes): the kubelet would refuse to start`,
			effectiveLimit, eviction.String(), eviction.Value())
	}

	return capacity - effectiveLimit, nil
}

const evictionSignalMemoryAvailable = "memory.available"

// hardMemoryEvictionThreshold returns the hard memory.available eviction threshold the kubelet will apply.
//
// It mirrors cmd/kubelet/app/server.go loadConfigFile and pkg/kubelet/eviction/helpers.go parseThresholdStatement
// (v1.38.0-alpha.1): an empty evictionHard map is dropped on serialization, so the kubelet fills in all defaults;
// mergeDefaultEvictionSettings fills in only the missing signals; a missing signal means no threshold.
func hardMemoryEvictionThreshold(config *kubeletconfig.KubeletConfiguration, memoryCapacity uint64) (resource.Quantity, error) {
	raw, present := config.EvictionHard[evictionSignalMemoryAvailable]

	if len(config.EvictionHard) == 0 || (!present && pointer.SafeDeref(config.MergeDefaultEvictionSettings)) {
		raw, present = kubelet.DefaultHardMemoryEviction, true
	}

	if !present {
		return resource.Quantity{}, nil
	}

	if strings.HasSuffix(raw, "%") {
		return percentageEvictionThreshold(raw, memoryCapacity)
	}

	quantity, err := resource.ParseQuantity(raw)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf(`invalid kubelet configuration field "evictionHard.memory.available" %q: %w`, raw, err)
	}

	if quantity.Sign() < 0 || quantity.IsZero() {
		return resource.Quantity{}, fmt.Errorf(`invalid kubelet configuration field "evictionHard.memory.available" %q: eviction threshold must be positive`, raw)
	}

	return quantity, nil
}

// percentageEvictionThreshold converts a percentage threshold into bytes exactly as the kubelet does:
// "0%" and "100%" disable the signal, the rest is a float32 fraction of the capacity truncated to bytes
// (parsePercentage and GetThresholdQuantity, v1.38.0-alpha.1).
//
// NaN passes the kubelet's range checks but yields an architecture-dependent byte count, so it is rejected here.
func percentageEvictionThreshold(raw string, memoryCapacity uint64) (resource.Quantity, error) {
	if raw == "0%" || raw == "100%" {
		return resource.Quantity{}, nil
	}

	value, err := strconv.ParseFloat(strings.TrimRight(raw, "%"), 32)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf(`invalid kubelet configuration field "evictionHard.memory.available" %q: %w`, raw, err)
	}

	percentage := float32(value) / 100

	switch {
	case math.IsNaN(value):
		return resource.Quantity{}, fmt.Errorf(`invalid kubelet configuration field "evictionHard.memory.available" %q: eviction percentage threshold is not a number`, raw)
	case percentage < 0:
		return resource.Quantity{}, fmt.Errorf(`invalid kubelet configuration field "evictionHard.memory.available" %q: eviction percentage threshold must be >= 0%%`, raw)
	case percentage > 1:
		return resource.Quantity{}, fmt.Errorf(`invalid kubelet configuration field "evictionHard.memory.available" %q: eviction percentage threshold must be <= 100%%`, raw)
	}

	return *resource.NewQuantity(int64(float64(memoryCapacity)*float64(percentage)), resource.BinarySI), nil
}
