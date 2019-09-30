/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	yaml "gopkg.in/yaml.v2"

	v1alpha1 "github.com/talos-systems/talos/pkg/userdata/v1alpha1"
)

func controlPlaneUd(in *Input) (string, error) {
	machine := &v1alpha1.MachineConfig{
		Type:  "controlplane",
		Token: in.TrustdInfo.Token,
		CA: &v1alpha1.MachineCAConfig{
			Crt: in.Certs.OsCert,
			Key: in.Certs.OsKey,
		},
		Kubelet: &v1alpha1.KubeletConfig{},
		Network: &v1alpha1.NetworkConfig{},
	}

	cluster := &v1alpha1.ClusterConfig{
		Token: in.KubeadmTokens.BootstrapToken,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Version: in.KubernetesVersion,
			IPs:     in.MasterIPs,
			Index:   in.Index,
		},
		Etcd: &v1alpha1.EtcdConfig{
			Enabled: true,
			CA: &v1alpha1.EtcdCAConfig{
				Crt: in.Certs.EtcdCert,
				Key: in.Certs.EtcdKey,
			},
		},
		CA: &v1alpha1.ClusterCAConfig{
			Crt: in.Certs.K8sCert,
			Key: in.Certs.K8sKey,
		},
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
