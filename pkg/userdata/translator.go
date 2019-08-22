/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	v1 "github.com/talos-systems/talos/pkg/userdata/v1"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeproxyconfig "k8s.io/kube-proxy/config/v1alpha1"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
	kubeadm "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
)

// TranslateV1 takes a v1 NodeConfig and translates it to a UserData struct
func TranslateV1(ncString string) (*UserData, error) {
	nc := &v1.NodeConfig{}

	err := yaml.Unmarshal([]byte(ncString), nc)
	if err != nil {
		return nil, err
	}

	// Lay down the absolute minimum for all node types
	ud := &UserData{
		Version:  "v1",
		Security: &Security{},
		Services: &Services{
			Init: &Init{
				CNI: "flannel",
			},
			Kubeadm: &Kubeadm{},
			Trustd: &Trustd{
				Token:     nc.Machine.Token,
				Endpoints: nc.Cluster.ControlPlane.IPs,
			},
		},
	}

	if nc.Machine.Install != nil {
		translateV1Install(nc, ud)
	}

	switch nc.Machine.Type {
	case "init":
		err = translateV1Init(nc, ud)
		if err != nil {
			return nil, err
		}

	case "controlplane":
		err = translateV1ControlPlane(nc, ud)
		if err != nil {
			return nil, err
		}

	case "worker":
		translateV1Worker(nc, ud)
	}
	return ud, nil
}

func translateV1Install(nc *v1.NodeConfig, ud *UserData) {

	ud.Install = &Install{
		Wipe:  nc.Machine.Install.Wipe,
		Force: nc.Machine.Install.Force,
	}

	if nc.Machine.Install.Boot != nil {
		ud.Install.Boot = &BootDevice{
			InstallDevice: InstallDevice{
				Device: nc.Machine.Install.Boot.InstallDevice.Device,
				Size:   nc.Machine.Install.Boot.InstallDevice.Size,
			},
			Kernel:    nc.Machine.Install.Boot.Kernel,
			Initramfs: nc.Machine.Install.Boot.Initramfs,
		}
	}

	if nc.Machine.Install.Ephemeral != nil {
		ud.Install.Ephemeral = &InstallDevice{
			Device: nc.Machine.Install.Ephemeral.Device,
			Size:   nc.Machine.Install.Ephemeral.Size,
		}
	}

	if nc.Machine.Install.ExtraDevices != nil {
		ud.Install.ExtraDevices = []*ExtraDevice{}
		for _, device := range nc.Machine.Install.ExtraDevices {
			ed := &ExtraDevice{
				Device:     device.Device,
				Partitions: []*ExtraDevicePartition{},
			}

			for _, partition := range device.Partitions {
				partToAppend := &ExtraDevicePartition{
					Size:       partition.Size,
					MountPoint: partition.MountPoint,
				}
				ed.Partitions = append(ed.Partitions, partToAppend)
			}
			ud.Install.ExtraDevices = append(ud.Install.ExtraDevices, ed)
		}
	}

	if nc.Machine.Install.ExtraKernelArgs != nil {
		ud.Install.ExtraKernelArgs = nc.Machine.Install.ExtraKernelArgs
	}
}

