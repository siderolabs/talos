/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	v1alpha1 "github.com/talos-systems/talos/pkg/userdata/v1alpha1"
	yaml "gopkg.in/yaml.v2"
)

func workerUd(in *Input) (string, error) {
	machine := &v1alpha1.MachineConfig{
		Type:    "worker",
		Token:   in.TrustdInfo.Token,
		Kubelet: &v1alpha1.KubeletConfig{},
		Network: &v1alpha1.NetworkConfig{},
	}

	cluster := &v1alpha1.ClusterConfig{
		Token: in.KubeadmTokens.BootstrapToken,
		ControlPlane: &v1alpha1.ControlPlaneConfig{
			IPs: in.MasterIPs,
		},
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
