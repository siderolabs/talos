/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	yaml "gopkg.in/yaml.v2"

	v1alpha1 "github.com/talos-systems/talos/pkg/config/types/v1alpha1"
)

func initUd(in *Input) (string, error) {
	machine := &v1alpha1.MachineConfig{
		MachineType:     "init",
		MachineKubelet:  &v1alpha1.KubeletConfig{},
		MachineNetwork:  &v1alpha1.NetworkConfig{},
		MachineCA:       in.Certs.OS,
		MachineCertSANs: []string{"127.0.0.1", "::1"},
		MachineToken:    in.TrustdInfo.Token,
	}

	certSANs := in.GetAPIServerSANs()

	cluster := &v1alpha1.ClusterConfig{
		ClusterName: in.ClusterName,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Version:  in.KubernetesVersion,
			Endpoint: in.ControlPlaneEndpoint,
			IPs:      in.MasterIPs,
		},
		APIServer: &v1alpha1.APIServerConfig{
			CertSANs: certSANs,
		},
		ControllerManager: &v1alpha1.ControllerManagerConfig{},
		Scheduler:         &v1alpha1.SchedulerConfig{},
		EtcdConfig: &v1alpha1.EtcdConfig{
			RootCA: in.Certs.Etcd,
		},
		ClusterNetwork: &v1alpha1.ClusterNetworkConfig{
			DNSDomain:     in.ServiceDomain,
			PodSubnet:     in.PodNet,
			ServiceSubnet: in.ServiceNet,
		},
		ClusterCA:                     in.Certs.K8s,
		BootstrapToken:                in.KubeadmTokens.BootstrapToken,
		CertificateKey:                in.KubeadmTokens.CertificateKey,
		ClusterAESCBCEncryptionSecret: in.KubeadmTokens.AESCBCEncryptionSecret,
	}

	ud := v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: machine,
		ClusterConfig: cluster,
	}

	udMarshal, err := yaml.Marshal(ud)
	if err != nil {
		return "", err
	}

	return string(udMarshal), nil
}
