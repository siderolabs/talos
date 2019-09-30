/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package v1alpha1

import (
	"bytes"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	kubeproxyconfig "k8s.io/kube-proxy/config/v1alpha1"
	kubeletconfig "k8s.io/kubelet/config/v1beta1"
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadm "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
	kubeadmv1beta2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	kubeletconfigv1beta1scheme "k8s.io/kubernetes/pkg/kubelet/apis/config/v1beta1"
	kubeproxyconfigv1alpha1scheme "k8s.io/kubernetes/pkg/proxy/apis/config/v1alpha1"

	"github.com/talos-systems/talos/internal/pkg/cis"
	"github.com/talos-systems/talos/pkg/config/cluster"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// ClusterConfig reperesents the cluster-wide config values
type ClusterConfig struct {
	ControlPlane                  *ControlPlaneConfig               `yaml:"controlPlane"`
	ClusterName                   string                            `yaml:"clusterName,omitempty"`
	Network                       *ClusterNetworkConfig             `yaml:"network,omitempty"`
	Token                         string                            `yaml:"token,omitempty"`
	CertificateKey                string                            `yaml:"certificateKey"`
	ClusterAESCBCEncryptionSecret string                            `yaml:"aescbcEncryptionSecret"`
	ClusterCA                     *x509.PEMEncodedCertificateAndKey `yaml:"ca,omitempty"`
	APIServer                     *APIServerConfig                  `yaml:"apiServer,omitempty"`
	ControllerManager             *ControllerManagerConfig          `yaml:"controllerManager,omitempty"`
	Scheduler                     *SchedulerConfig                  `yaml:"scheduler,omitempty"`
	EtcdConfig                    *EtcdConfig                       `yaml:"etcd,omitempty"`
}

// ControlPlaneConfig represents control plane config vals
type ControlPlaneConfig struct {
	Version string `yaml:"version"`

	// Endpoint is the canonical controlplane endpoint, which can be an IP
	// address or a DNS hostname, is single-valued, and may optionally include a
	// port number.  It is optional and if not supplied, the IP address of the
	// first master node will be used.
	Endpoint string `yaml:"endpoint,omitempty"`

	IPs []string `yaml:"ips"`
}

// APIServerConfig represents kube apiserver config vals
type APIServerConfig struct {
	Image     string            `yaml:"image,omitempty"`
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
	CertSANs  []string          `yaml:"certSANs,omitempty"`
}

// ControllerManagerConfig represents kube controller manager config vals
type ControllerManagerConfig struct {
	Image     string            `yaml:"image,omitempty"`
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

// SchedulerConfig represents kube scheduler config vals
type SchedulerConfig struct {
	Image     string            `yaml:"image,omitempty"`
	ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

// EtcdConfig represents etcd config vals
type EtcdConfig struct {
	ContainerImage string                            `yaml:"image,omitempty"`
	RootCA         *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
}

// ClusterNetworkConfig represents kube networking config vals
type ClusterNetworkConfig struct {
	DNSDomain     string   `yaml:"dnsDomain"`
	PodSubnet     []string `yaml:"podSubnets"`
	ServiceSubnet []string `yaml:"serviceSubnets"`
}

// Version implements the Configurator interface.
func (c *ClusterConfig) Version() string {
	return c.ControlPlane.Version
}

// IPs implements the Configurator interface.
func (c *ClusterConfig) IPs() []string {
	return c.ControlPlane.IPs
}

// CertSANs implements the Configurator interface.
func (c *ClusterConfig) CertSANs() []string {
	return c.APIServer.CertSANs
}

// CA implements the Configurator interface.
func (c *ClusterConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return c.ClusterCA
}

// AESCBCEncryptionSecret implements the Configurator interface.
func (c *ClusterConfig) AESCBCEncryptionSecret() string {
	return c.ClusterAESCBCEncryptionSecret
}

// Config implements the Configurator interface.
func (c *ClusterConfig) Config(t machine.Type) (string, error) {
	switch t {
	case machine.Bootstrap:
		return c.initConfig()
	case machine.ControlPlane:
		return c.controlplaneConfig()
	case machine.Worker:
		return c.joinConfig()
	default:
		return "", errors.New("unknown machine type")
	}
}

// Etcd implements the Configurator interface.
func (c *ClusterConfig) Etcd() cluster.Etcd {
	return c.EtcdConfig
}

// Image implements the Configurator interface.
func (e *EtcdConfig) Image() string {
	return e.ContainerImage
}

// CA implements the Configurator interface.
func (e *EtcdConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return e.RootCA
}

// nolint: gocyclo
func (c *ClusterConfig) initConfig() (string, error) {
	token := strings.Split(c.Token, ".")

	initConfig := &kubeadm.InitConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "InitConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta2",
		},
		BootstrapTokens: []kubeadm.BootstrapToken{
			{
				Token: &kubeadm.BootstrapTokenString{
					ID:     token[0],
					Secret: token[1],
				},
				TTL: &metav1.Duration{
					Duration: time.Duration(0),
				},
			},
		},
		CertificateKey: c.CertificateKey,
		NodeRegistration: kubeadm.NodeRegistrationOptions{
			CRISocket: constants.ContainerdAddress,
			// KubeletExtraArgs: ,
		},
	}

	endpoint := c.ControlPlane.Endpoint
	if endpoint == "" {
		endpoint = c.ControlPlane.IPs[0]
	}
	clusterConfig := &kubeadm.ClusterConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta2",
		},
		ClusterName:          c.ClusterName,
		KubernetesVersion:    c.ControlPlane.Version,
		UseHyperKubeImage:    true,
		ControlPlaneEndpoint: endpoint + ":443",
		Networking: kubeadm.Networking{
			DNSDomain:     c.Network.DNSDomain,
			PodSubnet:     c.Network.PodSubnet[0],
			ServiceSubnet: c.Network.ServiceSubnet[0],
		},
		Etcd: kubeadmv1beta2.Etcd{
			External: &kubeadmv1beta2.ExternalEtcd{
				// TODO probably need to find a better way to handle obtaining etcd addrs
				// since this becomes an ordering issue. We rely on k8s to discover etcd
				// endpoints, but need etcd endpoints to bring up k8s.
				// We'll set this to 127.0.0.1 for now since mvp will be stacked control
				// plane ( etcd living on the same hosts as masters )
				Endpoints: []string{"https://127.0.0.1:" + strconv.Itoa(constants.KubeadmEtcdListenClientPort)},
				CAFile:    constants.KubeadmEtcdCACert,
				// These are for apiserver -> etcd communication
				CertFile: constants.KubeadmAPIServerEtcdClientCert,
				KeyFile:  constants.KubeadmAPIServerEtcdClientKey,
			},
		},
		APIServer: kubeadm.APIServer{
			ControlPlaneComponent: kubeadm.ControlPlaneComponent{
				ExtraArgs: c.APIServer.ExtraArgs,
			},
			CertSANs: c.APIServer.CertSANs,
			TimeoutForControlPlane: &metav1.Duration{
				Duration: time.Duration(0),
			},
		},
		ControllerManager: kubeadm.ControlPlaneComponent{
			ExtraArgs: c.ControllerManager.ExtraArgs,
		},
		Scheduler: kubeadm.ControlPlaneComponent{
			ExtraArgs: c.Scheduler.ExtraArgs,
		},
	}

	if err := cis.EnforceBootstrapMasterRequirements(clusterConfig); err != nil {
		return "", err
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

	// TODO(andrewrynhard): This should only be configured when the runtime mode
	// is container.
	f := false
	kubeletConfig.FailSwapOn = &f
	// See https://github.com/kubernetes/kubernetes/issues/58610#issuecomment-359552443
	maxPerCore := int32(0)
	proxyConfig.Conntrack.MaxPerCore = &maxPerCore

	if err := kubeletconfigv1beta1scheme.AddToScheme(kubeadmscheme.Scheme); err != nil {
		return "", err
	}

	if err := kubeproxyconfigv1alpha1scheme.AddToScheme(kubeadmscheme.Scheme); err != nil {
		return "", err
	}

	encodedObjs := [][]byte{}
	for _, obj := range []kuberuntime.Object{initConfig, clusterConfig} {
		encoded, err := kubeadmutil.MarshalToYamlForCodecs(obj, kubeadmv1beta2.SchemeGroupVersion, kubeadmscheme.Codecs)
		if err != nil {
			return "", err
		}
		encodedObjs = append(encodedObjs, encoded)
	}

	encoded, err := kubeadmutil.MarshalToYamlForCodecs(kubeletConfig, kubeletconfigv1beta1scheme.SchemeGroupVersion, kubeadmscheme.Codecs)
	if err != nil {
		return "", err
	}
	encodedObjs = append(encodedObjs, encoded)

	encoded, err = kubeadmutil.MarshalToYamlForCodecs(proxyConfig, kubeproxyconfigv1alpha1scheme.SchemeGroupVersion, kubeadmscheme.Codecs)
	if err != nil {
		return "", err
	}
	encodedObjs = append(encodedObjs, encoded)

	kubeadmConfig := bytes.Join(encodedObjs, []byte("---\n"))

	return string(kubeadmConfig), nil
}

