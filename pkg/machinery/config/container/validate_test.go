// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"context"
	"net/netip"
	"net/url"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestValidateAsClient(t *testing.T) {
	t.Parallel()

	sideroLinkCfg := siderolink.NewConfigV1Alpha1()
	sideroLinkCfg.APIUrlConfig.URL = must.Value(url.Parse("https://siderolink.api/?jointoken=secret&user=alice"))(t)

	invalidSideroLinkCfg := siderolink.NewConfigV1Alpha1()

	v1alpha1Cfg := &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{ //nolint:staticcheck // testing legacy features
				Endpoint: &v1alpha1.Endpoint{
					URL: must.Value(url.Parse("https://localhost:6443"))(t),
				},
			},
		},
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
			MachineCA: &x509.PEMEncodedCertificateAndKey{
				Crt: []byte("cert"),
			},
		},
	}

	invalidV1alpha1Config := &v1alpha1.Config{}

	for _, tt := range []struct {
		name      string
		documents []config.Document

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
		},
		{
			name:      "multi-doc",
			documents: []config.Document{sideroLinkCfg, v1alpha1Cfg},
		},
		{
			name:      "only siderolink",
			documents: []config.Document{sideroLinkCfg},
		},
		{
			name:      "only v1alpha1",
			documents: []config.Document{v1alpha1Cfg},
		},
		{
			name:          "invalid siderolink",
			documents:     []config.Document{invalidSideroLinkCfg},
			expectedError: "1 error occurred:\n\t* SideroLinkConfig: apiUrl is required\n\n",
		},
		{
			name:          "invalid v1alpha1",
			documents:     []config.Document{invalidV1alpha1Config},
			expectedError: "1 error occurred:\n\t* v1alpha1.Config: 1 error occurred:\n\t* machine instructions are required\n\n\n\n",
		},
		{
			name:          "invalid multi-doc",
			documents:     []config.Document{invalidSideroLinkCfg, invalidV1alpha1Config},
			expectedError: "2 errors occurred:\n\t* v1alpha1.Config: 1 error occurred:\n\t* machine instructions are required\n\n\n\t* SideroLinkConfig: apiUrl is required\n\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(tt.documents...)
			require.NoError(t, err)

			warnings, err := ctr.ValidateAsClient(validationMode{})

			if tt.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.expectedError)
			}

			require.Equal(t, tt.expectedWarnings, warnings)
		})
	}
}

func TestCrossValidateEncryption(t *testing.T) {
	t.Parallel()

	v1alpha1Cfg := &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{ //nolint:staticcheck // testing legacy features
				Endpoint: &v1alpha1.Endpoint{
					URL: must.Value(url.Parse("https://localhost:6443"))(t),
				},
			},
		},
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
			MachineCA: &x509.PEMEncodedCertificateAndKey{
				Crt: []byte("cert"),
			},
			MachineSystemDiskEncryption: &v1alpha1.SystemDiskEncryptionConfig{ //nolint:staticcheck // testing legacy features
				EphemeralPartition: &v1alpha1.EncryptionConfig{
					EncryptionKeys: []*v1alpha1.EncryptionKey{
						{
							KeySlot: 1,
							KeyStatic: &v1alpha1.EncryptionKeyStatic{
								KeyData: "static-key",
							},
						},
					},
				},
			},
		},
	}

	defaultEphemeral := block.NewVolumeConfigV1Alpha1()
	defaultEphemeral.MetaName = constants.EphemeralPartitionLabel

	encryptedEphemeral := block.NewVolumeConfigV1Alpha1()
	encryptedEphemeral.MetaName = constants.EphemeralPartitionLabel
	encryptedEphemeral.EncryptionSpec = block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys: []block.EncryptionKey{
			{
				KeySlot: 2,
				KeyStatic: &block.EncryptionKeyStatic{
					KeyData: "encrypted-static-key",
				},
			},
		},
	}

	encryptedState := block.NewVolumeConfigV1Alpha1()
	encryptedState.MetaName = constants.StatePartitionLabel
	encryptedState.EncryptionSpec = block.EncryptionSpec{
		EncryptionProvider: blockres.EncryptionProviderLUKS2,
		EncryptionKeys: []block.EncryptionKey{
			{
				KeySlot: 3,
				KeyTPM:  &block.EncryptionKeyTPM{},
			},
		},
	}

	for _, tt := range []struct {
		name      string
		documents []config.Document

		expectedError    string
		expectedWarnings []string
	}{
		{
			name:      "only v1alpha1",
			documents: []config.Document{v1alpha1Cfg},
		},
		{
			name:      "v1alpha1 with no-conflict volumes",
			documents: []config.Document{v1alpha1Cfg, defaultEphemeral, encryptedState},
		},
		{
			name:      "v1alpha1 with no-conflict volumes",
			documents: []config.Document{v1alpha1Cfg, encryptedState},
		},
		{
			name:      "no v1alpha1",
			documents: []config.Document{encryptedEphemeral, encryptedState},
		},
		{
			name:          "conflict on ephemeral encryption",
			documents:     []config.Document{v1alpha1Cfg, encryptedEphemeral},
			expectedError: "1 error occurred:\n\t* system disk encryption for \"EPHEMERAL\" is configured in both v1alpha1.Config and VolumeConfig\n\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(tt.documents...)
			require.NoError(t, err)

			warnings, err := ctr.ValidateAsClient(validationMode{})

			if tt.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.expectedError)
			}

			require.Equal(t, tt.expectedWarnings, warnings)
		})
	}
}

