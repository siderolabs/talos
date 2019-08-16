/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	v1 "github.com/talos-systems/talos/pkg/userdata/v1"
	yaml "gopkg.in/yaml.v2"
)

func workerUd(in *Input) (string, error) {

	machine := &v1.MachineConfig{
		Type:    "worker",
		Token:   in.TrustdInfo.Token,
		Kubelet: &v1.KubeletConfig{},
		Network: &v1.NetworkConfig{},
	}

	cluster := &v1.ClusterConfig{
		ControlPlane: &v1.ControlPlaneConfig{
			IPs: in.MasterIPs,
		},
	}

	ud := v1.NodeConfig{
		Version: "v1",
		Machine: machine,
		Cluster: cluster,
	}

	udMarshal, err := yaml.Marshal(ud)
	if err != nil {
		return "", err
	}

	return string(udMarshal), nil
}