func translateV1Init(nc *v1.NodeConfig, ud *UserData) error {
	// Convert and decode certs back to byte slices
	osCert, err := base64.StdEncoding.DecodeString(nc.Machine.CA.Crt)
	if err != nil {
		return err
	}
	osKey, err := base64.StdEncoding.DecodeString(nc.Machine.CA.Key)
	if err != nil {
		return err
	}

	kubeCert, err := base64.StdEncoding.DecodeString(nc.Cluster.CA.Crt)
	if err != nil {
		return err
	}
	kubeKey, err := base64.StdEncoding.DecodeString(nc.Cluster.CA.Key)
	if err != nil {
		return err
	}

	// Inject certs and SANs
	ud.Security.OS = &OSSecurity{
		CA: &x509.PEMEncodedCertificateAndKey{
			Crt: osCert,
			Key: osKey,
		},
	}
	ud.Security.Kubernetes = &KubernetesSecurity{
		CA: &x509.PEMEncodedCertificateAndKey{
			Crt: kubeCert,
			Key: kubeKey,
		},
	}

	ud.Services.Trustd.CertSANs = []string{nc.Cluster.ControlPlane.IPs[nc.Cluster.ControlPlane.Index], "127.0.0.1", "::1"}

	ud.Services.Kubeadm.Token = nc.Cluster.InitToken
	ud.Services.Kubeadm.controlPlane = true

	kubeadmToken := strings.Split(nc.Cluster.Token, ".")

	// Craft an init kubeadm config
	initConfig := &kubeadm.InitConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "InitConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta2",
		},
		BootstrapTokens: []kubeadm.BootstrapToken{
			{
				Token: &kubeadm.BootstrapTokenString{
					ID:     kubeadmToken[0],
					Secret: kubeadmToken[1],
				},
				TTL: &metav1.Duration{
					Duration: time.Duration(0),
				},
			},
		},
		NodeRegistration: kubeadm.NodeRegistrationOptions{
			KubeletExtraArgs: nc.Machine.Kubelet.ExtraArgs,
		},
	}

	clusterConfig := &kubeadm.ClusterConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta2",
		},
		ClusterName:          nc.Cluster.ClusterName,
		KubernetesVersion:    constants.KubernetesVersion,
		ControlPlaneEndpoint: nc.Cluster.ControlPlane.IPs[0] + ":443",
		Networking: kubeadm.Networking{
			DNSDomain:     nc.Cluster.Network.DNSDomain,
			PodSubnet:     nc.Cluster.Network.PodSubnet[0],
			ServiceSubnet: nc.Cluster.Network.ServiceSubnet[0],
		},
		APIServer: kubeadm.APIServer{
			ControlPlaneComponent: kubeadm.ControlPlaneComponent{
				ExtraArgs: nc.Cluster.APIServer.ExtraArgs,
			},
			CertSANs: append(nc.Cluster.ControlPlane.IPs, "127.0.0.1", "::1"),
			TimeoutForControlPlane: &metav1.Duration{
				Duration: time.Duration(0),
			},
		},
		ControllerManager: kubeadm.ControlPlaneComponent{
			ExtraArgs: nc.Cluster.ControllerManager.ExtraArgs,
		},
		Scheduler: kubeadm.ControlPlaneComponent{
			ExtraArgs: nc.Cluster.Scheduler.ExtraArgs,
		},
	}

	kubeletConfig := &kubeletconfig.KubeletConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeletConfiguration",
			APIVersion: "kubelet.config.k8s.io/v1beta1",
		},
		FeatureGates: map[string]bool{
			"ExperimentalCriticalPodAnnotation": true,
		},
	}

	proxyConfig := &kubeproxyconfig.KubeProxyConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeProxyConfiguration",
			APIVersion: "kubeproxy.config.k8s.io/v1alpha1",
		},
		Mode: "ipvs",
		IPVS: kubeproxyconfig.KubeProxyIPVSConfiguration{
			Scheduler: "lc",
		},
	}

	ud.Services.Kubeadm.InitConfiguration = initConfig
	ud.Services.Kubeadm.ClusterConfiguration = clusterConfig
	ud.Services.Kubeadm.KubeletConfiguration = kubeletConfig
	ud.Services.Kubeadm.KubeProxyConfiguration = proxyConfig

	return nil
}

func translateV1ControlPlane(nc *v1.NodeConfig, ud *UserData) error {
	// Convert and decode certs back to byte slices
	osCert, err := base64.StdEncoding.DecodeString(nc.Machine.CA.Crt)
	if err != nil {
		return err
	}
	osKey, err := base64.StdEncoding.DecodeString(nc.Machine.CA.Key)
	if err != nil {
		return err
	}

	// Inject certs and SANs
	ud.Security.OS = &OSSecurity{
		CA: &x509.PEMEncodedCertificateAndKey{
			Crt: osCert,
			Key: osKey,
		},
	}
	ud.Services.Trustd.CertSANs = []string{nc.Cluster.ControlPlane.IPs[nc.Cluster.ControlPlane.Index], "127.0.0.1", "::1"}
	ud.Services.Kubeadm.controlPlane = true

	// Craft a control plane kubeadm config
	controlPlaneConfig := &kubeadm.JoinConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "JoinConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta2",
		},
		ControlPlane: &kubeadm.JoinControlPlane{},
		Discovery: kubeadm.Discovery{
			BootstrapToken: &kubeadm.BootstrapTokenDiscovery{
				Token:                    nc.Cluster.Token,
				APIServerEndpoint:        nc.Cluster.ControlPlane.IPs[nc.Cluster.ControlPlane.Index-1] + ":6443",
				UnsafeSkipCAVerification: true,
			},
		},
		NodeRegistration: kubeadm.NodeRegistrationOptions{
			KubeletExtraArgs: nc.Machine.Kubelet.ExtraArgs,
		},
	}

	ud.Services.Kubeadm.JoinConfiguration = controlPlaneConfig

	return nil
}

func translateV1Worker(nc *v1.NodeConfig, ud *UserData) {
	//Craft a worker kubeadm config
	workerConfig := &kubeadm.JoinConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "JoinConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta2",
		},
		Discovery: kubeadm.Discovery{
			BootstrapToken: &kubeadm.BootstrapTokenDiscovery{
				Token:                    nc.Cluster.Token,
				APIServerEndpoint:        nc.Cluster.ControlPlane.IPs[0] + ":443",
				UnsafeSkipCAVerification: true,
			},
		},
		NodeRegistration: kubeadm.NodeRegistrationOptions{
			KubeletExtraArgs: nc.Machine.Kubelet.ExtraArgs,
		},
	}

	ud.Services.Kubeadm.JoinConfiguration = workerConfig
}