func TestValidateContainer(t *testing.T) {
	t.Parallel()

	sideroLinkCfg := siderolink.NewConfigV1Alpha1()
	sideroLinkCfg.APIUrlConfig.URL = must.Value(url.Parse("https://siderolink.api/?jointoken=secret&user=alice"))(t)

	v1alpha1Cfg := &v1alpha1.Config{
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{ //nolint:staticcheck // testing legacy features
				Endpoint: &v1alpha1.Endpoint{
					URL: must.Value(url.Parse("https://localhost:6443"))(t),
				},
			},
		},
		MachineConfig: &v1alpha1.MachineConfig{
			MachineType: "worker",
			MachineCA: &x509.PEMEncodedCertificateAndKey{
				Crt: []byte("cert"),
			},
		},
	}

	v1alpha1CfgHostDNS := v1alpha1Cfg.DeepCopy()
	v1alpha1CfgHostDNS.MachineConfig.MachineFeatures = &v1alpha1.FeaturesConfig{
		HostDNSSupport: &v1alpha1.HostDNSConfig{ //nolint:staticcheck // testing legacy features
			HostDNSConfigEnabled:        new(true),
			HostDNSForwardKubeDNSToHost: new(true),
		},
	}

	v1alpha1CfgControlplane := v1alpha1Cfg.DeepCopy()
	v1alpha1CfgControlplane.MachineConfig.MachineType = "controlplane"
	v1alpha1CfgControlplane.MachineConfig.MachineCA.Key = []byte("controlplane-key")

	resolverConfig := network.NewResolverConfigV1Alpha1()
	resolverConfig.ResolverNameservers = []network.NameserverConfig{
		{
			Address: meta.Addr{Addr: netip.MustParseAddr("1.1.1.1")},
		},
	}

	hostDNSResolverConfig := network.NewResolverConfigV1Alpha1()
	hostDNSResolverConfig.ResolverHostDNS = network.HostDNSConfig{
		HostDNSEnabled:              new(true),
		HostDNSForwardKubeDNSToHost: new(true),
	}

	resolverConfigDoT := network.NewResolverConfigV1Alpha1()
	resolverConfigDoT.ResolverNameservers = []network.NameserverConfig{
		{
			Address: meta.Addr{
				Addr: netip.MustParseAddr("1.1.1.1"),
			},
			Protocol:      nethelpers.DNSProtocolDNSOverTLS,
			TLSServerName: "cloudflare-dns.com",
		},
	}

	kubeEtcdEncryptionConfig := k8s.NewKubeEtcdEncryptionConfigV1Alpha1()
	kubeEtcdEncryptionConfig.Config = meta.Unstructured{
		Object: map[string]any{
			"some": "thing",
		},
	}

	kubespanConfig := network.NewKubeSpanV1Alpha1()
	kubespanConfig.ConfigEnabled = new(true)

	discoveryIdentityConfig := cluster.NewDiscoveryIdentityConfigV1Alpha1(
		"MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=",
		"vlf2HU1NEZL3Ezi9Tk+RZBLJUbjnsHnTzs3wK9JNk6Q=",
	)

	discoveryServiceConfig := cluster.NewDiscoveryServiceConfigV1Alpha1("default", must.Value(url.Parse("https://discovery.api/"))(t))

	apiServerCAConfig := k8s.NewKubeAPIServerCAConfigV1Alpha1()

	// the cluster endpoint was migrated out of the v1alpha1 config
	v1alpha1CfgControlplaneNoEndpoint := v1alpha1CfgControlplane.DeepCopy()
	v1alpha1CfgControlplaneNoEndpoint.ClusterConfig.ControlPlane = nil //nolint:staticcheck // testing legacy features

	kubeClusterConfig := k8s.NewKubeClusterConfigV1Alpha1()
	kubeClusterConfig.ClusterNameConfig = "test-cluster"
	kubeClusterConfig.ClusterEndpointConfig = meta.URL{URL: must.Value(url.Parse("https://localhost:6443"))(t)}

	// .machine.nodeLabels migrated to the KubeNodeConfig document as-is, without the control plane role label
	kubeNodeConfig := k8s.NewKubeNodeConfigV1Alpha1()
	kubeNodeConfig.LabelsConfig = map[string]string{
		"rack": "r13a25",
	}

	kubeNodeConfigControlplane := k8s.NewKubeNodeConfigV1Alpha1()
	kubeNodeConfigControlplane.LabelsConfig = map[string]string{
		constants.LabelNodeRoleControlPlane: "",
	}
	kubeNodeConfigControlplane.TaintsConfig = map[string]string{
		constants.LabelNodeRoleControlPlane: constants.TaintEffectNoSchedule,
	}

	kubeNodeConfigStandalone := k8s.NewKubeNodeConfigV1Alpha1()
	kubeNodeConfigStandalone.SkipNodeRegistrationConfig = new(true)

	encryptedDNSWarning := "all configured nameservers use encrypted DNS (DoT or DoH): validating certificates requires a correct system clock, " +
		"so boot may stall when NTP servers are configured by hostname; consider keeping at least one plain-DNS fallback or configuring NTP servers by IP address"

	controlPlaneLabelWarning := "KubeNodeConfig document should set the \"node-role.kubernetes.io/control-plane\" node label on control plane machines " +
		"(and, unless scheduling on control planes is allowed, the \"node-role.kubernetes.io/control-plane: NoSchedule\" taint): " +
		"unlike .machine.nodeLabels/.machine.nodeTaints, the document contents are used as-is"

	for _, tt := range []struct {
		name        string
		documents   []config.Document
		inContainer bool

		expectedWarnings []string
		expectedError    string
	}{
		{
			name: "empty !container",
		},
		{
			name:        "empty container",
			inContainer: true,

			expectedError: "1 error occurred:\n\t* hostDNS config is required in container mode\n\n",
		},
		{
			name:        "empty v1alpha1 container",
			documents:   []config.Document{v1alpha1Cfg},
			inContainer: true,

			expectedError: "1 error occurred:\n\t* hostDNS config is required in container mode\n\n",
		},
		{
			name:        "just resolver in container",
			documents:   []config.Document{resolverConfig},
			inContainer: true,

			expectedError: "1 error occurred:\n\t* hostDNS config is required in container mode\n\n",
		},
		{
			name:        "hostDNS v1alpha1 container",
			documents:   []config.Document{v1alpha1CfgHostDNS},
			inContainer: true,
		},
		{
			name:        "hostDNS v1alpha1 container plus multi-doc",
			documents:   []config.Document{v1alpha1CfgHostDNS, resolverConfig},
			inContainer: true,
		},
		{
			name:        "just multi-doc with hostDNS",
			documents:   []config.Document{hostDNSResolverConfig},
			inContainer: true,
		},
		{
			name:        "multi-doc with hostDNS and v1alpha1",
			documents:   []config.Document{hostDNSResolverConfig, v1alpha1Cfg},
			inContainer: true,
		},
		{
			name:             "DoT without hostDNS",
			documents:        []config.Document{resolverConfigDoT},
			expectedWarnings: []string{encryptedDNSWarning},
			expectedError:    "1 error occurred:\n\t* hostDNS must be enabled when using non-default DNS protocols\n\n",
		},
		{
			name:             "DoT with hostDNS",
			documents:        []config.Document{resolverConfigDoT, v1alpha1CfgHostDNS},
			expectedWarnings: []string{encryptedDNSWarning},
		},
		{
			name:      "controlplane doc only",
			documents: []config.Document{kubeEtcdEncryptionConfig},

			expectedError: "1 error occurred:\n\t* the following document kinds are only allowed on control plane machines: [KubeEtcdEncryptionConfig]\n\n",
		},
		{
			name:      "controlplane doc with v1alpha1 worker",
			documents: []config.Document{v1alpha1Cfg, kubeEtcdEncryptionConfig},

			expectedError: "1 error occurred:\n\t* the following document kinds are only allowed on control plane machines: [KubeEtcdEncryptionConfig]\n\n",
		},
		{
			name:      "kubespan without discovery",
			documents: []config.Document{v1alpha1Cfg, kubespanConfig},

			expectedError: "1 error occurred:\n\t* KubeSpan requires cluster discovery to be enabled\n\n",
		},
		{
			name:      "discovery without identity",
			documents: []config.Document{discoveryServiceConfig, kubespanConfig},

			expectedError: "2 errors occurred:\n\t* cluster ID (.cluster.id or DiscoveryIdentityConfig) should be set when cluster discovery (DiscoveryServiceConfig) is enabled\n" +
				"\t* cluster secret (.cluster.secret or DiscoveryIdentityConfig) should be set when cluster discovery (DiscoveryServiceConfig) is enabled\n\n",
		},
		{
			name:      "discovery with kubespan and identity",
			documents: []config.Document{discoveryServiceConfig, discoveryIdentityConfig, kubespanConfig},
		},
		{
			name:      "api-server CA without etcd encryption",
			documents: []config.Document{v1alpha1CfgControlplane, apiServerCAConfig},

			expectedError: "1 error occurred:\n\t* etcd encryption config is required for control plane machines running kube-apiserver\n\n",
		},
		{
			// the cluster endpoint was removed from the v1alpha1 config, but the KubeClusterConfig
			// document was not added: Kubernetes is configured, but there is no way to reach it
			name:      "api-server CA without cluster endpoint",
			documents: []config.Document{v1alpha1CfgControlplaneNoEndpoint, apiServerCAConfig, kubeEtcdEncryptionConfig},

			expectedError: "1 error occurred:\n\t* cluster name and endpoint are required when Kubernetes is configured: " +
				"either .cluster.clusterName/.cluster.controlPlane.endpoint or the KubeClusterConfig document\n\n",
		},
		{
			name:      "api-server CA with migrated cluster endpoint",
			documents: []config.Document{v1alpha1CfgControlplaneNoEndpoint, apiServerCAConfig, kubeEtcdEncryptionConfig, kubeClusterConfig},
		},
		{
			// the control plane role label is not synthesized for the KubeNodeConfig document
			name:      "node config without control plane role label",
			documents: []config.Document{v1alpha1CfgControlplane, kubeNodeConfig},

			expectedWarnings: []string{controlPlaneLabelWarning},
		},
		{
			name:      "node config with control plane role label",
			documents: []config.Document{v1alpha1CfgControlplane, kubeNodeConfigControlplane},
		},
		{
			// the node is not registered in Kubernetes, so the labels and taints are not used
			name:      "node config with skipped node registration",
			documents: []config.Document{v1alpha1CfgControlplane, kubeNodeConfigStandalone},
		},
		{
			// worker machines never get the control plane role label
			name:      "node config without control plane role label on a worker",
			documents: []config.Document{v1alpha1Cfg, kubeNodeConfig},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(tt.documents...)
			require.NoError(t, err)

			warnings, err := ctr.ValidateAsClient(validationMode{inContainer: tt.inContainer})

			assert.Equal(t, tt.expectedWarnings, warnings)

			if tt.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.expectedError)
			}
		})
	}
}

