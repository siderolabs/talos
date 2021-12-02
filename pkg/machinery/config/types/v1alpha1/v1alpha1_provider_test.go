// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/bundle"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

type dynamicConfigProvider struct {
	externalIPs []net.IP
}

// Hostname implements DynamicConfigProvider.
func (cp *dynamicConfigProvider) Hostname(ctx context.Context) ([]byte, error) {
	return []byte("talos-default-master-1"), nil
}

// ExternalIPs implements DynamicConfigProvider.
func (cp *dynamicConfigProvider) ExternalIPs(ctx context.Context) ([]net.IP, error) {
	return cp.externalIPs, nil
}

// TestApplyDynamicConfig check ApplyDynamicConfig works and is idempotent.
func TestApplyDynamicConfig(t *testing.T) {
	b, err := bundle.NewConfigBundle(
		bundle.WithInputOptions(
			&bundle.InputOptions{
				ClusterName: "talos-default",
				Endpoint:    "10.5.0.1",
				KubeVersion: constants.DefaultKubernetesVersion,
			},
		),
	)
	require.NoError(t, err)

	config := b.ControlPlane()

	ctx := context.Background()

	provider := &dynamicConfigProvider{
		externalIPs: []net.IP{
			net.ParseIP("10.2.0.3"),
			net.ParseIP("10.10.1.2"),
		},
	}

	err = config.ApplyDynamicConfig(ctx, provider)
	require.NoError(t, err)

	c, ok := config.(*v1alpha1.Config)

	require.True(t, ok)

	require.Equal(t, "talos-default-master-1", c.Machine().Network().Hostname())
	require.Equal(t, []string{"10.2.0.3", "10.10.1.2"}, c.MachineConfig.CertSANs())

	provider.externalIPs = []net.IP{
		net.ParseIP("10.2.0.3"),
		net.ParseIP("10.10.1.2"),
	}

	provider = &dynamicConfigProvider{
		externalIPs: []net.IP{
			net.ParseIP("10.2.0.3"),
			net.ParseIP("10.10.1.2"),
			net.ParseIP("10.10.1.3"),
		},
	}

	err = config.ApplyDynamicConfig(ctx, provider)
	require.NoError(t, err)
	require.Equal(t, []string{"10.2.0.3", "10.10.1.2", "10.10.1.3"}, c.MachineConfig.CertSANs())
	require.Equal(t, []string{"10.2.0.3", "10.10.1.2", "10.10.1.3"}, c.ClusterConfig.CertSANs())

	c.MachineConfig.MachineNetwork.NetworkInterfaces = append(c.MachineConfig.MachineNetwork.NetworkInterfaces, &v1alpha1.Device{
		DeviceVIPConfig: &v1alpha1.DeviceVIPConfig{
			SharedIP: "192.168.88.77",
		},
		DeviceVlans: []*v1alpha1.Vlan{
			{
				VlanID: 100,
				VlanVIP: &v1alpha1.DeviceVIPConfig{
					SharedIP: "192.168.88.66",
				},
			},
		},
	})

	err = config.ApplyDynamicConfig(ctx, provider)
	require.NoError(t, err)
	require.Equal(t, []string{"10.2.0.3", "10.10.1.2", "10.10.1.3", "192.168.88.77", "192.168.88.66"}, c.MachineConfig.CertSANs())
}

func TestInterfaces(t *testing.T) {
	t.Parallel()

	assert.Implements(t, (*config.APIServer)(nil), (*v1alpha1.APIServerConfig)(nil))
	assert.Implements(t, (*config.ClusterConfig)(nil), (*v1alpha1.ClusterConfig)(nil))
	assert.Implements(t, (*config.ClusterNetwork)(nil), (*v1alpha1.ClusterConfig)(nil))
	assert.Implements(t, (*config.ControllerManager)(nil), (*v1alpha1.ControllerManagerConfig)(nil))
	assert.Implements(t, (*config.Etcd)(nil), (*v1alpha1.EtcdConfig)(nil))
	assert.Implements(t, (*config.ExternalCloudProvider)(nil), (*v1alpha1.ExternalCloudProviderConfig)(nil))
	assert.Implements(t, (*config.Features)(nil), (*v1alpha1.FeaturesConfig)(nil))
	assert.Implements(t, (*config.MachineConfig)(nil), (*v1alpha1.MachineConfig)(nil))
	assert.Implements(t, (*config.Scheduler)(nil), (*v1alpha1.SchedulerConfig)(nil))
	assert.Implements(t, (*config.Token)(nil), (*v1alpha1.ClusterConfig)(nil))

	tok := new(v1alpha1.ClusterConfig).Token()
	assert.Implements(t, (*config.Token)(nil), (tok))
}
