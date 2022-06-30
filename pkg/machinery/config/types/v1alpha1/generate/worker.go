// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"fmt"
	"net/url"

	"github.com/siderolabs/go-pointer"
	"github.com/talos-systems/crypto/x509"

	v1alpha1 "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

func workerUd(in *Input) (*v1alpha1.Config, error) {
	config := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ConfigDebug:   pointer.To(in.Debug),
		ConfigPersist: pointer.To(in.Persist),
	}

	networkConfig := &v1alpha1.NetworkConfig{}

	for _, opt := range in.NetworkConfigOptions {
		if err := opt(machine.TypeWorker, networkConfig); err != nil {
			return nil, err
		}
	}

	machine := &v1alpha1.MachineConfig{
		MachineType:     machine.TypeWorker.String(),
		MachineToken:    in.TrustdInfo.Token,
		MachineCertSANs: in.AdditionalMachineCertSANs,
		MachineKubelet: &v1alpha1.KubeletConfig{
			KubeletImage: emptyIf(fmt.Sprintf("%s:v%s", constants.KubeletImage, in.KubernetesVersion), in.KubernetesVersion),
		},
		MachineNetwork: networkConfig,
		MachineCA:      &x509.PEMEncodedCertificateAndKey{Crt: in.Certs.OS.Crt},
		MachineInstall: &v1alpha1.InstallConfig{
			InstallDisk:            in.InstallDisk,
			InstallImage:           in.InstallImage,
			InstallBootloader:      pointer.To(true),
			InstallWipe:            pointer.To(false),
			InstallExtraKernelArgs: in.InstallExtraKernelArgs,
		},
		MachineRegistries: v1alpha1.RegistriesConfig{
			RegistryMirrors: in.RegistryMirrors,
			RegistryConfig:  in.RegistryConfig,
		},
		MachineDisks:                in.MachineDisks,
		MachineSystemDiskEncryption: in.SystemDiskEncryptionConfig,
		MachineSysctls:              in.Sysctls,
		MachineFeatures:             &v1alpha1.FeaturesConfig{},
	}

	if in.VersionContract.SupportsRBACFeature() {
		machine.MachineFeatures.RBAC = pointer.To(true)
	}

	controlPlaneURL, err := url.Parse(in.ControlPlaneEndpoint)
	if err != nil {
		return config, err
	}

	cluster := &v1alpha1.ClusterConfig{
		ClusterID:      in.ClusterID,
		ClusterSecret:  in.ClusterSecret,
		ClusterCA:      &x509.PEMEncodedCertificateAndKey{Crt: in.Certs.K8s.Crt},
		BootstrapToken: in.Secrets.BootstrapToken,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Endpoint: &v1alpha1.Endpoint{URL: controlPlaneURL},
		},
		ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
			DNSDomain:     in.ServiceDomain,
			PodSubnet:     in.PodNet,
			ServiceSubnet: in.ServiceNet,
			CNI:           in.CNIConfig,
		},
	}

	if in.DiscoveryEnabled {
		cluster.ClusterDiscoveryConfig = &v1alpha1.ClusterDiscoveryConfig{
			DiscoveryEnabled: pointer.To(in.DiscoveryEnabled),
		}
	}

	config.MachineConfig = machine
	config.ClusterConfig = cluster

	return config, nil
}
