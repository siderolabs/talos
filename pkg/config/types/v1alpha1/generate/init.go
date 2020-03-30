// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"net/url"

	v1alpha1 "github.com/talos-systems/talos/pkg/config/types/v1alpha1"
)

func initUd(in *Input2) (*v1alpha1.Config, error) {
	config := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		ConfigDebug:   in.Debug,
	}

	machine := &v1alpha1.MachineConfig{
		MachineType:     "init",
		MachineKubelet:  &v1alpha1.KubeletConfig{},
		MachineNetwork:  in.Node.NetworkConfig,
		MachineCA:       in.Cluster.Certs.OS,
		MachineCertSANs: in.Node.AdditionalMachineCertSANs,
		MachineToken:    in.Cluster.TrustdInfo.Token,
		MachineInstall: &v1alpha1.InstallConfig{
			InstallDisk:       in.Node.InstallDisk,
			InstallImage:      in.Node.InstallImage,
			InstallBootloader: true,
		},
		MachineRegistries: v1alpha1.RegistriesConfig{
			RegistryMirrors: in.Cluster.RegistryMirrors,
		},
	}

	certSANs := in.GetAPIServerSANs2()

	controlPlaneURL, err := url.Parse(in.Cluster.ControlPlaneEndpoint)
	if err != nil {
		return config, err
	}

	cluster := &v1alpha1.ClusterConfig{
		ClusterName: in.Cluster.ClusterName,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Endpoint: &v1alpha1.Endpoint{URL: controlPlaneURL},
		},
		APIServerConfig: &v1alpha1.APIServerConfig{
			CertSANs: certSANs,
		},
		ControllerManagerConfig: &v1alpha1.ControllerManagerConfig{},
		SchedulerConfig:         &v1alpha1.SchedulerConfig{},
		EtcdConfig: &v1alpha1.EtcdConfig{
			RootCA: in.Cluster.Certs.Etcd,
		},
		ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
			DNSDomain:     in.Cluster.ServiceDomain,
			PodSubnet:     in.Cluster.PodNet,
			ServiceSubnet: in.Cluster.ServiceNet,
		},
		ClusterCA:                     in.Cluster.Certs.K8s,
		BootstrapToken:                in.Cluster.Secrets.BootstrapToken,
		ClusterAESCBCEncryptionSecret: in.Cluster.Secrets.AESCBCEncryptionSecret,
	}

	config.MachineConfig = machine
	config.ClusterConfig = cluster

	return config, nil
}
