// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package constants

import (
	"time"

	"github.com/containerd/containerd/defaults"
	cni "github.com/containerd/go-cni"
)

const (
	// DefaultKernelVersion is the default Linux kernel version
	DefaultKernelVersion = "5.4.11-talos"

	// KernelParamConfig is the kernel parameter name for specifying the URL
	// to the config.
	KernelParamConfig = "talos.config"

	// KernelParamPlatform is the kernel parameter name for specifying the
	// platform.
	KernelParamPlatform = "talos.platform"

	// KernelParamHostname is the kernel parameter name for specifying the
	// hostname.
	KernelParamHostname = "talos.hostname"

	// KernelParamDefaultInterface is the kernel parameter for specifying the
	// initial interface used to bootstrap the node
	KernelParamDefaultInterface = "talos.interface"

	// KernelParamNetworkInterfaceIgnore is the kernel parameter for specifying network interfaces which should be ignored by talos
	KernelParamNetworkInterfaceIgnore = "talos.network.interface.ignore"

	// KernelCurrentRoot is the kernel parameter name for specifying the
	// current root partition.
	KernelCurrentRoot = "talos.root"

	// NewRoot is the path where the switchroot target is mounted.
	NewRoot = "/root"

	// BootPartitionLabel is the label of the partition to use for mounting at
	// the boot path.
	BootPartitionLabel = "ESP"

	// BootMountPoint is the label of the partition to use for mounting at
	// the boot path.
	BootMountPoint = "/boot"

	// EphemeralPartitionLabel is the label of the partition to use for
	// mounting at the data path.
	EphemeralPartitionLabel = "EPHEMERAL"

	// EphemeralMountPoint is the label of the partition to use for mounting at
	// the data path.
	EphemeralMountPoint = "/var"

	// RootMountPoint is the label of the partition to use for mounting at
	// the root path.
	RootMountPoint = "/"

	// ISOFilesystemLabel is the label of the ISO file system for the Talos
	// installer.
	ISOFilesystemLabel = "TALOS"

	// PATH defines all locations where executables are stored.
	PATH = "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin:" + cni.DefaultCNIDir

	// CNICalico is used to specify Calico CNI.
	CNICalico = "calico"

	// CNIFlannel is used to specify Flannel CNI.
	CNIFlannel = "flannel"

	// DefaultCertificatesDir is the path the the Kubernetes PKI directory.
	DefaultCertificatesDir = "/etc/kubernetes/pki"

	// KubernetesCACert is the path to the root CA certificate.
	KubernetesCACert = DefaultCertificatesDir + "/" + "ca.crt"

	// KubernetesCAKey is the path to the root CA private key.
	KubernetesCAKey = DefaultCertificatesDir + "/" + "ca.key"

	// KubernetesSACert is the path to the SA certificate.
	KubernetesSACert = DefaultCertificatesDir + "/" + "sa.crt"

	// KubernetesSAKey is the path to the SA private key.
	KubernetesSAKey = DefaultCertificatesDir + "/" + "sa.key"

	// KubernetesFrontProxyCACert is the path to the front proxy CA certificate.
	KubernetesFrontProxyCACert = DefaultCertificatesDir + "/" + "fp.crt"

	// KubernetesFrontProxyCAKey is the path to the front proxy CA private key.
	KubernetesFrontProxyCAKey = DefaultCertificatesDir + "/" + "fp.key"

	// KubernetesEtcdCACert is the path to the etcd CA certificate.
	KubernetesEtcdCACert = EtcdPKIPath + "/" + "ca.crt"

	// KubernetesEtcdCAKey is the path to the etcd CA private key.
	KubernetesEtcdCAKey = EtcdPKIPath + "/" + "ca.key"

	// KubernetesEtcdPeerCert is the path to the etcd CA certificate.
	KubernetesEtcdPeerCert = EtcdPKIPath + "/" + "peer.crt"

	// KubernetesEtcdPeerKey is the path to the etcd CA private key.
	KubernetesEtcdPeerKey = EtcdPKIPath + "/" + "peer.key"

	// KubernetesEtcdServerCert defines etcd's server certificate name
	KubernetesEtcdServerCert = EtcdPKIPath + "/" + "client.crt"

	// KubernetesEtcdServerKey defines etcd's server key name
	KubernetesEtcdServerKey = EtcdPKIPath + "/" + "client.key"

	// KubernetesEtcdListenClientPort defines the port etcd listen on for client traffic
	KubernetesEtcdListenClientPort = "2379"

	// KubernetesAPIServerEtcdClientCert defines apiserver's etcd client certificate name
	KubernetesAPIServerEtcdClientCert = DefaultCertificatesDir + "/" + "apiserver-etcd-client.crt"

	// KubernetesAPIServerEtcdClientKey defines apiserver's etcd client key name
	KubernetesAPIServerEtcdClientKey = DefaultCertificatesDir + "/" + "apiserver-etcd-client.key"

	// KubeletBootstrapKubeconfig is the path to the kubeconfig required to
	// bootstrap the kubelet.
	KubeletBootstrapKubeconfig = "/etc/kubernetes/bootstrap-kubeconfig"

	// DefaultKubernetesVersion is the default target version of the control plane.
	DefaultKubernetesVersion = "1.17.1"

	// KubernetesImage is the enforced hyperkube image to use for the control plane.
	KubernetesImage = "k8s.gcr.io/hyperkube"

	// PodCheckpointerImage is the enforced pod checkpointer for bootkube to use
	// TODO(andrewrynhard): This is a hack workaround for now. Update this once
	// there is an official image.
	PodCheckpointerImage = "docker.io/autonomy/pod-checkpointer:51fba9528e96d3be488562574c288b2fb82a1e3b"

	// LabelNodeRoleMaster is the node label required by a control plane node.
	LabelNodeRoleMaster = "node-role.kubernetes.io/master"

	// AssetsDirectory is the directory that contains all bootstrap assets.
	AssetsDirectory = "/etc/kubernetes/assets"

	// GeneratedKubeconfigAsset is the directory that contains bootstrap TLS assets.
	GeneratedKubeconfigAsset = AssetsDirectory + "/auth/kubeconfig"

	// AdminKubeconfig is the generated admin kubeconfig.
	AdminKubeconfig = "/etc/kubernetes/kubeconfig"

	// KubeletKubeconfig is the generated kubeconfig for kubelet.
	KubeletKubeconfig = "/etc/kubernetes/kubeconfig-kubelet"

	// DefaultEtcdVersion is the default target version of etcd.
	DefaultEtcdVersion = "3.3.15-0"

	// EtcdImage is the reposistory for the etcd image.
	EtcdImage = "k8s.gcr.io/etcd"

	// EtcdPKIPath is the path to the etcd PKI directory.
	EtcdPKIPath = DefaultCertificatesDir + "/etcd"

	// EtcdDataPath is the path where etcd stores its' data.
	EtcdDataPath = "/var/lib/etcd"

	// ConfigPath is the path to the downloaded config.
	ConfigPath = "/run/config.yaml"

	// MetalConfigISOLabel is the volume label for ISO based configuration.
	MetalConfigISOLabel = "metal-iso"

	// ConfigGuestInfo is the name of the VMware guestinfo config strategy.
	ConfigGuestInfo = "guestinfo"

	// VMwareGuestInfoConfigKey is the guestinfo key used to provide a config file.
	VMwareGuestInfoConfigKey = "talos.config"

	// AuditPolicyPath is the path to the audit-policy.yaml relative to initramfs.
	AuditPolicyPath = "/etc/kubernetes/audit-policy.yaml"

	// EncryptionConfigPath is the path to the EncryptionConfig relative to initramfs.
	EncryptionConfigPath = "/etc/kubernetes/encryptionconfig.yaml"

	// EncryptionConfigRootfsPath is the path to the EncryptionConfig relative to rootfs.
	EncryptionConfigRootfsPath = "/etc/kubernetes/encryptionconfig.yaml"

	// ApidPort is the port for the apid service.
	ApidPort = 50000

	// TrustdPort is the port for the trustd service.
	TrustdPort = 50001

	// DefaultContainerdVersion is the default container runtime version
	DefaultContainerdVersion = "1.3.3"

	// SystemContainerdNamespace is the Containerd namespace for Talos services.
	SystemContainerdNamespace = "system"

	// SystemContainerdAddress is the path to the system containerd socket.
	SystemContainerdAddress = SystemRunPath + "/containerd/containerd.sock"

	// CRIContainerdConfig is the path to the config for the containerd instance that provides the CRI.
	CRIContainerdConfig = "/etc/cri/containerd.toml"

	// TalosConfigEnvVar is the environment variable for setting the Talos configuration file path.
	TalosConfigEnvVar = "TALOSCONFIG"

	// MachineSocketPath is the path to file socket of machine API.
	MachineSocketPath = SystemRunPath + "/machined/machine.sock"

	// TimeSocketPath is the path to file socket of time API.
	TimeSocketPath = SystemRunPath + "/ntpd/ntpd.sock"

	// NetworkSocketPath is the path to file socket of network API.
	NetworkSocketPath = SystemRunPath + "/networkd/networkd.sock"

	// OSSocketPath is the path to file socket of os API.
	OSSocketPath = SystemRunPath + "/osd/osd.sock"

	// KernelUncompressedAsset defines a well known name for our uncompressed kernel filename
	KernelUncompressedAsset = "vmlinux"

	// KernelAsset defines a well known name for our kernel filename
	KernelAsset = "vmlinuz"

	// KernelAssetPath is the path to the kernel on disk.
	KernelAssetPath = "/usr/install/" + KernelAsset

	// InitramfsAsset defines a well known name for our initramfs filename
	InitramfsAsset = "initramfs.xz"

	// InitramfsAssetPath is the path to the initramfs on disk.
	InitramfsAssetPath = "/usr/install/" + InitramfsAsset

	// RootfsAsset defines a well known name for our rootfs filename
	RootfsAsset = "rootfs.sqsh"

	// DefaultCertificateValidityDuration is the default duration for a certificate.
	DefaultCertificateValidityDuration = 24 * time.Hour

	// SystemVarPath is the path to write runtime system related files and
	// directories.
	SystemVarPath = "/var/system"

	// SystemRunPath is the path to write temporary runtime system related files
	// and directories.
	SystemRunPath = "/run/system"

	// DefaultInstallerImageName is the default container image name for
	// the installer.
	DefaultInstallerImageName = "autonomy/installer"

	// DefaultInstallerImageRepository is the default container repository for
	// the installer.
	DefaultInstallerImageRepository = "docker.io/" + DefaultInstallerImageName

	// DefaultTalosImageRepository is the default container repository for
	// the talos image.
	DefaultTalosImageRepository = "docker.io/autonomy/talos"

	// DefaultLogPath is the default path to the log storage directory.
	DefaultLogPath = SystemRunPath + "/log"

	// DefaultCNI is the default CNI.
	DefaultCNI = "flannel"

	// CustomCNI is the string to use custom CNI.
	CustomCNI = "custom"

	// DefaultPodCIDR is the default pod CIDR block.
	DefaultPodCIDR = "10.244.0.0/16"

	// DefaultServiceCIDR is the default service CIDR block.
	DefaultServiceCIDR = "10.96.0.0/12"

	// InitializedKey is the key used to indicate if the cluster has been
	// initialized.
	InitializedKey = "initialized"

	// MetadataFile is the name of the file that contains a node's metadata.
	MetadataFile = "metadata.yaml"
)

// See https://linux.die.net/man/3/klogctl
//nolint: stylecheck
const (
	// SYSLOG_ACTION_SIZE_BUFFER is a named type argument to klogctl.
	// nolint: golint
	SYSLOG_ACTION_SIZE_BUFFER = 10

	// SYSLOG_ACTION_READ_ALL is a named type argument to klogctl.
	// nolint: golint
	SYSLOG_ACTION_READ_ALL = 3
)

// Containerd
const (
	ContainerdAddress = defaults.DefaultAddress
)
