// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"fmt"
	"net/url"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	clustertypes "github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//nolint:gocyclo,cyclop
func (in *Input) init() ([]config.Document, error) {
	v1alpha1Config := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ConfigDebug:   new(in.Options.Debug),
		ConfigPersist: new(true),
	}

	machine := &v1alpha1.MachineConfig{
		MachineType: machine.TypeInit.String(),
		MachineKubelet: &v1alpha1.KubeletConfig{
			KubeletImage: fmt.Sprintf("%s:v%s", constants.KubeletImage, in.KubernetesVersion),
		},
		MachineCA:       in.Options.SecretsBundle.Certs.OS,
		MachineCertSANs: in.AdditionalMachineCertSANs,
		MachineToken:    in.Options.SecretsBundle.TrustdInfo.Token,
		MachineInstall: &v1alpha1.InstallConfig{
			InstallDisk:            in.Options.InstallDisk,
			InstallImage:           in.Options.InstallImage,
			InstallWipe:            new(false),
			InstallExtraKernelArgs: in.Options.InstallExtraKernelArgs,
		},
		MachineDisks:    in.Options.MachineDisks,
		MachineFeatures: &v1alpha1.FeaturesConfig{},
	}

	if !in.Options.VersionContract.MultidocSysctlConfigSupported() {
		machine.MachineSysctls = in.Options.Sysctls //nolint:staticcheck // legacy configuration
	}

	if in.Options.VersionContract.GrubUseUKICmdlineDefault() {
		machine.MachineInstall.InstallGrubUseUKICmdline = new(true)
	}

	if !in.Options.VersionContract.HideRBACAndKeyUsage() {
		machine.MachineFeatures.RBAC = new(true)

		if in.Options.VersionContract.ApidExtKeyUsageCheckEnabled() {
			machine.MachineFeatures.ApidCheckExtKeyUsage = new(true)
		}
	}

	if in.Options.VersionContract.DiskQuotaSupportEnabled() {
		machine.MachineFeatures.DiskQuotaSupport = new(true)
	}

	if kubePrismPort, optionSet := in.Options.KubePrismPort.Get(); optionSet { // default to enabled, but if set explicitly, allow it to be disabled
		if kubePrismPort > 0 {
			machine.MachineFeatures.KubePrismSupport = &v1alpha1.KubePrism{
				ServerEnabled: new(true),
				ServerPort:    kubePrismPort,
			}
		}
	} else if in.Options.VersionContract.KubePrismEnabled() {
		machine.MachineFeatures.KubePrismSupport = &v1alpha1.KubePrism{
			ServerEnabled: new(true),
			ServerPort:    constants.DefaultKubePrismPort,
		}
	}

	if in.Options.VersionContract.KubeletDefaultRuntimeSeccompProfileEnabled() {
		machine.MachineKubelet.KubeletDefaultRuntimeSeccompProfileEnabled = new(true)
	}

	if in.Options.VersionContract.KubeletManifestsDirectoryDisabled() {
		machine.MachineKubelet.KubeletDisableManifestsDirectory = new(true)
	}

	if in.Options.VersionContract.HostDNSEnabled() && !in.Options.VersionContract.HostDNSMultidocConfig() {
		machine.MachineFeatures.HostDNSSupport = &v1alpha1.HostDNSConfig{ //nolint:staticcheck // legacy configuration
			HostDNSConfigEnabled:        new(true),
			HostDNSForwardKubeDNSToHost: ptrOrNil(in.Options.HostDNSForwardKubeDNSToHost.ValueOrZero() || in.Options.VersionContract.HostDNSForwardKubeDNSToHost()),
		}
	}

	if in.Options.VersionContract.AddExcludeFromExternalLoadBalancer() {
		if machine.MachineNodeLabels == nil {
			machine.MachineNodeLabels = map[string]string{}
		}

		machine.MachineNodeLabels[constants.LabelExcludeFromExternalLB] = ""
	}

	certSANs := in.GetAPIServerSANs()

	controlPlaneURL, err := url.Parse(in.ControlPlaneEndpoint)
	if err != nil {
		return nil, err
	}

	var admissionControlConfig []*v1alpha1.AdmissionPluginConfig

	if in.Options.VersionContract.PodSecurityAdmissionEnabled() && !in.Options.VersionContract.MultidocKubernetesConfigSupported() {
		admissionControlConfig = append(
			admissionControlConfig,
			&v1alpha1.AdmissionPluginConfig{
				PluginName:          "PodSecurity",
				PluginConfiguration: k8s.DefaultPodSecurityAdmissionControlConfig().PluginConfig,
			},
		)
	}

	var auditPolicyConfig meta.Unstructured

	if in.Options.VersionContract.APIServerAuditPolicySupported() && !in.Options.VersionContract.MultidocKubernetesConfigSupported() {
		auditPolicyConfig = v1alpha1.APIServerDefaultAuditPolicy
	}

	cluster := &v1alpha1.ClusterConfig{
		ClusterID:     in.Options.SecretsBundle.Cluster.ID,
		ClusterName:   in.ClusterName,
		ClusterSecret: in.Options.SecretsBundle.Cluster.Secret,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Endpoint:           &v1alpha1.Endpoint{URL: controlPlaneURL},
			LocalAPIServerPort: nilIf(in.Options.VersionContract.MultidocKubernetesConfigSupported(), in.Options.LocalAPIServerPort),
		},
		APIServerConfig: nilIf(in.Options.VersionContract.MultidocKubernetesConfigSupported(), &v1alpha1.APIServerConfig{
			ExtraCertSANs:          certSANs,
			ContainerImage:         fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, in.KubernetesVersion),
			AdmissionControlConfig: admissionControlConfig,
			AuditPolicyConfig:      auditPolicyConfig,
		}),
		ControllerManagerConfig: nilIf(in.Options.VersionContract.MultidocKubernetesConfigSupported(), &v1alpha1.ControllerManagerConfig{ //nolint:staticcheck // legacy configuration
			ContainerImage: fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, in.KubernetesVersion),
		}),
		ProxyConfig: nilIf(in.Options.VersionContract.MultidocKubernetesConfigSupported(), &v1alpha1.ProxyConfig{ //nolint:staticcheck // legacy configuration
			ContainerImage: fmt.Sprintf("%s:v%s", constants.KubeProxyImage, in.KubernetesVersion),
		}),
		SchedulerConfig: nilIf(in.Options.VersionContract.MultidocKubernetesConfigSupported(), &v1alpha1.SchedulerConfig{ //nolint:staticcheck // legacy configuration
			ContainerImage: fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, in.KubernetesVersion),
		}),
		EtcdConfig: &v1alpha1.EtcdConfig{
			RootCA: in.Options.SecretsBundle.Certs.Etcd,
		},
		ClusterNetwork: nilIf(
			in.Options.VersionContract.MultidocKubernetesConfigSupported(),
			&v1alpha1.ClusterNetworkConfig{
				DNSDomain:     in.Options.DNSDomain,
				PodSubnet:     in.PodNet,
				ServiceSubnet: in.ServiceNet,
			},
		),
		ClusterCA:              in.Options.SecretsBundle.Certs.K8s,
		ClusterAggregatorCA:    in.Options.SecretsBundle.Certs.K8sAggregator,
		ClusterServiceAccount:  in.Options.SecretsBundle.Certs.K8sServiceAccount,
		BootstrapToken:         in.Options.SecretsBundle.Secrets.BootstrapToken,
		ExtraManifests:         []string{},
		ClusterInlineManifests: v1alpha1.ClusterInlineManifests{},
	}

	if in.Options.CNICustomURL != "" {
		if !in.Options.VersionContract.MultidocKubernetesConfigSupported() {
			cluster.ClusterNetwork.CNI = &v1alpha1.CNIConfig{ //nolint:staticcheck // legacy configuration
				CNIName: constants.CustomCNI,
				CNIUrls: []string{in.Options.CNICustomURL},
			}
		} else {
			// we don't have extra manifests as multi-doc yet, so put it in the legacy field for now
			cluster.ExtraManifests = append(cluster.ExtraManifests, in.Options.CNICustomURL)
		}
	}

	if in.Options.AllowSchedulingOnControlPlanes {
		if in.Options.VersionContract.KubernetesAllowSchedulingOnControlPlanes() {
			cluster.AllowSchedulingOnControlPlanes = new(in.Options.AllowSchedulingOnControlPlanes)
		} else {
			// backwards compatibility for Talos versions older than 1.2
			cluster.AllowSchedulingOnMasters = new(in.Options.AllowSchedulingOnControlPlanes) //nolint:staticcheck
		}
	}

	if in.Options.DiscoveryEnabled != nil && !in.Options.VersionContract.DiscoveryServiceMultidocConfig() {
		cluster.ClusterDiscoveryConfig = &v1alpha1.ClusterDiscoveryConfig{ //nolint:staticcheck // legacy configuration
			DiscoveryEnabled: new(*in.Options.DiscoveryEnabled),
		}

		if in.Options.VersionContract.KubernetesDiscoveryBackendDisabled() {
			cluster.ClusterDiscoveryConfig.DiscoveryRegistries.RegistryKubernetes.RegistryDisabled = new(true) //nolint:staticcheck // legacy configuration
		}
	}

	if !in.Options.VersionContract.HideDisablePSP() && !in.Options.VersionContract.MultidocKubernetesConfigSupported() {
		cluster.APIServerConfig.DisablePodSecurityPolicyConfig = new(true) //nolint:staticcheck // legacy configuration
	}

	if !in.Options.VersionContract.MultidocKubernetesConfigSupported() {
		if in.Options.VersionContract.SecretboxEncryptionSupported() {
			cluster.ClusterSecretboxEncryptionSecret = in.Options.SecretsBundle.Secrets.SecretboxEncryptionSecret //nolint:staticcheck // legacy configuration
		} else {
			cluster.ClusterAESCBCEncryptionSecret = in.Options.SecretsBundle.Secrets.AESCBCEncryptionSecret //nolint:staticcheck // legacy configuration
		}
	}

	v1alpha1Config.MachineConfig = machine
	v1alpha1Config.ClusterConfig = cluster

	documents := []config.Document{v1alpha1Config}

	if pointer.SafeDeref(in.Options.DiscoveryEnabled) && in.Options.VersionContract.DiscoveryServiceMultidocConfig() {
		endpointURL, err := url.Parse(constants.DefaultDiscoveryServiceEndpoint)
		if err != nil {
			return nil, err
		}

		documents = append(documents, clustertypes.NewDiscoveryServiceConfigV1Alpha1("default", endpointURL))
	}

	if in.Options.VersionContract.HostDNSEnabled() && in.Options.VersionContract.HostDNSMultidocConfig() {
		resolverConfig := network.NewResolverConfigV1Alpha1()
		resolverConfig.ResolverHostDNS = network.HostDNSConfig{
			HostDNSEnabled:              new(true),
			HostDNSForwardKubeDNSToHost: ptrOrNil(in.Options.HostDNSForwardKubeDNSToHost.ValueOrZero() || in.Options.VersionContract.HostDNSForwardKubeDNSToHost()),
		}

		documents = append(documents, resolverConfig)
	}

	if len(in.Options.Sysctls) > 0 && in.Options.VersionContract.MultidocSysctlConfigSupported() {
		sysctlConfig := runtime.NewSysctlConfigV1Alpha1()
		sysctlConfig.Params = in.Options.Sysctls

		documents = append(documents, sysctlConfig)
	}

	documents = append(documents, in.generateBlockConfigs()...)

	extraDocuments, err := in.generateRegistryConfigs(machine)
	if err != nil {
		return nil, fmt.Errorf("failed to generate registry configs: %w", err)
	}

	documents = append(documents, extraDocuments...)

	extraDocuments, err = in.generateNetworkConfigs(machine)
	if err != nil {
		return nil, fmt.Errorf("failed to generate network configs: %w", err)
	}

	documents = append(documents, extraDocuments...)

	extraDocuments = in.generateKubernetesUniversalConfigs()

	documents = append(documents, extraDocuments...)

	extraDocuments = in.generateKubernetesControlplaneConfigs()

	documents = append(documents, extraDocuments...)

	return documents, nil
}

func ptrOrNil(b bool) *bool {
	if b {
		return &b
	}

	return nil
}

func nilIf[T any](condition bool, value T) T {
	if condition {
		var zero T

		return zero
	}

	return value
}
