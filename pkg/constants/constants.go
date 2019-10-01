/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package constants

import (
	"time"

	"github.com/containerd/containerd/defaults"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
)

const (
	// KernelParamUserData is the kernel parameter name for specifying the URL
	// to the user data.
	KernelParamUserData = "talos.userdata"

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
	PATH = "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin:/opt/cni/bin"

	// CNICalico is used to specify Calico CNI.
	CNICalico = "calico"

	// CNIFlannel is used to specify Flannel CNI.
	CNIFlannel = "flannel"

	// KubeadmConfig is the path to the kubeadm manifest file.
	KubeadmConfig = "/etc/kubernetes/kubeadm-config.yaml"

	// KubeadmCACert is the path to the root CA certificate.
	KubeadmCACert = v1beta2.DefaultCertificatesDir + "/" + constants.CACertName

	// KubeadmCAKey is the path to the root CA private key.
	KubeadmCAKey = v1beta2.DefaultCertificatesDir + "/" + constants.CAKeyName

	// KubeadmSACert is the path to the SA certificate.
	KubeadmSACert = v1beta2.DefaultCertificatesDir + "/" + constants.ServiceAccountPublicKeyName

	// KubeadmSAKey is the path to the SA private key.
	KubeadmSAKey = v1beta2.DefaultCertificatesDir + "/" + constants.ServiceAccountPrivateKeyName

	// KubeadmFrontProxyCACert is the path to the front proxy CA certificate.
	KubeadmFrontProxyCACert = v1beta2.DefaultCertificatesDir + "/" + constants.FrontProxyCACertName

	// KubeadmFrontProxyCAKey is the path to the front proxy CA private key.
	KubeadmFrontProxyCAKey = v1beta2.DefaultCertificatesDir + "/" + constants.FrontProxyCAKeyName

	// KubeadmEtcdCACert is the path to the etcd CA certificate.
	KubeadmEtcdCACert = v1beta2.DefaultCertificatesDir + "/" + constants.EtcdCACertName

	// KubeadmEtcdCAKey is the path to the etcd CA private key.
	KubeadmEtcdCAKey = v1beta2.DefaultCertificatesDir + "/" + constants.EtcdCAKeyName

	// KubeadmEtcdPeerCert is the path to the etcd CA certificate.
	KubeadmEtcdPeerCert = v1beta2.DefaultCertificatesDir + "/" + constants.EtcdPeerCertName

	// KubeadmEtcdPeerKey is the path to the etcd CA private key.
	KubeadmEtcdPeerKey = v1beta2.DefaultCertificatesDir + "/" + constants.EtcdPeerKeyName

	// KubeadmEtcdServerCert defines etcd's server certificate name
	KubeadmEtcdServerCert = v1beta2.DefaultCertificatesDir + "/" + constants.EtcdServerCertName

	// KubeadmEtcdServerKey defines etcd's server key name
	KubeadmEtcdServerKey = v1beta2.DefaultCertificatesDir + "/" + constants.EtcdServerKeyName

	// KubeadmEtcdListenClientPort defines the port etcd listen on for client traffic
	KubeadmEtcdListenClientPort = constants.EtcdListenClientPort

	// KubeadmAPIServerEtcdClientCert defines apiserver's etcd client certificate name
	KubeadmAPIServerEtcdClientCert = v1beta2.DefaultCertificatesDir + "/" + constants.APIServerEtcdClientCertName

	// KubeadmAPIServerEtcdClientKey defines apiserver's etcd client key name
	KubeadmAPIServerEtcdClientKey = v1beta2.DefaultCertificatesDir + "/" + constants.APIServerEtcdClientKeyName

	// DefaultKubernetesVersion is the default target version of the control plane.
	DefaultKubernetesVersion = "1.16.0"

	// KubernetesImage is the enforced hyperkube image to use for the control plane.
	KubernetesImage = "k8s.gcr.io/hyperkube"

	// UserDataPath is the path to the downloaded user data.
	UserDataPath = "/run/userdata.yaml"

	// UserDataCIData is the volume label for NoCloud cloud-init.
	// See https://cloudinit.readthedocs.io/en/latest/topics/datasources/nocloud.html#datasource-nocloud.
	UserDataCIData = "cidata"

	// UserDataGuestInfo is the name of the VMware guestinfo user data strategy.
	UserDataGuestInfo = "guestinfo"

	// VMwareGuestInfoUserDataKey is the guestinfo key used to provide a user data file.
	VMwareGuestInfoUserDataKey = "talos.userdata"

	// AuditPolicyPath is the path to the audit-policy.yaml relative to initramfs.
	AuditPolicyPath = "/etc/kubernetes/audit-policy.yaml"

	// EncryptionConfigPath is the path to the EncryptionConfig relative to initramfs.
	EncryptionConfigPath = "/etc/kubernetes/encryptionconfig.yaml"

	// EncryptionConfigRootfsPath is the path to the EncryptionConfig relative to rootfs.
	EncryptionConfigRootfsPath = "/etc/kubernetes/encryptionconfig.yaml"

	// OsdPort is the port for the osd service.
	OsdPort = 50000

	// TrustdPort is the port for the trustd service.
	TrustdPort = 50001

	// SystemContainerdNamespace is the Containerd namespace for Talos services.
	SystemContainerdNamespace = "system"

	// SystemContainerdAddress is the path to the system containerd socket.
	SystemContainerdAddress = SystemRunPath + "/containerd/containerd.sock"

	// TalosConfigEnvVar is the environment variable for setting the Talos configuration file path.
	TalosConfigEnvVar = "TALOSCONFIG"

	// InitSocketPath is the path to file socket of init API
	InitSocketPath = SystemRunPath + "/init/init.sock"

	// ProxydSocketPath is the path to file socket of proxyd API
	ProxydSocketPath = SystemRunPath + "/proxyd/proxyd.sock"

	// NtpdSocketPath is the path to file socket of proxyd API
	NtpdSocketPath = SystemRunPath + "/ntpd/ntpd.sock"

	// NetworkdSocketPath is the path to file socket of proxyd API
	NetworkdSocketPath = SystemRunPath + "/networkd/networkd.sock"

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

	// DefaultInstallerImageRepository is the default container repository for
	// the installer.
	DefaultInstallerImageRepository = "docker.io/autonomy/installer"

	// DefaultLogPath is the default path to the log storage directory.
	DefaultLogPath = SystemRunPath + "/log"
)

// See https://linux.die.net/man/3/klogctl
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
