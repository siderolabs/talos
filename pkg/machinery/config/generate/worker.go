// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"fmt"
	"net/url"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	clustertypes "github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//nolint:gocyclo,cyclop
func (in *Input) worker() ([]config.Document, error) {
	v1alpha1Config := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ConfigDebug:   new(in.Options.Debug),
		ConfigPersist: new(true),
	}

	machine := &v1alpha1.MachineConfig{
		MachineType:     machine.TypeWorker.String(),
		MachineToken:    in.Options.SecretsBundle.TrustdInfo.Token,
		MachineCertSANs: in.AdditionalMachineCertSANs,
		MachineKubelet: &v1alpha1.KubeletConfig{
			KubeletImage: fmt.Sprintf("%s:v%s", constants.KubeletImage, in.KubernetesVersion),
		},
		MachineCA:       &x509.PEMEncodedCertificateAndKey{Crt: in.Options.SecretsBundle.Certs.OS.Crt},
		MachineDisks:    in.Options.MachineDisks,
		MachineFeatures: &v1alpha1.FeaturesConfig{},
	}

	if !in.Options.VersionContract.MultidocSysctlConfigSupported() {
		machine.MachineSysctls = in.Options.Sysctls //nolint:staticcheck // legacy configuration
	}

	// .machine.install is deprecated in favor of the UnattendedInstallConfig multi-document config;
	// only generate it for older version contracts that don't support the new document.
	if !in.Options.VersionContract.UnattendedInstallConfig() {
		machine.MachineInstall = &v1alpha1.InstallConfig{ //nolint:staticcheck // legacy configuration
			InstallDisk:            in.Options.InstallDisk,
			InstallImage:           in.Options.InstallImage,
			InstallWipe:            new(false),
			InstallExtraKernelArgs: in.Options.InstallExtraKernelArgs,
		}

		if in.Options.VersionContract.GrubUseUKICmdlineDefault() {
			machine.MachineInstall.InstallGrubUseUKICmdline = new(true) //nolint:staticcheck // legacy configuration
		}
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
		machine.MachineFeatures.HostDNSSupport = &v1alpha1.HostDNSConfig{ //nolint:staticcheck // legacy config
			HostDNSConfigEnabled:        new(true),
			HostDNSForwardKubeDNSToHost: ptrOrNil(in.Options.HostDNSForwardKubeDNSToHost.ValueOrZero() || in.Options.VersionContract.HostDNSForwardKubeDNSToHost()),
		}
	}

	controlPlaneURL, err := url.Parse(in.ControlPlaneEndpoint)
	if err != nil {
		return nil, err
	}

	cluster := &v1alpha1.ClusterConfig{
		ClusterID:      nilIf(in.Options.VersionContract.DiscoveryIdentityMultidocConfig(), in.Options.SecretsBundle.Cluster.ID),     //nolint:staticcheck // legacy configuration
		ClusterSecret:  nilIf(in.Options.VersionContract.DiscoveryIdentityMultidocConfig(), in.Options.SecretsBundle.Cluster.Secret), //nolint:staticcheck // legacy configuration
		ClusterCA:      nilIf(in.Options.VersionContract.MultidocKubernetesConfigSupported(), &x509.PEMEncodedCertificateAndKey{Crt: in.Options.SecretsBundle.Certs.K8s.Crt}),
		BootstrapToken: in.Options.SecretsBundle.Secrets.BootstrapToken,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Endpoint: &v1alpha1.Endpoint{URL: controlPlaneURL},
		},
		ClusterNetwork: nilIf(
			in.Options.VersionContract.MultidocKubernetesConfigSupported(),
			&v1alpha1.ClusterNetworkConfig{
				DNSDomain:     in.Options.DNSDomain,
				PodSubnet:     in.PodNet,
				ServiceSubnet: in.ServiceNet,
			},
		),
	}

	if !in.Options.VersionContract.MultidocKubernetesConfigSupported() && in.Options.CNICustomURL != "" {
		cluster.ClusterNetwork.CNI = &v1alpha1.CNIConfig{ //nolint:staticcheck // legacy configuration
			CNIName: constants.CustomCNI,
			CNIUrls: []string{in.Options.CNICustomURL},
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

	if machine.MachineRegistries.RegistryMirrors == nil { //nolint:staticcheck // backwards compatibility
		machine.MachineRegistries.RegistryMirrors = map[string]*v1alpha1.RegistryMirrorConfig{} //nolint:staticcheck // backwards compatibility
	}

	if in.Options.VersionContract.KubernetesAlternateImageRegistries() {
		if _, ok := machine.MachineRegistries.RegistryMirrors["k8s.gcr.io"]; !ok { //nolint:staticcheck // backwards compatibility Talos v1.1->1.2
			machine.MachineRegistries.RegistryMirrors["k8s.gcr.io"] = &v1alpha1.RegistryMirrorConfig{ //nolint:staticcheck // backwards compatibility Talos v1.1->1.2
				MirrorEndpoints: []string{
					"https://registry.k8s.io",
					"https://k8s.gcr.io",
				},
			}
		}
	}

	if in.Options.VersionContract.ClusterNameForWorkers() {
		cluster.ClusterName = in.ClusterName
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

	if in.Options.VersionContract.DiscoveryIdentityMultidocConfig() {
		documents = append(documents, clustertypes.NewDiscoveryIdentityConfigV1Alpha1(
			in.Options.SecretsBundle.Cluster.ID,
			in.Options.SecretsBundle.Cluster.Secret,
		))
	}

	// The UnattendedInstallConfig document requires a volume selector, which is derived from the install disk,
	// so only generate it when an install disk is provided.
	if in.Options.VersionContract.UnattendedInstallConfig() && in.Options.InstallDisk != "" && !in.Options.SkipUnattendedInstallConfig {
		unattended, err := in.unattendedInstallConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to generate unattended install config: %w", err)
		}

		documents = append(documents, unattended)
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

	documents = append(documents, in.generateSecurityProfileConfigs()...)

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

	extraDocuments = in.generateKubernetesUniversalConfigs(false)

	documents = append(documents, extraDocuments...)

	return documents, nil
}
