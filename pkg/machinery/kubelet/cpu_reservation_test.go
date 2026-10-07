// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubelet_test

import (
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/kubelet"
)

func TestCPUReservationEffectiveOptions(t *testing.T) {
	config := map[string]string{"file-only": "true", "full-pcpus-only": "true"}
	options, err := kubelet.CPUReservationOptions(config, map[string][]string{"cpu_manager_policy_options": {"full-pcpus-only=true", "full-pcpus-only=false"}})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"full-pcpus-only": "false", "strict-cpu-reservation": "true"}, options)
	require.Equal(t, "true", config["file-only"])
	_, err = kubelet.CPUReservationOptions(config, map[string][]string{"cpu-manager-policy-options": {"full-pcpus-only=true"}, "cpu_manager_policy_options": {"full-pcpus-only=false"}})
	require.Error(t, err)
}

func TestCPUReservationGateOverlay(t *testing.T) {
	gates := map[string]bool{"CPUManagerPolicyBetaOptions": false}
	args := map[string][]string{"feature_gates": {"RotateKubeletServerCertificate=true"}}
	require.Error(t, kubelet.ValidateCPUReservationGates(34, gates, args), "unrelated CLI gates must not erase file gates")
	args["feature_gates"] = append(args["feature_gates"], "CPUManagerPolicyBetaOptions=true")
	require.NoError(t, kubelet.ValidateCPUReservationGates(34, gates, args))
	require.Equal(t, map[string]bool{"CPUManagerPolicyBetaOptions": false}, gates)
	require.NoError(t, kubelet.ValidateCPUReservationGates(35, gates, map[string][]string{"feature-gates": {"Other=true"}}))
}

func TestCPUReservationRequiredGate(t *testing.T) {
	for _, minor := range []uint64{33, 34} {
		require.Error(t, kubelet.ValidateCPUReservationGates(minor, map[string]bool{"CPUManagerPolicyBetaOptions": false}, nil))
		require.NoError(t, kubelet.ValidateCPUReservationGates(minor, map[string]bool{"CPUManagerPolicyBetaOptions": false}, map[string][]string{"feature-gates": {"CPUManagerPolicyBetaOptions=true"}}))
		require.Error(t, kubelet.ValidateCPUReservationGates(minor, nil, map[string][]string{"feature_gates": {"CPUManagerPolicyBetaOptions=true", "CPUManagerPolicyBetaOptions=false"}}))
	}

	for _, minor := range []uint64{35, 38} {
		require.NoError(t, kubelet.ValidateCPUReservationGates(minor, map[string]bool{"CPUManagerPolicyBetaOptions": false}, nil))
	}
}

func TestCPUReservationOwnership(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		config   map[string]any
		args     map[string][]string
		mounts   []specs.Mount
		conflict bool
	}{
		{name: "defaults"},
		{name: "null defaults", config: map[string]any{"cpuManagerPolicy": nil, "cpuManagerPolicyOptions": nil}},
		{name: "empty policy defaults", config: map[string]any{"cpuManagerPolicy": ""}},
		{name: "explicit reserved", config: map[string]any{"reservedSystemCPUs": "0"}, conflict: true},
		{name: "none policy", config: map[string]any{"cpuManagerPolicy": "none"}, conflict: true},
		{name: "static compatible options", config: map[string]any{"cpuManagerPolicy": "static", "cpuManagerPolicyOptions": map[string]any{"strict-cpu-reservation": "true", "full-pcpus-only": "true"}}},
		{name: "strict disabled", config: map[string]any{"cpuManagerPolicyOptions": map[string]any{"strict-cpu-reservation": "false"}}, conflict: true},
		{name: "reserved CLI", args: map[string][]string{"reserved-cpus": {"0"}}, conflict: true},
		{name: "normalized reserved CLI", args: map[string][]string{"reserved_cpus": {"0"}}, conflict: true},
		{name: "normalized runtime endpoint", args: map[string][]string{"container_runtime_endpoint": {"unix:///other-runtime.sock"}}, conflict: true},
		{name: "CLI none", args: map[string][]string{"cpu-manager-policy": {"none"}}, conflict: true},
		{name: "CLI last wins", args: map[string][]string{"cpu-manager-policy": {"static", "none"}}, conflict: true},
		{name: "CLI static", args: map[string][]string{"cpu-manager-policy": {"static"}}},
		{name: "CLI options replace map", args: map[string][]string{"cpu-manager-policy-options": {"full-pcpus-only=true"}}},
		{name: "CLI options retain strict", args: map[string][]string{"cpu-manager-policy-options": {"full-pcpus-only=true", "strict-cpu-reservation=true"}}},
		{name: "CLI strict overridden", args: map[string][]string{"cpu-manager-policy-options": {"strict-cpu-reservation=true", "strict-cpu-reservation=false"}}, conflict: true},
		{name: "dropins", args: map[string][]string{"config-dir": {"/etc/custom"}}, conflict: true},
		{name: "alternate config", args: map[string][]string{"config": {"/etc/custom"}}, conflict: true},
		{name: "alternate pods", args: map[string][]string{"cgroup-root": {"/elsewhere"}}, conflict: true},
		{name: "obscured config", mounts: []specs.Mount{{Destination: "/etc"}}, conflict: true},
		{
			name: "unrelated", config: map[string]any{"maxPods": 20},
			args:   map[string][]string{"feature-gates": {"CPUManagerPolicyBetaOptions=false"}, "v": {"2"}},
			mounts: []specs.Mount{{Destination: "/etc/other"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := kubelet.ValidateCPUReservation(tc.config, tc.args, tc.mounts)
			if tc.conflict {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