func (c *ClusterConfig) controlplaneConfig() (string, error) {
	controlPlaneConfig := &kubeadm.JoinConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "JoinConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta2",
		},
		ControlPlane: &kubeadm.JoinControlPlane{
			CertificateKey: c.CertificateKey,
		},
		Discovery: kubeadm.Discovery{
			BootstrapToken: &kubeadm.BootstrapTokenDiscovery{
				Token:                    c.Token,
				APIServerEndpoint:        c.ControlPlane.IPs[0] + ":443",
				UnsafeSkipCAVerification: true,
			},
		},
		NodeRegistration: kubeadm.NodeRegistrationOptions{
			CRISocket: constants.ContainerdAddress,
			// KubeletExtraArgs: ,
		},
	}

	encodedObjs := [][]byte{}
	encoded, err := kubeadmutil.MarshalToYamlForCodecs(controlPlaneConfig, kubeadmv1beta2.SchemeGroupVersion, kubeadmscheme.Codecs)
	if err != nil {
		return "", err
	}
	encodedObjs = append(encodedObjs, encoded)

	kubeadmConfig := bytes.Join(encodedObjs, []byte("---\n"))

	return string(kubeadmConfig), nil
}

func (c *ClusterConfig) joinConfig() (string, error) {
	joinConfig := &kubeadm.JoinConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "JoinConfiguration",
			APIVersion: "kubeadm.k8s.io/v1beta2",
		},
		Discovery: kubeadm.Discovery{
			BootstrapToken: &kubeadm.BootstrapTokenDiscovery{
				Token:                    c.Token,
				APIServerEndpoint:        c.ControlPlane.IPs[0] + ":443",
				UnsafeSkipCAVerification: true,
			},
		},
		NodeRegistration: kubeadm.NodeRegistrationOptions{
			CRISocket: constants.ContainerdAddress,
			// KubeletExtraArgs: ,
		},
	}

	if err := cis.EnforceWorkerRequirements(joinConfig); err != nil {
		return "", err
	}

	encodedObjs := [][]byte{}
	encoded, err := kubeadmutil.MarshalToYamlForCodecs(joinConfig, kubeadmv1beta2.SchemeGroupVersion, kubeadmscheme.Codecs)
	if err != nil {
		return "", err
	}
	encodedObjs = append(encodedObjs, encoded)

	kubeadmConfig := bytes.Join(encodedObjs, []byte("---\n"))

	return string(kubeadmConfig), nil
}
