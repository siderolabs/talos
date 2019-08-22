/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	v1 "github.com/talos-systems/talos/pkg/userdata/v1"
	yaml "gopkg.in/yaml.v2"
)

func initUd(in *Input) (string, error) {

	machine := &v1.MachineConfig{
		Type:    "init",
		Kubelet: &v1.KubeletConfig{},
		Network: &v1.NetworkConfig{},
		CA: &v1.MachineCAConfig{
			Crt: in.Certs.OsCert,
			Key: in.Certs.OsKey,
		},
		Token: in.TrustdInfo.Token,
	}

	certSANs := in.GetAPIServerSANs()

	cluster := &v1.ClusterConfig{
		ClusterName: in.ClusterName,
		ControlPlane: &v1.ControlPlaneConfig{
			IPs:   in.MasterIPs,
			Index: in.Index,
		},
		APIServer: &v1.APIServerConfig{
			CertSANs: certSANs,
		},
		ControllerManager: &v1.ControllerManagerConfig{},
		Scheduler:         &v1.SchedulerConfig{},
		Etcd:              &v1.EtcdConfig{},
		Network: &v1.ClusterNetworkConfig{
			DNSDomain:     in.ServiceDomain,
			PodSubnet:     in.PodNet,
			ServiceSubnet: in.ServiceNet,
		},
		CA: &v1.ClusterCAConfig{
			Crt: in.Certs.K8sCert,
			Key: in.Certs.K8sKey,
		},
		Token:     in.KubeadmTokens.BootstrapToken,
		InitToken: in.InitToken,
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
