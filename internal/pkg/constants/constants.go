/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package constants

const (
	// KernelParamUserData is the kernel parameter name for specifying the URL
	// to the user data.
	KernelParamUserData = "talos.userdata"

	// KernelParamPlatform is the kernel parameter name for specifying the
	// platform.
	KernelParamPlatform = "talos.platform"

	// NewRoot is the path where the switchroot target is mounted.
	NewRoot = "/root"

	// BootPartitionLabel is the label of the partition to use for mounting at
	// the boot path.
	BootPartitionLabel = "ESP"

	// BootMountPoint is the label of the partition to use for mounting at
	// the boot path.
	BootMountPoint = "/boot"

	// DataPartitionLabel is the label of the partition to use for mounting at
	// the data path.
	DataPartitionLabel = "DATA"

	// DataMountPoint is the label of the partition to use for mounting at
	// the data path.
	DataMountPoint = "/var"

	// RootPartitionLabel is the label of the partition to use for mounting at
	// the root path.
	RootPartitionLabel = "ROOT"

	// RootMountPoint is the label of the partition to use for mounting at
	// the root path.
	RootMountPoint = "/"

	// PATH defines all locations where executables are stored.
	PATH = "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin:/opt/cni/bin"

	// CNICalico is used to specify Calico CNI.
	CNICalico = "calico"

	// CNIFlannel is used to specify Flannel CNI.
	CNIFlannel = "flannel"

	// KubeadmConfig is the path to the kubeadm manifest file.
	KubeadmConfig = "/etc/kubernetes/kubeadm-config.yaml"

	// KubeadmCACert is the path to the root CA certificate.
	KubeadmCACert = "/etc/kubernetes/pki/ca.crt"

	// KubeadmCAKey is the path to the root CA private key.
	KubeadmCAKey = "/etc/kubernetes/pki/ca.key"

	// KubernetesVersion is the enforced target version of the control plane.
	KubernetesVersion = "v1.14.0"

	// KubernetesImage is the enforced hyperkube image to use for the control plane.
	KubernetesImage = "k8s.gcr.io/hyperkube:" + KubernetesVersion

	// UserDataPath is the path to the downloaded user data.
	UserDataPath = "/run/userdata.yaml"

	// UserDataCIData is the volume label for NoCloud cloud-init.
	// See https://cloudinit.readthedocs.io/en/latest/topics/datasources/nocloud.html#datasource-nocloud.
	UserDataCIData = "cidata"

	// UserDataGuestInfo is the name of the VMware guestinfo user data strategy.
	UserDataGuestInfo = "guestinfo"

	// VMwareGuestInfoUserDataKey is the guestinfo key used to provide a user data file.
	VMwareGuestInfoUserDataKey = "talos.userdata"

	// AuditPolicyPathInitramfs is the path to the audit-policy.yaml relative to initramfs.
	AuditPolicyPathInitramfs = "/etc/kubernetes/audit-policy.yaml"

	// EncryptionConfigInitramfsPath is the path to the EncryptionConfig relative to initramfs.
	EncryptionConfigInitramfsPath = "/etc/kubernetes/encryptionconfig.yaml"

	// EncryptionConfigRootfsPath is the path to the EncryptionConfig relative to rootfs.
	EncryptionConfigRootfsPath = "/etc/kubernetes/encryptionconfig.yaml"

	// OsdPort is the port for the osd service.
	OsdPort = 50000

	// TrustdPort is the port for the trustd service.
	TrustdPort = 50001

	// SystemContainerdNamespace is the Containerd namespace for Talos services.
	SystemContainerdNamespace = "system"

	// TalosConfigEnvVar is the environment variable for setting the Talos configuration file path.
	TalosConfigEnvVar = "TALOSCONFIG"
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
