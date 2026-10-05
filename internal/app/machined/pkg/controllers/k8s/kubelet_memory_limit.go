// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/prometheus/procfs"
	"github.com/siderolabs/go-pointer"
	"k8s.io/apimachinery/pkg/api/resource"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"

	"github.com/siderolabs/talos/pkg/machinery/kernel"
)

// MemoryCapacityReader returns the usable memory capacity of the host in bytes.
type MemoryCapacityReader func() (uint64, error)

// ProcMemoryCapacity reads MemTotal from /proc/meminfo.
//
// This is the same source and conversion (kB * 1024) the kubelet uses for node memory capacity via cAdvisor.
func ProcMemoryCapacity() (uint64, error) {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return 0, fmt.Errorf("error opening procfs: %w", err)
	}

	info, err := fs.Meminfo()
	if err != nil {
		return 0, fmt.Errorf("error reading meminfo: %w", err)
	}

	if info.MemTotalBytes == nil {
		return 0, errors.New("MemTotal is missing from meminfo")
	}

	return *info.MemTotalBytes, nil
}

// KubeletConfigurationOption adjusts the host-dependent parts of NewKubeletConfiguration.
type KubeletConfigurationOption func(*kubeletConfigurationOptions)

type hostMemory struct {
	readCapacity MemoryCapacityReader
	pageSize     int
}

type kubeletConfigurationOptions struct {
	hostMemory                *hostMemory
	ignoreKubepodsMemoryLimit bool
}

// WithHostMemory supplies the memory capacity reader and page size the kubepods memory limit projection needs.
//
// The capacity is read only after the static conflict checks pass.
func WithHostMemory(readCapacity MemoryCapacityReader, pageSize int) KubeletConfigurationOption {
	return func(o *kubeletConfigurationOptions) {
		o.hostMemory = &hostMemory{readCapacity: readCapacity, pageSize: pageSize}
	}
}

// IgnoringKubepodsMemoryLimit renders the configuration as if no kubepods memory limit were configured.
//
// The kubelet does not own the cgroup hierarchy in container mode, so the limit is inactive there.
func IgnoringKubepodsMemoryLimit() KubeletConfigurationOption {
	return func(o *kubeletConfigurationOptions) {
		o.ignoreKubepodsMemoryLimit = true
	}
}

// kubepodsMemoryReservation derives systemReserved.memory (S) from the kubepods memory limit (C):
//
//	S = M - K - C
//
// M is the host memory capacity and K is kubeReserved.memory, so the kubelet sets kubepods memory.max = M - S - K = C.
// The kubelet refuses to start when S + K + E > M, E being the hard memory.available eviction threshold, so C >= E is
// required as well. Hugepage capacity only reduces the advertised allocatable on the kubelet side and is not part of S.
func kubepodsMemoryReservation(config *kubeletconfig.KubeletConfiguration, limit uint64, host hostMemory) (uint64, error) {
	effectiveLimit, err := kernel.NormalizeMemoryLimit(limit, host.pageSize)
	if err != nil {
		return 0, fmt.Errorf("invalid kubepods memory limit: %w", err)
	}

	kubeReserved, err := reservedMemory(config.KubeReserved)
	if err != nil {
		return 0, fmt.Errorf(`invalid kubelet configuration field "kubeReserved.memory": %w`, err)
	}

	capacity, err := host.readCapacity()
	if err != nil {
		return 0, fmt.Errorf("error reading memory capacity for the kubepods memory limit: %w", err)
	}

	if capacity == 0 || capacity > math.MaxInt64 {
		return 0, fmt.Errorf("host memory capacity %d bytes is outside the kubelet's range", capacity)
	}

	if kubeReserved > capacity || effectiveLimit > capacity-kubeReserved {
		return 0, fmt.Errorf(`kubepods memory limit %d bytes exceeds the host memory capacity %d bytes minus "kubeReserved.memory" %d bytes`,
			effectiveLimit, capacity, kubeReserved)
	}

	eviction, err := hardMemoryEvictionThreshold(config, capacity)
	if err != nil {
		return 0, err
	}

	if eviction.CmpInt64(int64(effectiveLimit)) > 0 {
		return 0, fmt.Errorf(`kubepods memory limit %d bytes is below the hard eviction threshold "evictionHard.memory.available" of %s (%d bytes): the kubelet would refuse to start`,
			effectiveLimit, eviction.String(), eviction.Value())
	}

	return capacity - kubeReserved - effectiveLimit, nil
}

// reservedMemory parses a reserved memory entry the way the kubelet does (resource.ParseQuantity, negatives rejected).
//
// A whole number of bytes is additionally required so that M - S - K stays exact.
func reservedMemory(reserved map[string]string) (uint64, error) {
	raw, ok := reserved["memory"]
	if !ok {
		return 0, nil
	}

	quantity, err := resource.ParseQuantity(raw)
	if err != nil {
		return 0, fmt.Errorf("quantity %q: %w", raw, err)
	}

	if quantity.Sign() < 0 {
		return 0, fmt.Errorf("quantity %q cannot be negative", raw)
	}

	bytes, ok := quantity.AsScaledInt64(0)
	if !ok {
		return 0, fmt.Errorf("quantity %q exceeds the kubelet's range", raw)
	}

	if quantity.CmpInt64(bytes) != 0 {
		return 0, fmt.Errorf("quantity %q is not a whole number of bytes", raw)
	}

	return uint64(bytes), nil
}

const (
	evictionSignalMemoryAvailable = "memory.available"

	// defaultHardMemoryEviction is DefaultEvictionHard["memory.available"] from kubelet pkg/kubelet/eviction/defaults_linux.go (v1.38.0-alpha.1).
	defaultHardMemoryEviction = "100Mi"
)

// hardMemoryEvictionThreshold returns the hard memory.available eviction threshold the kubelet will apply.
//
// It mirrors cmd/kubelet/app/server.go loadConfigFile and pkg/kubelet/eviction/helpers.go parseThresholdStatement
// (v1.38.0-alpha.1): an empty evictionHard map is dropped on serialization, so the kubelet fills in all defaults;
// mergeDefaultEvictionSettings fills in only the missing signals; a missing signal means no threshold.
func hardMemoryEvictionThreshold(config *kubeletconfig.KubeletConfiguration, memoryCapacity uint64) (resource.Quantity, error) {
	raw, present := config.EvictionHard[evictionSignalMemoryAvailable]

	if len(config.EvictionHard) == 0 || (!present && pointer.SafeDeref(config.MergeDefaultEvictionSettings)) {
		raw, present = defaultHardMemoryEviction, true
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
