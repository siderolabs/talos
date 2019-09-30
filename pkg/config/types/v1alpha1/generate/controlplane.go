/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	yaml "gopkg.in/yaml.v2"

	v1alpha1 "github.com/talos-systems/talos/pkg/config/types/v1alpha1"
)

func controlPlaneUd(in *Input) (string, error) {
	machine := &v1alpha1.MachineConfig{
		MachineType:     "controlplane",
		MachineToken:    in.TrustdInfo.Token,
		MachineCA:       in.Certs.OS,
		MachineCertSANs: []string{"127.0.0.1", "::1"},
		MachineKubelet:  &v1alpha1.KubeletConfig{},
		MachineNetwork:  &v1alpha1.NetworkConfig{},
	}

	cluster := &v1alpha1.ClusterConfig{
		Token: in.KubeadmTokens.BootstrapToken,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			Version: in.KubernetesVersion,
			IPs:     in.MasterIPs,
		},
		EtcdConfig: &v1alpha1.EtcdConfig{
			RootCA: in.Certs.Etcd,
		},
		ClusterCA:                     in.Certs.K8s,
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
