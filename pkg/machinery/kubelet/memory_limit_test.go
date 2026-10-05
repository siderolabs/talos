// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubelet_test

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/kubelet"
)

func TestValidateMemoryLimitConfiguration(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		extraConfig map[string]any
		extraArgs   map[string][]string

		expectedError string
	}{
		{
			name: "no settings",
		},
		{
			name: "unrelated settings",
			extraConfig: map[string]any{
				"serverTLSBootstrap":     true,
				"systemReserved":         map[string]any{"cpu": "500m", "pid": "1000"},
				"kubeReserved":           map[string]any{"memory": "512Mi", "cpu": "100m"},
				"evictionHard":           map[string]any{"memory.available": "5%"},
				"systemReservedCgroup":   "/system",
				"kubeReservedCgroup":     "/podruntime",
				"enforceNodeAllocatable": []any{"pods", "system-reserved-compressible", "kube-reserved-compressible"},
				"cgroupsPerQOS":          true,
				"cgroupDriver":           "cgroupfs",
				"memoryManagerPolicy":    "None",
			},
			extraArgs: map[string][]string{"feature-gates": {"AllBeta=true"}, "node-labels": {"a=b"}},
		},
		{
			name:        "kube-reserved enforcement on an unmanaged cgroup",
			extraConfig: map[string]any{"enforceNodeAllocatable": []any{"pods", "kube-reserved"}, "kubeReservedCgroup": "/podruntime"},
		},
		{
			name:        "managed cgroup name without enforcement",
			extraConfig: map[string]any{"kubeReservedCgroup": "/kubepods", "systemReservedCgroup": "/taloscontainers"},
		},
		{
			name:        "typed shapes without conflicts",
			extraConfig: map[string]any{"systemReserved": map[string]string{"cpu": "500m"}, "enforceNodeAllocatable": []string{"pods", "kube-reserved-compressible"}},
		},
		{
			name:        "systemReserved memory",
			extraConfig: map[string]any{"systemReserved": map[string]any{"memory": "1Gi", "cpu": "1"}},

			expectedError: `kubelet configuration field "systemReserved.memory" conflicts with the kubepods memory limit: Talos derives it from the limit`,
		},
		{
			name:        "typed systemReserved memory",
			extraConfig: map[string]any{"systemReserved": map[string]string{"memory": "1Gi"}},

			expectedError: `kubelet configuration field "systemReserved.memory" conflicts with the kubepods memory limit: Talos derives it from the limit`,
		},
		{
			name:        "typed enforcement without pods",
			extraConfig: map[string]any{"enforceNodeAllocatable": []string{"system-reserved-compressible"}},

			expectedError: `kubelet configuration field "enforceNodeAllocatable" must include "pods" with the kubepods memory limit`,
		},
		{
			name:        "enforcement without pods",
			extraConfig: map[string]any{"enforceNodeAllocatable": []any{"none"}},

			expectedError: `kubelet configuration field "enforceNodeAllocatable" must include "pods" with the kubepods memory limit`,
		},
		{
			name:        "empty enforcement",
			extraConfig: map[string]any{"enforceNodeAllocatable": []any{}},

			expectedError: `kubelet configuration field "enforceNodeAllocatable" must include "pods" with the kubepods memory limit`,
		},
		{
			name:        "system-reserved enforcement",
			extraConfig: map[string]any{"enforceNodeAllocatable": []any{"pods", "system-reserved"}, "systemReservedCgroup": "/system"},

			expectedError: `kubelet configuration field "enforceNodeAllocatable" must not include "system-reserved" with the kubepods memory limit: ` +
				`the derived "systemReserved.memory" is not a system daemon budget`,
		},
		{
			name:        "kube-reserved enforcement on a managed cgroup",
			extraConfig: map[string]any{"enforceNodeAllocatable": []any{"pods", "kube-reserved"}, "kubeReservedCgroup": "virtualmachines.partition/"},

			expectedError: `kubelet configuration field "kubeReservedCgroup" "virtualmachines.partition/" is a Talos-managed cgroup: ` +
				`it cannot be enforced with "kube-reserved" together with the kubepods memory limit`,
		},
		{
			name:        "cgroupsPerQOS disabled",
			extraConfig: map[string]any{"cgroupsPerQOS": false},

			expectedError: `kubelet configuration field "cgroupsPerQOS" must be enabled with the kubepods memory limit`,
		},
		{
			name:        "systemd cgroup driver",
			extraConfig: map[string]any{"cgroupDriver": "systemd"},

			expectedError: `kubelet configuration field "cgroupDriver" "systemd" conflicts with the kubepods memory limit: only "cgroupfs" is supported`,
		},
		{
			name:        "static memory manager",
			extraConfig: map[string]any{"memoryManagerPolicy": "Static"},

			expectedError: `kubelet configuration field "memoryManagerPolicy" "Static" is not supported with the kubepods memory limit`,
		},
		{
			name: "overriding arguments",
			extraArgs: map[string][]string{
				"system-reserved":          {"cpu=500m"},
				"kube-reserved":            {"memory=1Gi"},
				"system-reserved-cgroup":   {"/system"},
				"kube-reserved-cgroup":     {"/podruntime"},
				"enforce-node-allocatable": {"pods"},
				"cgroups-per-qos":          {"true"},
				"cgroup-root":              {"/"},
				"cgroup-driver":            {"cgroupfs"},
				"memory-manager-policy":    {"None"},
				"reserved-memory":          {"0:memory=1Gi"},
				"eviction-hard":            {"memory.available<100Mi"},
				"feature-gates":            {"AllBeta=true"},
			},

			expectedError: `kubelet argument "cgroup-driver" conflicts with the kubepods memory limit: use the "cgroupDriver" configuration field instead` + "\n" +
				`kubelet argument "cgroup-root" conflicts with the kubepods memory limit` + "\n" +
				`kubelet argument "cgroups-per-qos" conflicts with the kubepods memory limit: use the "cgroupsPerQOS" configuration field instead` + "\n" +
				`kubelet argument "enforce-node-allocatable" conflicts with the kubepods memory limit: use the "enforceNodeAllocatable" configuration field instead` + "\n" +
				`kubelet argument "eviction-hard" conflicts with the kubepods memory limit: use the "evictionHard" configuration field instead` + "\n" +
				`kubelet argument "kube-reserved" conflicts with the kubepods memory limit: use the "kubeReserved" configuration field instead` + "\n" +
				`kubelet argument "kube-reserved-cgroup" conflicts with the kubepods memory limit: use the "kubeReservedCgroup" configuration field instead` + "\n" +
				`kubelet argument "memory-manager-policy" conflicts with the kubepods memory limit: use the "memoryManagerPolicy" configuration field instead` + "\n" +
				`kubelet argument "reserved-memory" conflicts with the kubepods memory limit: use the "reservedMemory" configuration field instead` + "\n" +
				`kubelet argument "system-reserved" conflicts with the kubepods memory limit: use the "systemReserved" configuration field instead` + "\n" +
				`kubelet argument "system-reserved-cgroup" conflicts with the kubepods memory limit: use the "systemReservedCgroup" configuration field instead`,
		},
		{
			name:        "mixed argument and configuration",
			extraConfig: map[string]any{"systemReserved": map[string]any{"memory": "1Gi"}},
			extraArgs:   map[string][]string{"system-reserved": {"cpu=500m"}},

			expectedError: `kubelet argument "system-reserved" conflicts with the kubepods memory limit: use the "systemReserved" configuration field instead` + "\n" +
				`kubelet configuration field "systemReserved.memory" conflicts with the kubepods memory limit: Talos derives it from the limit`,
		},
		{
			name: "underscore argument aliases",
			extraArgs: map[string][]string{
				"system_reserved":          {"cpu=500m"},
				"cgroup_root":              {"/"},
				"enforce_node-allocatable": {"pods"},
				"node_labels":              {"a=b"},
			},

			expectedError: `kubelet argument "cgroup_root" conflicts with the kubepods memory limit` + "\n" +
				`kubelet argument "enforce_node-allocatable" conflicts with the kubepods memory limit: use the "enforceNodeAllocatable" configuration field instead` + "\n" +
				`kubelet argument "system_reserved" conflicts with the kubepods memory limit: use the "systemReserved" configuration field instead`,
		},
		{
			name:      "configuration directory",
			extraArgs: map[string][]string{"config-dir": {"/etc/kubernetes/kubelet.conf.d"}},

			expectedError: `kubelet argument "config-dir" conflicts with the kubepods memory limit: drop-in files are merged over the derived configuration`,
		},
		{
			name:      "configuration directory alias",
			extraArgs: map[string][]string{"config_dir": {"/etc/kubernetes/kubelet.conf.d"}},

			expectedError: `kubelet argument "config_dir" conflicts with the kubepods memory limit: drop-in files are merged over the derived configuration`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			snapshot := maps.Clone(test.extraArgs)

			err := kubelet.ValidateMemoryLimitConfiguration(test.extraConfig, test.extraArgs)

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedError)
			}

			assert.Equal(t, snapshot, test.extraArgs)
		})
	}
}

