// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

func TestRegisterResource(t *testing.T) {
	ctx := context.TODO()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(resources)

	for _, resource := range []resource.Resource{
		&k8s.AdmissionControlConfig{},
		&k8s.APIServerConfig{},
		&k8s.AuditPolicyConfig{},
		&k8s.ConfigStatus{},
		&k8s.ControllerManagerConfig{},
		&k8s.Endpoint{},
		&k8s.ExtraManifestsConfig{},
		&k8s.KubeletConfig{},
		&k8s.KubeletLifecycle{},
		&k8s.KubeletSpec{},
		&k8s.ManifestStatus{},
		&k8s.Manifest{},
		&k8s.BootstrapManifestsConfig{},
		&k8s.Nodename{},
		&k8s.NodeIP{},
		&k8s.NodeIPConfig{},
		&k8s.SchedulerConfig{},
		&k8s.SecretsStatus{},
		&k8s.StaticPodStatus{},
		&k8s.StaticPod{},
	} {
		assert.NoError(t, resourceRegistry.Register(ctx, resource))
	}
}

func TestKubeletConfig(t *testing.T) {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v1.0.0"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.10"}
	cfg.TypedSpec().ClusterDomain = "cluster.local"
	cfg.TypedSpec().ExtraArgs = map[string]string{"foo": "bar"}
	cfg.TypedSpec().ExtraMounts = []specs.Mount{
		{
			Destination: "/tmp",
			Source:      "/var",
			Type:        "tmpfs",
		},
	}
	cfg.TypedSpec().CloudProviderExternal = true

	res, err := protobuf.FromResource(cfg)
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestKubeletSpec(t *testing.T) {
	cfg := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v1.0.0"
	cfg.TypedSpec().ExtraMounts = []specs.Mount{
		{
			Destination: "/tmp",
			Source:      "/var",
			Type:        "tmpfs",
		},
	}

	res, err := protobuf.FromResource(cfg)
	require.NoError(t, err)
	require.NotNil(t, res)
}
