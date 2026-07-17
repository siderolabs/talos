// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/kubenodeconfig.yaml
var expectedKubeNodeConfigDocument []byte

func kubeNodeConfig() *k8s.KubeNodeConfigV1Alpha1 {
	cfg := k8s.NewKubeNodeConfigV1Alpha1()
	cfg.RegisterWithFQDNConfig = new(true)
	cfg.NodeIPConfig = k8s.NodeIPConfig{
		NodeIPValidSubnets: []string{
			"10.0.0.0/8",
			"!10.0.0.3/32",
			"fdc7::/16",
		},
	}
	cfg.LabelsConfig = map[string]string{
		"examplelabel": "examplevalue",
	}
	cfg.AnnotationsConfig = map[string]string{
		"customer.io/rack": "r13a25",
	}
	cfg.TaintsConfig = map[string]string{
		"exampletaint": "examplevalue:NoSchedule",
	}

	return cfg
}

func TestKubeNodeConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := kubeNodeConfig()

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeNodeConfigDocument, marshaled)
}

func TestKubeNodeConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeNodeConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeNodeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeNodeConfig,
		},
		RegisterWithFQDNConfig: new(true),
		NodeIPConfig: k8s.NodeIPConfig{
			NodeIPValidSubnets: []string{
				"10.0.0.0/8",
				"!10.0.0.3/32",
				"fdc7::/16",
			},
		},
		LabelsConfig: map[string]string{
			"examplelabel": "examplevalue",
		},
		AnnotationsConfig: map[string]string{
			"customer.io/rack": "r13a25",
		},
		TaintsConfig: map[string]string{
			"exampletaint": "examplevalue:NoSchedule",
		},
	}, docs[0])
}

func TestKubeNodeConfigAccessors(t *testing.T) {
	t.Parallel()

	cfg := kubeNodeConfig()

	assert.False(t, cfg.SkipNodeRegistration())
	assert.True(t, cfg.RegisterWithFQDN())
	assert.Equal(t, []string{"10.0.0.0/8", "!10.0.0.3/32", "fdc7::/16"}, cfg.NodeIP().ValidSubnets())
	assert.Equal(t, map[string]string{"examplelabel": "examplevalue"}, cfg.Labels())
	assert.Equal(t, map[string]string{"customer.io/rack": "r13a25"}, cfg.Annotations())
	assert.Equal(t, map[string]string{"exampletaint": "examplevalue:NoSchedule"}, cfg.Taints())

	skipCfg := k8s.NewKubeNodeConfigV1Alpha1()
	skipCfg.SkipNodeRegistrationConfig = new(true)

	assert.True(t, skipCfg.SkipNodeRegistration())
}

func TestKubeNodeConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeNodeConfigV1Alpha1

		expectedError string
	}{
		{
			name: "valid",
			cfg:  kubeNodeConfig,
		},
		{
			name: "empty",
			cfg:  k8s.NewKubeNodeConfigV1Alpha1,
		},
		{
			name: "invalid nodeIP subnet",
			cfg: func() *k8s.KubeNodeConfigV1Alpha1 {
				cfg := k8s.NewKubeNodeConfigV1Alpha1()
				cfg.NodeIPConfig = k8s.NodeIPConfig{
					NodeIPValidSubnets: []string{"not-a-subnet"},
				}

				return cfg
			},
			expectedError: `nodeIP subnet is not valid: "not-a-subnet"`,
		},
		{
			name: "invalid label",
			cfg: func() *k8s.KubeNodeConfigV1Alpha1 {
				cfg := k8s.NewKubeNodeConfigV1Alpha1()
				cfg.LabelsConfig = map[string]string{
					"": "value",
				}

				return cfg
			},
			expectedError: "invalid node labels: 1 error occurred:\n\t* name cannot be empty: \"\"\n\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})
			assert.Nil(t, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKubeNodeConfigV1Alpha1ConflictValidate(t *testing.T) {
	t.Parallel()

	cfg := kubeNodeConfig()

	for _, test := range []struct {
		name        string
		v1alpha1Cfg *v1alpha1.Config

		expectedError string
	}{
		{
			name:        "empty",
			v1alpha1Cfg: &v1alpha1.Config{},
		},
		{
			name: "empty machine and cluster config",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		},
		{
			name: "legacy allowSchedulingOnControlPlanes present",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					AllowSchedulingOnControlPlanes: new(true), //nolint:staticcheck // testing legacy config conflict
				},
			},
			expectedError: ".cluster.allowSchedulingOnControlPlanes is already set in v1alpha1 config",
		},
		{
			name: "legacy skipNodeRegistration present",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineKubelet: &v1alpha1.KubeletConfig{ //nolint:staticcheck // legacy config
						KubeletSkipNodeRegistration: new(true),
					},
				},
			},
			expectedError: ".machine.kubelet.skipNodeRegistration is already set in v1alpha1 config",
		},
		{
			name: "legacy registerWithFQDN present",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineKubelet: &v1alpha1.KubeletConfig{ //nolint:staticcheck // legacy config
						KubeletRegisterWithFQDN: new(true),
					},
				},
			},
			expectedError: ".machine.kubelet.registerWithFQDN is already set in v1alpha1 config",
		},
		{
			name: "legacy nodeIP present",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineKubelet: &v1alpha1.KubeletConfig{ //nolint:staticcheck // testing legacy config conflict
						KubeletNodeIP: &v1alpha1.KubeletNodeIPConfig{},
					},
				},
			},
			expectedError: ".machine.kubelet.nodeIP is already set in v1alpha1 config",
		},
		{
			name: "legacy node labels present",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNodeLabels: map[string]string{"foo": "bar"}, //nolint:staticcheck // testing legacy config conflict
				},
			},
			expectedError: ".machine.nodeLabels is already set in v1alpha1 config",
		},
		{
			name: "legacy node annotations present",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNodeAnnotations: map[string]string{"foo": "bar"}, //nolint:staticcheck // testing legacy config conflict
				},
			},
			expectedError: ".machine.nodeAnnotations is already set in v1alpha1 config",
		},
		{
			name: "legacy node taints present",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNodeTaints: map[string]string{"foo": "bar:NoSchedule"}, //nolint:staticcheck // testing legacy config conflict
				},
			},
			expectedError: ".machine.nodeTaints is already set in v1alpha1 config",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := cfg.V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
