/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	yaml "gopkg.in/yaml.v2"

	v1alpha1 "github.com/talos-systems/talos/pkg/userdata/v1alpha1"
)

func initUd(in *Input) (string, error) {
	machine := &v1alpha1.MachineConfig{
		Type:    "init",
		Kubelet: &v1alpha1.KubeletConfig{},
		Network: &v1alpha1.NetworkConfig{},
		CA: &v1alpha1.MachineCAConfig{
			Crt: in.Certs.OsCert,
			Key: in.Certs.OsKey,
		},
		Token: in.TrustdInfo.Token,
	}

	certSANs := in.GetAPIServerSANs()

	cluster := &v1alpha1.ClusterConfig{
		ClusterName: in.ClusterName,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Endpoint: in.ControlPlaneEndpoint,
			IPs:      in.MasterIPs,
			Index:    in.Index,
		},
		APIServer: &v1alpha1.APIServerConfig{
			CertSANs: certSANs,
		},
		ControllerManager: &v1alpha1.ControllerManagerConfig{},
		Scheduler:         &v1alpha1.SchedulerConfig{},
		Etcd:              &v1alpha1.EtcdConfig{},
		Network: &v1alpha1.ClusterNetworkConfig{
			DNSDomain:     in.ServiceDomain,
			PodSubnet:     in.PodNet,
			ServiceSubnet: in.ServiceNet,
		},
		CA: &v1alpha1.ClusterCAConfig{
			Crt: in.Certs.K8sCert,
			Key: in.Certs.K8sKey,
		},
		Token:                  in.KubeadmTokens.BootstrapToken,
		CertificateKey:         in.KubeadmTokens.CertificateKey,
		AESCBCEncryptionSecret: in.KubeadmTokens.AESCBCEncryptionSecret,
	}

	ud := v1alpha1.NodeConfig{
		Version: "v1alpha1",
		Machine: machine,
		Cluster: cluster,
	}

	udMarshal, err := yaml.Marshal(ud)
	if err != nil {
		return "", err
	}

	return string(udMarshal), nil
}