func TestValidateWorkloadRootConfiguration(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		extraConfig map[string]any
		extraArgs   map[string][]string

		expectedError string
	}{
		{
			name: "no settings",
		},
		{
			name: "unrelated settings",
			extraConfig: map[string]any{
				"systemReserved":         map[string]any{"memory": "1Gi", "cpu": "500m"},
				"kubeReserved":           map[string]any{"memory": "512Mi"},
				"systemReservedCgroup":   "/system",
				"kubeReservedCgroup":     "/podruntime",
				"enforceNodeAllocatable": []any{"pods", "system-reserved", "kube-reserved"},
				"cgroupsPerQOS":          false,
			},
			extraArgs: map[string][]string{"eviction-hard": {"memory.available<100Mi"}, "node-labels": {"a=b"}},
		},
		{
			name: "compressible enforcement on Talos roots",
			extraConfig: map[string]any{
				"systemReserved":         map[string]any{"memory": "1Gi", "cpu": "500m"},
				"kubeReserved":           map[string]string{"memory": "512Mi"},
				"systemReservedCgroup":   "/taloscontainers",
				"kubeReservedCgroup":     "virtualmachines.partition",
				"enforceNodeAllocatable": []string{"pods", "system-reserved-compressible", "kube-reserved-compressible"},
			},
		},
		{
			name: "enforcement without a memory reservation on a Talos root",
			extraConfig: map[string]any{
				"systemReserved":         map[string]any{"cpu": "500m", "pid": "1000"},
				"systemReservedCgroup":   "/taloscontainers",
				"enforceNodeAllocatable": []any{"pods", "system-reserved"},
			},
		},
		{
			name: "absent system reservation is defaulted by Talos with memory",
			extraConfig: map[string]any{
				"systemReservedCgroup":   "/taloscontainers",
				"enforceNodeAllocatable": []any{"pods", "system-reserved"},
			},

			expectedError: `kubelet enforces the "systemReserved.memory" reservation on the Talos-managed cgroup "/taloscontainers" through "enforceNodeAllocatable" "system-reserved": ` +
				`use "system-reserved-compressible" or a cgroup outside the Talos-managed workload roots`,
		},
		{
			name: "empty system reservation is defaulted by Talos with memory",
			extraConfig: map[string]any{
				"systemReserved":         map[string]any{},
				"systemReservedCgroup":   "/taloscontainers",
				"enforceNodeAllocatable": []any{"pods", "system-reserved"},
			},

			expectedError: `kubelet enforces the "systemReserved.memory" reservation on the Talos-managed cgroup "/taloscontainers" through "enforceNodeAllocatable" "system-reserved": ` +
				`use "system-reserved-compressible" or a cgroup outside the Talos-managed workload roots`,
		},
		{
			name: "absent kube reservation is not defaulted",
			extraConfig: map[string]any{
				"kubeReservedCgroup":     "/taloscontainers",
				"enforceNodeAllocatable": []any{"pods", "kube-reserved"},
			},
		},
		{
			name: "mixed spelling of the enforcement argument",
			extraConfig: map[string]any{
				"systemReserved":       map[string]any{"memory": "1Gi"},
				"systemReservedCgroup": "/taloscontainers",
			},
			extraArgs: map[string][]string{
				"enforce-node-allocatable": {"system-reserved"},
				"enforce_node_allocatable": {"pods"},
			},

			expectedError: `kubelet enforces the "systemReserved.memory" reservation on the Talos-managed cgroup "/taloscontainers" through "enforceNodeAllocatable" "system-reserved": ` +
				`use "system-reserved-compressible" or a cgroup outside the Talos-managed workload roots`,
		},
		{
			name: "mixed spelling of the reservation argument",
			extraConfig: map[string]any{
				"systemReservedCgroup":   "/taloscontainers",
				"enforceNodeAllocatable": []any{"pods", "system-reserved"},
			},
			extraArgs: map[string][]string{
				"system-reserved": {"memory=1Gi"},
				"system_reserved": {"cpu=500m"},
			},

			expectedError: `kubelet enforces the "systemReserved.memory" reservation on the Talos-managed cgroup "/taloscontainers" through "enforceNodeAllocatable" "system-reserved": ` +
				`use "system-reserved-compressible" or a cgroup outside the Talos-managed workload roots`,
		},
		{
			name: "memory enforcement on the kubelet-owned kubepods root",
			extraConfig: map[string]any{
				"kubeReserved":           map[string]any{"memory": "512Mi"},
				"kubeReservedCgroup":     "/kubepods",
				"enforceNodeAllocatable": []any{"pods", "kube-reserved"},
			},
		},
		{
			name: "system reserved memory enforced on the containers root",
			extraConfig: map[string]any{
				"systemReserved":         map[string]any{"memory": "1Gi"},
				"systemReservedCgroup":   "/taloscontainers/",
				"enforceNodeAllocatable": []any{"pods", "system-reserved"},
			},

			expectedError: `kubelet enforces the "systemReserved.memory" reservation on the Talos-managed cgroup "/taloscontainers/" through "enforceNodeAllocatable" "system-reserved": ` +
				`use "system-reserved-compressible" or a cgroup outside the Talos-managed workload roots`,
		},
		{
			name: "kube reserved memory enforced on the virtual machines root with typed shapes",
			extraConfig: map[string]any{
				"kubeReserved":           map[string]string{"memory": "2Gi"},
				"kubeReservedCgroup":     "virtualmachines.partition",
				"enforceNodeAllocatable": []string{"pods", "kube-reserved"},
			},

			expectedError: `kubelet enforces the "kubeReserved.memory" reservation on the Talos-managed cgroup "virtualmachines.partition" through "enforceNodeAllocatable" "kube-reserved": ` +
				`use "kube-reserved-compressible" or a cgroup outside the Talos-managed workload roots`,
		},
		{
			name: "both reservations enforced on Talos roots",
			extraConfig: map[string]any{
				"systemReserved":         map[string]any{"memory": "1Gi"},
				"kubeReserved":           map[string]any{"memory": "2Gi"},
				"systemReservedCgroup":   "/taloscontainers",
				"kubeReservedCgroup":     "/virtualmachines.partition",
				"enforceNodeAllocatable": []any{"pods", "system-reserved", "kube-reserved"},
			},

			expectedError: `kubelet enforces the "systemReserved.memory" reservation on the Talos-managed cgroup "/taloscontainers" through "enforceNodeAllocatable" "system-reserved": ` +
				`use "system-reserved-compressible" or a cgroup outside the Talos-managed workload roots` + "\n" +
				`kubelet enforces the "kubeReserved.memory" reservation on the Talos-managed cgroup "/virtualmachines.partition" through "enforceNodeAllocatable" "kube-reserved": ` +
				`use "kube-reserved-compressible" or a cgroup outside the Talos-managed workload roots`,
		},
		{
			name:        "arguments override a safe configuration",
			extraConfig: map[string]any{"enforceNodeAllocatable": []any{"pods"}, "systemReservedCgroup": "/system"},
			extraArgs: map[string][]string{
				"enforce-node-allocatable": {"pods,system-reserved"},
				"system-reserved-cgroup":   {"/taloscontainers"},
				"system-reserved":          {"cpu=500m,memory=1Gi"},
			},

			expectedError: `kubelet enforces the "systemReserved.memory" reservation on the Talos-managed cgroup "/taloscontainers" through "enforceNodeAllocatable" "system-reserved": ` +
				`use "system-reserved-compressible" or a cgroup outside the Talos-managed workload roots`,
		},
		{
			name: "underscore argument aliases",
			extraArgs: map[string][]string{
				"enforce_node_allocatable": {"pods", "kube-reserved"},
				"kube_reserved_cgroup":     {"virtualmachines.partition"},
				"kube_reserved":            {"memory=2Gi"},
			},

			expectedError: `kubelet enforces the "kubeReserved.memory" reservation on the Talos-managed cgroup "virtualmachines.partition" through "enforceNodeAllocatable" "kube-reserved": ` +
				`use "kube-reserved-compressible" or a cgroup outside the Talos-managed workload roots`,
		},
		{
			name: "arguments move the enforcement off the Talos root",
			extraConfig: map[string]any{
				"systemReserved":         map[string]any{"memory": "1Gi"},
				"systemReservedCgroup":   "/taloscontainers",
				"enforceNodeAllocatable": []any{"pods", "system-reserved"},
			},
			extraArgs: map[string][]string{"system-reserved-cgroup": {"/system"}},
		},
		{
			name: "arguments drop the memory reservation",
			extraConfig: map[string]any{
				"systemReserved":         map[string]any{"memory": "1Gi"},
				"systemReservedCgroup":   "/taloscontainers",
				"enforceNodeAllocatable": []any{"pods", "system-reserved"},
			},
			extraArgs: map[string][]string{"system-reserved": {"cpu=500m"}},
		},
		{
			name: "arguments switch to compressible enforcement",
			extraConfig: map[string]any{
				"systemReserved":         map[string]any{"memory": "1Gi"},
				"systemReservedCgroup":   "/taloscontainers",
				"enforceNodeAllocatable": []any{"pods", "system-reserved"},
			},
			extraArgs: map[string][]string{"enforce-node-allocatable": {"pods,system-reserved-compressible"}},
		},
		{
			name:      "configuration directory",
			extraArgs: map[string][]string{"config-dir": {"/etc/kubernetes/kubelet.conf.d"}},

			expectedError: `kubelet argument "config-dir" conflicts with WorkloadResourceConfig: drop-in files can enforce reservations on the Talos-managed cgroups unseen`,
		},
		{
			name:      "configuration directory alias",
			extraArgs: map[string][]string{"config_dir": {"/etc/kubernetes/kubelet.conf.d"}},

			expectedError: `kubelet argument "config_dir" conflicts with WorkloadResourceConfig: drop-in files can enforce reservations on the Talos-managed cgroups unseen`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			snapshot := maps.Clone(test.extraArgs)

			err := kubelet.ValidateWorkloadRootConfiguration(test.extraConfig, test.extraArgs)

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedError)
			}

			assert.Equal(t, snapshot, test.extraArgs)
		})
	}
}
