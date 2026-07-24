// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"net/url"
	"path/filepath"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	clustertypes "github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
	critypes "github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/hardware"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	runtimeconfig "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime/extensions"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
	assert.Equal(t, "topsecret", cfg.DiscoveryIdentityConfig().ClusterSecret())
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
	assert.Equal(t, "REDACTED", cfgRedacted.DiscoveryIdentityConfig().ClusterSecret())
	assert.Equal(t, "https://siderolink.api/join?jointoken=REDACTED&user=alice", cfgRedacted.SideroLink().APIUrl().String())
}

func TestNetworkBGPConfigs(t *testing.T) {
	t.Parallel()

	cfg, err := container.New()
	require.NoError(t, err)
	assert.Empty(t, cfg.NetworkBGPInstanceConfigs())

	instance1 := network.NewBGPInstanceConfigV1Alpha1("fabric")
	instance2 := network.NewBGPInstanceConfigV1Alpha1("metallb")

	cfg, err = container.New(instance1, instance2)
	require.NoError(t, err)
	assert.Equal(t, []config.NetworkBGPInstanceConfig{instance1, instance2}, cfg.NetworkBGPInstanceConfigs())
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

func TestCRICustomizationConfigs(t *testing.T) {
	t.Parallel()

	legacy := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineFiles: []*v1alpha1.MachineFile{ //nolint:staticcheck // test deprecated compatibility
				{
					FilePath:    filepath.Join("/etc", constants.CRICustomizationConfigPart),
					FileContent: "legacy",
				},
			},
		},
	}

	document := critypes.NewCRICustomizationConfigV1Alpha1("document")
	document.CustomizationContent = "document"

	cfg, err := container.New(legacy, document)
	require.NoError(t, err)

	customizations := cfg.CRICustomizationConfigs()
	require.Len(t, customizations, 2)
	assert.Equal(t, config.LegacyCRICustomizationConfigName, customizations[0].Name())
	assert.Equal(t, "legacy", customizations[0].Content())
	assert.Equal(t, "document", customizations[1].Name())
	assert.Equal(t, "document", customizations[1].Content())
}

func TestCRIBaseRuntimeSpecConfig(t *testing.T) {
	t.Parallel()

	document := critypes.NewCRIBaseRuntimeSpecConfigV1Alpha1()
	document.OverridesConfig.Object = map[string]any{
		"process": map[string]any{"noNewPrivileges": true},
	}

	cfg, err := container.New(document)
	require.NoError(t, err)

	assert.Equal(t, document, cfg.CRIBaseRuntimeSpecConfig())

	legacy := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineBaseRuntimeSpecOverrides: meta.Unstructured{ //nolint:staticcheck // test deprecated compatibility
				Object: map[string]any{"process": map[string]any{"cwd": "/legacy"}},
			},
		},
	}

	cfg, err = container.New(legacy)
	require.NoError(t, err)

	require.NotNil(t, cfg.CRIBaseRuntimeSpecConfig())
	assert.Equal(t, legacy.MachineConfig.MachineBaseRuntimeSpecOverrides.Object, cfg.CRIBaseRuntimeSpecConfig().Overrides()) //nolint:staticcheck // test deprecated compatibility
}

