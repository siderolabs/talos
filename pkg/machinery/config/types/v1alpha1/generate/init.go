// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"fmt"
	"net/url"

	v1alpha1 "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

func initUd(in *Input) (*v1alpha1.Config, error) {
	config := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ConfigDebug:   in.Debug,
		ConfigPersist: in.Persist,
	}

	networkConfig := &v1alpha1.NetworkConfig{}

	for _, opt := range in.NetworkConfigOptions {
		if err := opt(machine.TypeControlPlane, networkConfig); err != nil {
			return nil, err
		}
	}

	machine := &v1alpha1.MachineConfig{
		MachineType: "init",
		MachineKubelet: &v1alpha1.KubeletConfig{
			KubeletImage: emptyIf(fmt.Sprintf("%s:v%s", constants.KubeletImage, in.KubernetesVersion), in.KubernetesVersion),
		},
		MachineNetwork:  networkConfig,
		MachineCA:       in.Certs.OS,
		MachineCertSANs: in.AdditionalMachineCertSANs,
		MachineToken:    in.TrustdInfo.Token,
		MachineInstall: &v1alpha1.InstallConfig{
			InstallDisk:            in.InstallDisk,
			InstallImage:           in.InstallImage,
			InstallBootloader:      true,
			InstallExtraKernelArgs: in.InstallExtraKernelArgs,
		},
		MachineRegistries: v1alpha1.RegistriesConfig{
			RegistryMirrors: in.RegistryMirrors,
			RegistryConfig:  in.RegistryConfig,
		},
		MachineDisks:                in.MachineDisks,
		MachineSystemDiskEncryption: in.SystemDiskEncryptionConfig,
	}

	certSANs := in.GetAPIServerSANs()

	controlPlaneURL, err := url.Parse(in.ControlPlaneEndpoint)
	if err != nil {
		return config, err
	}

	cluster := &v1alpha1.ClusterConfig{
		ClusterName: in.ClusterName,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Endpoint: &v1alpha1.Endpoint{URL: controlPlaneURL},
		},
		APIServerConfig: &v1alpha1.APIServerConfig{
			CertSANs:       certSANs,
			ContainerImage: emptyIf(fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, in.KubernetesVersion), in.KubernetesVersion),
		},
		ControllerManagerConfig: &v1alpha1.ControllerManagerConfig{
			ContainerImage: emptyIf(fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, in.KubernetesVersion), in.KubernetesVersion),
		},
		ProxyConfig: &v1alpha1.ProxyConfig{
			ContainerImage: emptyIf(fmt.Sprintf("%s:v%s", constants.KubeProxyImage, in.KubernetesVersion), in.KubernetesVersion),
		},
		SchedulerConfig: &v1alpha1.SchedulerConfig{
			ContainerImage: emptyIf(fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, in.KubernetesVersion), in.KubernetesVersion),
		},
		EtcdConfig: &v1alpha1.EtcdConfig{
			RootCA: in.Certs.Etcd,
		},
		ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
			DNSDomain:     in.ServiceDomain,
			PodSubnet:     in.PodNet,
			ServiceSubnet: in.ServiceNet,
			CNI:           in.CNIConfig,
		},
		ClusterCA:                     in.Certs.K8s,
		ClusterAggregatorCA:           in.Certs.K8sAggregator,
		ClusterServiceAccount:         in.Certs.K8sServiceAccount,
		BootstrapToken:                in.Secrets.BootstrapToken,
		ClusterAESCBCEncryptionSecret: in.Secrets.AESCBCEncryptionSecret,
		ExtraManifests:                []string{},
		ClusterInlineManifests:        v1alpha1.ClusterInlineManifests{},
	}

	config.MachineConfig = machine
	config.ClusterConfig = cluster

	return config, nil
}
