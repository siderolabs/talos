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
	v1alpha1 "github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//nolint:gocyclo
func (in *Input) worker() ([]config.Document, error) {
	v1alpha1Config := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ConfigDebug:   pointer.To(in.Options.Debug),
		ConfigPersist: pointer.To(in.Options.Persist),
	}

	networkConfig := &v1alpha1.NetworkConfig{}

	for _, opt := range in.Options.NetworkConfigOptions {
		if err := opt(machine.TypeWorker, networkConfig); err != nil {
			return nil, err
		}
	}

	machine := &v1alpha1.MachineConfig{
		MachineType:     machine.TypeWorker.String(),
		MachineToken:    in.Options.SecretsBundle.TrustdInfo.Token,
		MachineCertSANs: in.AdditionalMachineCertSANs,
		MachineKubelet: &v1alpha1.KubeletConfig{
			KubeletImage: emptyIf(fmt.Sprintf("%s:v%s", constants.KubeletImage, in.KubernetesVersion), in.KubernetesVersion),
		},
		MachineNetwork: networkConfig,
		MachineCA:      &x509.PEMEncodedCertificateAndKey{Crt: in.Options.SecretsBundle.Certs.OS.Crt},
		MachineInstall: &v1alpha1.InstallConfig{
			InstallDisk:            in.Options.InstallDisk,
			InstallImage:           in.Options.InstallImage,
			InstallWipe:            pointer.To(false),
			InstallExtraKernelArgs: in.Options.InstallExtraKernelArgs,
		},
		MachineRegistries: v1alpha1.RegistriesConfig{
			RegistryMirrors: in.Options.RegistryMirrors,
			RegistryConfig:  in.Options.RegistryConfig,
		},
		MachineDisks:                in.Options.MachineDisks,
		MachineSystemDiskEncryption: in.Options.SystemDiskEncryptionConfig,
		MachineSysctls:              in.Options.Sysctls,
		MachineFeatures:             &v1alpha1.FeaturesConfig{},
	}

	if in.Options.VersionContract.SupportsRBACFeature() {
		machine.MachineFeatures.RBAC = pointer.To(true)
	}

	if in.Options.VersionContract.StableHostnameEnabled() {
		machine.MachineFeatures.StableHostname = pointer.To(true)
	}

	if in.Options.VersionContract.ApidExtKeyUsageCheckEnabled() {
		machine.MachineFeatures.ApidCheckExtKeyUsage = pointer.To(true)
	}

	if in.Options.VersionContract.DiskQuotaSupportEnabled() {
		machine.MachineFeatures.DiskQuotaSupport = pointer.To(true)
	}

	if kubePrismPort, optionSet := in.Options.KubePrismPort.Get(); optionSet { // default to enabled, but if set explicitly, allow it to be disabled
		if kubePrismPort > 0 {
			machine.MachineFeatures.KubePrismSupport = &v1alpha1.KubePrism{
				ServerEnabled: pointer.To(true),
				ServerPort:    kubePrismPort,
			}
		}
	} else if in.Options.VersionContract.KubePrismEnabled() {
		machine.MachineFeatures.KubePrismSupport = &v1alpha1.KubePrism{
			ServerEnabled: pointer.To(true),
			ServerPort:    constants.DefaultKubePrismPort,
		}
	}

	if in.Options.VersionContract.KubeletDefaultRuntimeSeccompProfileEnabled() {
		machine.MachineKubelet.KubeletDefaultRuntimeSeccompProfileEnabled = pointer.To(true)
	}

	if in.Options.VersionContract.KubeletManifestsDirectoryDisabled() {
		machine.MachineKubelet.KubeletDisableManifestsDirectory = pointer.To(true)
	}

	if in.Options.VersionContract.LocalDNSEnabled() {
		machine.MachineFeatures.HostDNSSupport = &v1alpha1.HostDNSConfig{
			HostDNSEnabled: pointer.To(true),
		}
	}

	controlPlaneURL, err := url.Parse(in.ControlPlaneEndpoint)
	if err != nil {
		return nil, err
	}

	cluster := &v1alpha1.ClusterConfig{
		ClusterID:      in.Options.SecretsBundle.Cluster.ID,
		ClusterSecret:  in.Options.SecretsBundle.Cluster.Secret,
		ClusterCA:      &x509.PEMEncodedCertificateAndKey{Crt: in.Options.SecretsBundle.Certs.K8s.Crt},
		BootstrapToken: in.Options.SecretsBundle.Secrets.BootstrapToken,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Endpoint: &v1alpha1.Endpoint{URL: controlPlaneURL},
		},
		ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
			DNSDomain:     in.Options.DNSDomain,
			PodSubnet:     in.PodNet,
			ServiceSubnet: in.ServiceNet,
			CNI:           in.Options.CNIConfig,
		},
	}

	if in.Options.DiscoveryEnabled != nil {
		cluster.ClusterDiscoveryConfig = &v1alpha1.ClusterDiscoveryConfig{
			DiscoveryEnabled: pointer.To(*in.Options.DiscoveryEnabled),
		}

		if in.Options.VersionContract.KubernetesDiscoveryBackendDisabled() {
			cluster.ClusterDiscoveryConfig.DiscoveryRegistries.RegistryKubernetes.RegistryDisabled = pointer.To(true)
		}
	}

	if machine.MachineRegistries.RegistryMirrors == nil {
		machine.MachineRegistries.RegistryMirrors = map[string]*v1alpha1.RegistryMirrorConfig{}
	}

	if in.Options.VersionContract.KubernetesAlternateImageRegistries() {
		if _, ok := machine.MachineRegistries.RegistryMirrors["k8s.gcr.io"]; !ok {
			machine.MachineRegistries.RegistryMirrors["k8s.gcr.io"] = &v1alpha1.RegistryMirrorConfig{
				MirrorEndpoints: []string{
					"https://registry.k8s.io",
					"https://k8s.gcr.io",
				},
			}
		}
	}

	v1alpha1Config.MachineConfig = machine
	v1alpha1Config.ClusterConfig = cluster

	return []config.Document{v1alpha1Config}, nil
}