func TestRuntimeValidateSystemVolumeBacking(t *testing.T) {
	t.Parallel()

	const (
		documentAbsent              = "absent"
		documentWithoutProvisioning = "directory"
		documentWithProvisioning    = "partition"
	)

	for _, test := range []struct {
		name string

		// document controls the KUBELET VolumeConfig document in the config:
		// absent (removed), present without provisioning, or present with provisioning.
		document string

		// current volume to seed into the state (skipped if seedStatus is false).
		seedStatus   bool
		currentType  blockres.VolumeType
		currentPhase blockres.VolumePhase

		expectedErrorContains string
	}{
		{
			name:         "removing config of a ready partition volume is rejected",
			document:     documentAbsent,
			seedStatus:   true,
			currentType:  blockres.VolumeTypePartition,
			currentPhase: blockres.VolumePhaseReady,

			expectedErrorContains: `the "KUBELET" system volume is backed by a dedicated partition and its VolumeConfig cannot be removed; ` +
				`migrating a system volume off a dedicated partition is not supported`,
		},
		{
			name:         "removing config of a ready directory volume is allowed",
			document:     documentAbsent,
			seedStatus:   true,
			currentType:  blockres.VolumeTypeDirectory,
			currentPhase: blockres.VolumePhaseReady,
		},
		{
			name:       "removing config with no established volume is allowed (cluster creation)",
			document:   documentAbsent,
			seedStatus: false,
		},
		{
			name:         "removing config of an unsettled partition volume is allowed",
			document:     documentAbsent,
			seedStatus:   true,
			currentType:  blockres.VolumeTypePartition,
			currentPhase: blockres.VolumePhaseWaiting,
		},
		{
			name:         "demoting via present config (drop provisioning) is still rejected by the per-document guard",
			document:     documentWithoutProvisioning,
			seedStatus:   true,
			currentType:  blockres.VolumeTypePartition,
			currentPhase: blockres.VolumePhaseReady,

			expectedErrorContains: `the backing of the "KUBELET" system volume cannot be changed after creation (current: partition, requested: directory)`,
		},
		{
			name:         "matching present partition config is allowed",
			document:     documentWithProvisioning,
			seedStatus:   true,
			currentType:  blockres.VolumeTypePartition,
			currentPhase: blockres.VolumePhaseReady,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			if test.seedStatus {
				vs := blockres.NewVolumeStatus(blockres.NamespaceName, constants.KubeletDataVolumeID)
				vs.TypedSpec().Type = test.currentType
				vs.TypedSpec().Phase = test.currentPhase
				require.NoError(t, st.Create(ctx, vs))
			}

			var documents []config.Document

			if test.document != documentAbsent {
				vc := block.NewVolumeConfigV1Alpha1()
				vc.MetaName = constants.KubeletDataVolumeID

				if test.document == documentWithProvisioning {
					vc.ProvisioningSpec.ProvisioningMaxSize = block.MustSize("5GB")
				}

				documents = append(documents, vc)
			}

			ctr, err := container.New(documents...)
			require.NoError(t, err)

			_, err = ctr.ValidateAtRuntime(ctx, st, validationMode{})

			if test.expectedErrorContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.expectedErrorContains)
			}
		})
	}
}

type validationMode struct {
	inContainer bool
}

func (validationMode) String() string {
	return ""
}

func (validationMode) RequiresInstall() bool {
	return false
}

func (v validationMode) InContainer() bool {
	return v.inContainer
}
