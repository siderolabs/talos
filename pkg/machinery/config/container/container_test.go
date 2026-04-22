// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"net/url"
	"testing"

	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/hardware"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime/extensions"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

func TestNew(t *testing.T) {
	t.Parallel()

	v1alpha1Cfg := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineFeatures: &v1alpha1.FeaturesConfig{
				DiskQuotaSupport: new(true),
			},
		},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterSecret: "topsecret",
		},
	}

	sideroLinkCfg := siderolink.NewConfigV1Alpha1()
	sideroLinkCfg.APIUrlConfig.URL = must.Value(url.Parse("https://siderolink.api/join?jointoken=secret&user=alice"))(t)

	extensionsCfg := extensions.NewServicesConfigV1Alpha1()
	extensionsCfg.ServiceName = "test-extension"
	extensionsCfg.ServiceConfigFiles = []extensions.ConfigFile{
		{
			ConfigFileContent:   "test",
			ConfigFileMountPath: "/etc/test",
		},
	}

	pciDriverRebindCfg := hardware.NewPCIDriverRebindConfigV1Alpha1()
	pciDriverRebindCfg.MetaName = "0000:04:00.00"
	pciDriverRebindCfg.PCITargetDriver = "vfio-pci"

	cfg, err := container.New(v1alpha1Cfg, sideroLinkCfg, extensionsCfg, pciDriverRebindCfg)
	require.NoError(t, err)

	assert.False(t, cfg.Readonly())
	assert.False(t, cfg.Debug())
	assert.True(t, cfg.Machine().Features().DiskQuotaSupportEnabled())
	assert.Equal(t, "topsecret", cfg.Cluster().Secret())
	assert.Equal(t, "https://siderolink.api/join?jointoken=secret&user=alice", cfg.SideroLink().APIUrl().String())
	assert.Equal(t, "test-extension", cfg.ExtensionServiceConfigs()[0].Name())
	assert.Equal(t, "0000:04:00.00", cfg.PCIDriverRebindConfig().PCIDriverRebindConfigs()[0].PCIID())
	assert.Same(t, v1alpha1Cfg, cfg.RawV1Alpha1())
	assert.Equal(t, []config.Document{v1alpha1Cfg, sideroLinkCfg, extensionsCfg, pciDriverRebindCfg}, cfg.Documents())

	bytes, err := cfg.Bytes()
	require.NoError(t, err)

	cfgBack, err := configloader.NewFromBytes(bytes)
	require.NoError(t, err)

	assert.True(t, cfgBack.Readonly())
	assert.NotEqual(t, v1alpha1Cfg, cfgBack.RawV1Alpha1())

	cfgRedacted := cfg.RedactSecrets("REDACTED")
	assert.Equal(t, "REDACTED", cfgRedacted.Cluster().Secret())
	assert.Equal(t, "https://siderolink.api/join?jointoken=REDACTED&user=alice", cfgRedacted.SideroLink().APIUrl().String())
}

func TestNewDuplicate(t *testing.T) {
	t.Parallel()

	v1alpha1Cfg1 := &v1alpha1.Config{}
	v1alpha1Cfg2 := &v1alpha1.Config{}

	siderolink1 := siderolink.NewConfigV1Alpha1()
	siderolink2 := siderolink.NewConfigV1Alpha1()

	_, err := container.New(v1alpha1Cfg1, siderolink1, v1alpha1Cfg2)
	assert.EqualError(t, err, "duplicate v1alpha1.Config")

	_, err = container.New(siderolink1, siderolink2)
	assert.EqualError(t, err, "duplicate document: SideroLinkConfig/")
}

func TestNewConflict(t *testing.T) {
	t.Parallel()

	v1alpha1Cfg1 := &v1alpha1.Config{}
	uv1 := block.NewUserVolumeConfigV1Alpha1()
	uv1.MetaName = "my-user-volume-1"
	uv2 := block.NewUserVolumeConfigV1Alpha1()
	uv2.MetaName = "my-user-volume-2"

	ev1 := block.NewExistingVolumeConfigV1Alpha1()
	ev1.MetaName = "my-user-volume-1"
	ev2 := block.NewExistingVolumeConfigV1Alpha1()
	ev2.MetaName = "my-user-volume-2"

	_, err := container.New(v1alpha1Cfg1, uv1, uv2, ev1)
	assert.EqualError(t, err, "conflicting documents: UserVolumeConfig/my-user-volume-1 and ExistingVolumeConfig/my-user-volume-1")

	_, err = container.New(uv1, uv2)
	require.NoError(t, err)

	_, err = container.New(ev2, ev1, uv1, uv2)
	assert.EqualError(t, err, "conflicting documents: ExistingVolumeConfig/my-user-volume-1 and UserVolumeConfig/my-user-volume-1")
}

func TestPatchV1Alpha1(t *testing.T) {
	t.Parallel()

	v1alpha1Cfg := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
		},
	}

	sideroLinkCfg := siderolink.NewConfigV1Alpha1()
	sideroLinkCfg.APIUrlConfig.URL = must.Value(url.Parse("https://siderolink.api/?jointoken=secret&user=alice"))(t)

	cfg, err := container.New(v1alpha1Cfg, sideroLinkCfg)
	require.NoError(t, err)

	patchedCfg, err := cfg.PatchV1Alpha1(func(cfg *v1alpha1.Config) error {
		cfg.MachineConfig.MachineType = "controlplane"

		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, machine.TypeWorker, cfg.Machine().Type())
	assert.Equal(t, machine.TypeControlPlane, patchedCfg.Machine().Type())

	assert.Equal(t, "https://siderolink.api/?jointoken=secret&user=alice", cfg.SideroLink().APIUrl().String())
	assert.Equal(t, "https://siderolink.api/?jointoken=secret&user=alice", patchedCfg.SideroLink().APIUrl().String())
}

func TestRunDefaultDHCPOperators(t *testing.T) {
	t.Parallel()

	v1alpha1Cfg := &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{},
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
		},
	}

	dummyLinkConfig := network.NewDummyLinkConfigV1Alpha1("dummy1")

	for _, tt := range []struct {
		name      string
		documents []config.Document

		expected bool
	}{
		{
			name:      "empty",
			documents: []config.Document{},

			expected: true,
		},
		{
			name:      "only v1alpha1",
			documents: []config.Document{v1alpha1Cfg},

			expected: true,
		},
		{
			name:      "has dummy link config",
			documents: []config.Document{v1alpha1Cfg, dummyLinkConfig},

			expected: false,
		},
		{
			name:      "only dummy link config",
			documents: []config.Document{dummyLinkConfig},

			expected: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(tt.documents...)
			require.NoError(t, err)

			assert.Equal(t, tt.expected, ctr.RunDefaultDHCPOperators())
		})
	}
}
