/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package translate

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/userdata"
	v1 "github.com/talos-systems/talos/pkg/userdata/v1"
	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeproxyconfig "k8s.io/kube-proxy/config/v1alpha1"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
	kubeadm "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
)

// V1Translator holds info about a v1 machine config translation layer
type V1Translator struct {
	nodeConfig string
}

// Translate takes a v1 NodeConfig and translates it to a UserData struct
func (tv1 *V1Translator) Translate() (*userdata.UserData, error) {
	nc := &v1.NodeConfig{}

	err := yaml.Unmarshal([]byte(tv1.nodeConfig), nc)
	if err != nil {
		return nil, err
	}

	// Lay down the absolute minimum for all node types
	ud := &userdata.UserData{
		Version:  "v1",
		Security: &userdata.Security{},
		Services: &userdata.Services{
			Init: &userdata.Init{
				CNI: "flannel",
			},
			Kubeadm: &userdata.Kubeadm{},
			Trustd: &userdata.Trustd{
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

func translateV1Install(nc *v1.NodeConfig, ud *userdata.UserData) {

	ud.Install = &userdata.Install{
		Disk:       nc.Machine.Install.Disk,
		Image:      nc.Machine.Install.Image,
		Wipe:       nc.Machine.Install.Wipe,
		Force:      nc.Machine.Install.Force,
		Bootloader: nc.Machine.Install.Bootloader,
	}

	if nc.Machine.Install.ExtraDisks != nil {
		ud.Install.ExtraDevices = []*userdata.ExtraDevice{}
		for _, device := range nc.Machine.Install.ExtraDisks {
			ed := &userdata.ExtraDevice{
				Device:     device.Disk,
				Partitions: []*userdata.ExtraDevicePartition{},
			}

			for _, partition := range device.Partitions {
				partToAppend := &userdata.ExtraDevicePartition{
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

func translateV1Init(nc *v1.NodeConfig, ud *userdata.UserData) error {
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
	ud.Security.OS = &userdata.OSSecurity{
		CA: &x509.PEMEncodedCertificateAndKey{
			Crt: osCert,
			Key: osKey,
		},
	}
	ud.Security.Kubernetes = &userdata.KubernetesSecurity{
		CA: &x509.PEMEncodedCertificateAndKey{
			Crt: kubeCert,
			Key: kubeKey,
		},
	}

	ud.Services.Trustd.CertSANs = []string{nc.Cluster.ControlPlane.IPs[nc.Cluster.ControlPlane.Index], "127.0.0.1", "::1"}

	ud.Services.Kubeadm.Token = nc.Cluster.InitToken
	ud.Services.Kubeadm.ControlPlane = true

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
			CertSANs: nc.Cluster.APIServer.CertSANs,
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
		FeatureGates: map[string]bool{},
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

func translateV1ControlPlane(nc *v1.NodeConfig, ud *userdata.UserData) error {
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
	ud.Security.OS = &userdata.OSSecurity{
		CA: &x509.PEMEncodedCertificateAndKey{
			Crt: osCert,
			Key: osKey,
		},
	}
	ud.Services.Trustd.CertSANs = []string{nc.Cluster.ControlPlane.IPs[nc.Cluster.ControlPlane.Index], "127.0.0.1", "::1"}
	ud.Services.Kubeadm.ControlPlane = true

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

func translateV1Worker(nc *v1.NodeConfig, ud *userdata.UserData) {
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