func TestVethLinkConfigs(t *testing.T) {
	t.Parallel()

	veth := network.NewVethConfigV1Alpha1("veth-host", "veth-router")

	cfg, err := container.New(veth)
	require.NoError(t, err)

	links := cfg.NetworkCommonLinkConfigs()
	require.Len(t, links, 2)
	assert.Equal(t, "veth-host", links[0].Name())
	assert.Equal(t, "veth-router", links[1].Name())

	second := network.NewVethConfigV1Alpha1("veth-host-2", "veth-router-2")
	cfg, err = container.New(veth, second)
	require.NoError(t, err)
	assert.Len(t, cfg.NetworkCommonLinkConfigs(), 4)

	reverse := network.NewVethConfigV1Alpha1("veth-router", "veth-host")
	_, err = container.New(veth, reverse)
	assert.EqualError(t, err, `conflicting link configurations: VethConfig/veth-host and VethConfig/veth-router both configure "veth-router"`)

	physical := network.NewLinkConfigV1Alpha1("veth-router")
	_, err = container.New(veth, physical)
	assert.EqualError(t, err, `conflicting link configurations: VethConfig/veth-host and LinkConfig/veth-router both configure "veth-router"`)

	_, err = container.New(physical, veth)
	assert.EqualError(t, err, `conflicting link configurations: LinkConfig/veth-router and VethConfig/veth-host both configure "veth-router"`)

	dummy := network.NewDummyLinkConfigV1Alpha1("veth-router")
	_, err = container.New(veth, dummy)
	assert.EqualError(t, err, `conflicting link configurations: VethConfig/veth-host and DummyLinkConfig/veth-router both configure "veth-router"`)
}

func TestUdevRulesConfig(t *testing.T) {
	t.Parallel()

	v1alpha1Cfg := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineUdev: &v1alpha1.UdevConfig{ //nolint:staticcheck // legacy config
				UdevRules: []string{"legacy-rule"},
			},
		},
	}

	cfg, err := container.New(v1alpha1Cfg)
	require.NoError(t, err)

	require.NotNil(t, cfg.UdevRulesConfig())
	assert.Equal(t, []string{"legacy-rule"}, cfg.UdevRulesConfig().Rules())

	udevRulesCfg := runtimeconfig.NewUdevRulesConfigV1Alpha1()
	udevRulesCfg.UdevRules = []string{"document-rule"}

	cfg, err = container.New(v1alpha1Cfg, udevRulesCfg)
	require.NoError(t, err)

	require.NotNil(t, cfg.UdevRulesConfig())
	assert.Equal(t, []string{"document-rule"}, cfg.UdevRulesConfig().Rules())
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

func TestDiscoveryServiceConfigs(t *testing.T) {
	t.Parallel()

	// legacy v1alpha1 cluster discovery config, surfaces as a single config named "legacy"
	legacyEnabled := &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // legacy config
				DiscoveryEnabled: new(true),
				DiscoveryRegistries: v1alpha1.DiscoveryRegistriesConfig{ //nolint:staticcheck // legacy config
					RegistryService: v1alpha1.RegistryServiceConfig{ //nolint:staticcheck // legacy config
						RegistryEndpoint: "https://legacy.discovery.test/",
					},
				},
			},
		},
	}

	// legacy cluster discovery disabled, surfaces no config
	legacyDisabled := &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{
			ClusterDiscoveryConfig: &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // legacy config
				DiscoveryEnabled: new(false),
			},
		},
	}

	primaryDoc := clustertypes.NewDiscoveryServiceConfigV1Alpha1("primary", must.Value(url.Parse("https://primary.discovery.test/"))(t))
	secondaryDoc := clustertypes.NewDiscoveryServiceConfigV1Alpha1("secondary", must.Value(url.Parse("grpc://secondary.discovery.test/"))(t))

	for _, tt := range []struct {
		name      string
		documents []config.Document

		// expected (name -> endpoint) of the returned configs
		expected map[string]string
	}{
		{
			name:      "no configs at all",
			documents: []config.Document{&v1alpha1.Config{}},
			expected:  map[string]string{},
		},
		{
			// v1alpha1 with a cluster config but no discovery block must not panic
			name:      "v1alpha1 without discovery block",
			documents: []config.Document{&v1alpha1.Config{ClusterConfig: &v1alpha1.ClusterConfig{}}},
			expected:  map[string]string{},
		},
		{
			name:      "only legacy",
			documents: []config.Document{legacyEnabled},
			expected:  map[string]string{"legacy": "https://legacy.discovery.test/"},
		},
		{
			name:      "legacy disabled",
			documents: []config.Document{legacyDisabled},
			expected:  map[string]string{},
		},
		{
			name:      "only multi-doc, no v1alpha1 config present",
			documents: []config.Document{primaryDoc, secondaryDoc},
			expected: map[string]string{
				"primary":   "https://primary.discovery.test/",
				"secondary": "grpc://secondary.discovery.test/",
			},
		},
		{
			name:      "legacy disabled with multi-doc",
			documents: []config.Document{legacyDisabled, primaryDoc},
			expected: map[string]string{
				"primary": "https://primary.discovery.test/",
			},
		},
		{
			// such a config is rejected by validation, but the accessor still prefers the v1alpha1 config
			name:      "legacy takes precedence over documents",
			documents: []config.Document{legacyEnabled, primaryDoc, secondaryDoc},
			expected: map[string]string{
				"legacy": "https://legacy.discovery.test/",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(tt.documents...)
			require.NoError(t, err)

			got := ctr.DiscoveryServiceConfigs()

			// len check also guards against duplicate names collapsing in the map below
			assert.Len(t, got, len(tt.expected), "returned configs should not contain duplicate names")

			actual := xslices.ToMap(got, func(c config.DiscoveryServiceConfig) (string, string) {
				return c.Name(), c.Endpoint().String()
			})

			for name, endpoint := range tt.expected {
				assert.Equal(t, endpoint, actual[name], "discovery service config %q", name)
			}
		})
	}
}

