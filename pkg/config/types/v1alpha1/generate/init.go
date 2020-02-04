// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package generate

import (
	"net/url"

	"github.com/talos-systems/talos/pkg/config/machine"
	v1alpha1 "github.com/talos-systems/talos/pkg/config/types/v1alpha1"
)

func initUd(in *Input) (*v1alpha1.Config, error) {
	config := &v1alpha1.Config{ConfigVersion: "v1alpha1"}

	machine := &v1alpha1.MachineConfig{
		MachineType:     "init",
		MachineKubelet:  &v1alpha1.KubeletConfig{},
		MachineNetwork:  in.NetworkConfig,
		MachineCA:       in.Certs.OS,
		MachineCertSANs: in.AdditionalMachineCertSANs,
		MachineToken:    in.TrustdInfo.Token,
		MachineInstall: &v1alpha1.InstallConfig{
			InstallDisk:       in.InstallDisk,
			InstallImage:      in.InstallImage,
			InstallBootloader: true,
		},
		MachineFiles: []machine.File{
			{
				Path:        "/etc/cri/containerd.toml",
				Permissions: 0644,
				Op:          "append",
				Content: `
				[plugins.cri.registry.mirrors]
				  [plugins.cri.registry.mirrors."docker.io"]
					endpoint = ["http://172.20.0.1:5000"]
  				  [plugins.cri.registry.mirrors."k8s.gcr.io"]
					endpoint = ["http://172.20.0.1:5001"]
  				  [plugins.cri.registry.mirrors."quay.io"]
					endpoint = ["http://172.20.0.1:5002"]
					`,
			},
		},
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
			CertSANs: certSANs,
		},
		ControllerManagerConfig: &v1alpha1.ControllerManagerConfig{},
		SchedulerConfig:         &v1alpha1.SchedulerConfig{},
		EtcdConfig: &v1alpha1.EtcdConfig{
			RootCA: in.Certs.Etcd,
		},
		ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
			DNSDomain:     in.ServiceDomain,
			PodSubnet:     in.PodNet,
			ServiceSubnet: in.ServiceNet,
		},
		ClusterCA:                     in.Certs.K8s,
		BootstrapToken:                in.Secrets.BootstrapToken,
		ClusterAESCBCEncryptionSecret: in.Secrets.AESCBCEncryptionSecret,
	}

	config.MachineConfig = machine
	config.ClusterConfig = cluster

	return config, nil
}
