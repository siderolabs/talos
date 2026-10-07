// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration && integration_api

package api_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/integration/api"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
)

func TestKubeletExtraConfigPatch(t *testing.T) {
	for name, fixture := range map[string]string{
		"legacy": `version: v1alpha1
machine:
  type: worker
  kubelet:
    image: example.com/kubelet:custom
    extraConfig:
      maxPods: 42
`,
		"modern": `apiVersion: v1alpha1
kind: KubeletConfig
image: example.com/kubelet:custom
clusterDNS:
  - 10.96.0.10
config:
  maxPods: 42
`,
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := configloader.NewFromBytes([]byte(fixture))
			require.NoError(t, err)

			patchBytes, err := yaml.Marshal(api.KubeletExtraConfigPatch(cfg.K8sKubeletConfig(), map[string]any{
				"evictionHard":                 map[string]any{"memory.available": "0%"},
				"mergeDefaultEvictionSettings": true,
			}))
			require.NoError(t, err)

			patch, err := configpatcher.LoadPatch(patchBytes)
			require.NoError(t, err)

			out, err := configpatcher.Apply(configpatcher.WithConfig(cfg), []configpatcher.Patch{patch})
			require.NoError(t, err)

			patched, err := out.Config()
			require.NoError(t, err)
			require.IsType(t, cfg.K8sKubeletConfig(), patched.K8sKubeletConfig())
			require.Equal(t, "example.com/kubelet:custom", patched.K8sKubeletConfig().Image())
			require.Equal(t, cfg.K8sKubeletConfig().ClusterDNS(), patched.K8sKubeletConfig().ClusterDNS())
			require.EqualValues(t, 42, patched.K8sKubeletConfig().ExtraConfig()["maxPods"])
			require.Equal(t, map[string]any{"memory.available": "0%"}, patched.K8sKubeletConfig().ExtraConfig()["evictionHard"])
			require.Equal(t, true, patched.K8sKubeletConfig().ExtraConfig()["mergeDefaultEvictionSettings"])
			require.NotContains(t, cfg.K8sKubeletConfig().ExtraConfig(), "evictionHard")
		})
	}
}