func TestKernelModuleConfigsMixing(t *testing.T) {
	t.Parallel()

	legacy := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineKernel: &v1alpha1.KernelConfig{ //nolint:staticcheck // legacy configuration
				KernelModules: []*v1alpha1.KernelModuleConfig{ //nolint:staticcheck // legacy configuration
					{
						ModuleName:       "btrfs",
						ModuleParameters: []string{"legacy-param"},
					},
					{
						ModuleName: "e1000",
					},
				},
			},
		},
	}

	overlappingDoc := runtimeconfig.NewKernelModuleConfigV1Alpha1("btrfs")
	overlappingDoc.ModuleParameters = []string{"doc-param"}

	standaloneDoc := runtimeconfig.NewKernelModuleConfigV1Alpha1("vrf")

	for _, tt := range []struct {
		name      string
		documents []config.Document

		// expected (name -> parameters) of the returned modules, in order
		expected [][2]any
	}{
		{
			name:      "no config at all",
			documents: []config.Document{&v1alpha1.Config{}},
			expected:  nil,
		},
		{
			name:      "only legacy",
			documents: []config.Document{legacy},
			expected: [][2]any{
				{"btrfs", []string{"legacy-param"}},
				{"e1000", []string(nil)},
			},
		},
		{
			name:      "only multi-doc, no v1alpha1 config present",
			documents: []config.Document{standaloneDoc},
			expected: [][2]any{
				{"vrf", []string(nil)},
			},
		},
		{
			name:      "legacy and non-overlapping multi-doc are merged",
			documents: []config.Document{legacy, standaloneDoc},
			expected: [][2]any{
				{"btrfs", []string{"legacy-param"}},
				{"e1000", []string(nil)},
				{"vrf", []string(nil)},
			},
		},
		{
			// the document is ordered after the legacy entries, so downstream consumers processing
			// the list in order and keying by module name (as KernelModuleConfigController does) see
			// the document's parameters win on a name conflict.
			name:      "multi-doc is appended after legacy on module name conflict",
			documents: []config.Document{legacy, overlappingDoc},
			expected: [][2]any{
				{"btrfs", []string{"legacy-param"}},
				{"e1000", []string(nil)},
				{"btrfs", []string{"doc-param"}},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(tt.documents...)
			require.NoError(t, err)

			got := ctr.KernelModuleConfigs()

			actual := xslices.Map(got, func(m config.KernelModuleConfig) [2]any {
				return [2]any{m.Name(), m.Parameters()}
			})

			assert.Equal(t, tt.expected, actual)
		})
	}
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
