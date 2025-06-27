// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package constants defines constants used throughout Talos.
package constants

import (
	"time"

	cni "github.com/containerd/go-cni"
	"github.com/siderolabs/crypto/x509"
)

const (
	// DefaultKernelVersion is the default Linux kernel version.
	DefaultKernelVersion = "6.12.35-talos"

	// KernelParamConfig is the kernel parameter name for specifying the URL.
	// to the config.
	KernelParamConfig = "talos.config"

	// KernelParamConfigInline is the kernel parameter name for specifying the inline config.
	//
	// The inline config should be base64 encoded and zstd-compressed.
	KernelParamConfigInline = "talos.config.inline"

	// KernelParamConfigOAuthClientID is the kernel parameter name for specifying the OAuth2 client ID.
	KernelParamConfigOAuthClientID = "talos.config.oauth.client_id"

	// KernelParamConfigOAuthClientSecret is the kernel parameter name for specifying the OAuth2 client secret.
	KernelParamConfigOAuthClientSecret = "talos.config.oauth.client_secret"

	// KernelParamConfigOAuthAudience is the kernel parameter name for specifying the OAuth2 audience.
	KernelParamConfigOAuthAudience = "talos.config.oauth.audience"

	// KernelParamConfigOAuthScope is the kernel parameter name for specifying the OAuth2 scopes (might be repeated).
	KernelParamConfigOAuthScope = "talos.config.oauth.scope"

	// KernelParamConfigOAuthDeviceAuthURL is the kernel parameter name for specifying the OAuth2 device auth URL.
	KernelParamConfigOAuthDeviceAuthURL = "talos.config.oauth.device_auth_url"

	// KernelParamConfigOAuthTokenURL is the kernel parameter name for specifying the OAuth2 token URL.
	KernelParamConfigOAuthTokenURL = "talos.config.oauth.token_url"

	// KernelParamConfigOAuthExtraVariable is the kernel parameter name for specifying the OAuth2 extra variable (might be repeated).
	KernelParamConfigOAuthExtraVariable = "talos.config.oauth.extra_variable"

	// ConfigNone indicates no config is required.
	ConfigNone = "none"

	// KernelParamPlatform is the kernel parameter name for specifying the
	// platform.
	KernelParamPlatform = "talos.platform"

	// KernelParamBoard is the kernel parameter name for specifying the
	// SBC.
	KernelParamBoard = "talos.board"

	// KernelParamEventsSink is the kernel parameter name for specifying the
	// events sink server.
	KernelParamEventsSink = "talos.events.sink"

	// KernelParamLoggingKernel is the kernel parameter name for specifying the
	// kernel log delivery destination.
	KernelParamLoggingKernel = "talos.logging.kernel"

	// KernelParamWipe is the kernel parameter name for specifying the
	// disk to wipe on the next boot and reboot.
	KernelParamWipe = "talos.experimental.wipe"

	// KernelParamDeviceSettleTime is the kernel parameter name for specifying the
	// extra device settle timeout.
	KernelParamDeviceSettleTime = "talos.device.settle_time"

	// KernelParamCGroups is the legacy kernel parameter not supported anymore.
	KernelParamCGroups = "talos.unified_cgroup_hierarchy"

	// KernelParamAuditdDisabled is the kernel parameter name for disabling auditd service.
	KernelParamAuditdDisabled = "talos.auditd.disabled"

	// KernelParamDashboardDisabled is the kernel parameter name for disabling the dashboard.
	KernelParamDashboardDisabled = "talos.dashboard.disabled"

	// KernelParamEnvironment is the kernel parameter name for passing process environment.
	KernelParamEnvironment = "talos.environment"

	// KernelParamNetIfnames is the kernel parameter name to control predictable network interface names.
	KernelParamNetIfnames = "net.ifnames"

	// KernelParamHaltIfInstalled is the kernel parameter name to control if Talos should pause if booting from boot media while Talos is already installed.
	KernelParamHaltIfInstalled = "talos.halt_if_installed"

	// KernelParamSELinux is the kernel parameter name to enable/disable SELinux.
	KernelParamSELinux = "selinux"

	// KernelParamSELinuxEnforcing is the kernel parameter name to control SELinux enforcement mode.
	KernelParamSELinuxEnforcing = "enforcing"

	// BoardNone indicates that the install is not for a specific board.
	BoardNone = "none"

	// BoardLibretechAllH3CCH5 is the  name of the Libre Computer board ALL-H3-CC.
	BoardLibretechAllH3CCH5 = "libretech_all_h3_cc_h5"

	// BoardRPiGeneric is the  name of the Raspberry Pi Compute Module 4.
	BoardRPiGeneric = "rpi_generic"

	// BoardBananaPiM64 is the  name of the Banana Pi M64.
	BoardBananaPiM64 = "bananapi_m64"

	// BoardPine64 is the  name of the Pine64.
	BoardPine64 = "pine64"

	// BoardJetsonNano is the name of the Jetson Nano.
	BoardJetsonNano = "jetson_nano"

	// BoardRock64 is the  name of the Rock64.
	BoardRock64 = "rock64"

	// BoardRockpi4 is the name of the Radxa Rock pi 4 revisions A and B.
	BoardRockpi4 = "rockpi_4"

	// BoardRockpi4c is the name of the Radxa Rock pi 4 revision C.
	BoardRockpi4c = "rockpi_4c"

	// BoardNanoPiR4S is the name of the Friendlyelec Nano Pi R4S.
	BoardNanoPiR4S = "nanopi_r4s"

	// KernelParamHostname is the kernel parameter name for specifying the
	// hostname.
	KernelParamHostname = "talos.hostname"

	// KernelParamShutdown is the kernel parameter for specifying the
	// shutdown type (halt/poweroff).
	KernelParamShutdown = "talos.shutdown"

	// KernelParamNetworkInterfaceIgnore is the kernel parameter for specifying network interfaces which should be ignored by talos.
	KernelParamNetworkInterfaceIgnore = "talos.network.interface.ignore"

	// KernelParamVlan is the kernel parameter for specifying vlan for the interface.
	KernelParamVlan = "vlan"

	// KernelParamBonding is the kernel parameter for specifying bonded network interfaces.
	KernelParamBonding = "bond"

	// KernelParamPanic is the kernel parameter name for specifying the time to wait until rebooting after kernel panic (0 disables reboot).
	KernelParamPanic = "panic"

	// KernelParamSideroLink is the kernel parameter name to specify SideroLink API endpoint.
	KernelParamSideroLink = "siderolink.api"

	// KernelParamEquinixMetalEvents is the kernel parameter name to specify the Equinix Metal phone home endpoint.
	// This param is injected by Equinix Metal and depends on the device ID and datacenter.
	KernelParamEquinixMetalEvents = "em.events_url"

	// NewRoot is the path where the switchroot target is mounted.
	NewRoot = "/root"

	// ExtensionLayers is the path where the extensions layers are stored.
	ExtensionLayers = "/layers"

	// ExtensionsConfigFile is the extensions layers configuration file name in the initramfs.
	ExtensionsConfigFile = "/extensions.yaml"

	// ExtensionsRuntimeConfigFile extensions layers configuration file name in the rootfs.
	ExtensionsRuntimeConfigFile = "/etc/extensions.yaml"

	// EFIPartitionLabel is the label of the partition to use for mounting at
	// the boot path.
	EFIPartitionLabel = "EFI"

	// EFIMountPoint is the label of the partition to use for mounting at
	// the boot path.
	EFIMountPoint = BootMountPoint + "/EFI"

	// EFIVarsMountPoint is mount point for efivars filesystem type.
	// https://www.kernel.org/doc/html/next/filesystems/efivarfs.html
	EFIVarsMountPoint = "/sys/firmware/efi/efivars"

	// BIOSGrubPartitionLabel is the label of the partition used by grub's second
	// stage bootloader.
	BIOSGrubPartitionLabel = "BIOS"

	// MetaPartitionLabel is the label of the meta partition.
	MetaPartitionLabel = "META"

	// StatePartitionLabel is the label of the state partition.
	StatePartitionLabel = "STATE"

	// StateMountPoint is the label of the partition to use for mounting at
	// the state path.
	StateMountPoint = "/system/state"

	// StateSelinuxLabel is the label to be assigned to the state mount.
	StateSelinuxLabel = "system_u:object_r:system_state_t:s0"

	// BootPartitionLabel is the label of the partition to use for mounting at
	// the boot path.
	BootPartitionLabel = "BOOT"

	// BootMountPoint is the label of the partition to use for mounting at
	// the boot path.
	BootMountPoint = "/boot"

	// EphemeralPartitionLabel is the label of the partition to use for
	// mounting at the data path.
	EphemeralPartitionLabel = "EPHEMERAL"

	// EphemeralMountPoint is the label of the partition to use for mounting at
	// the data path.
	EphemeralMountPoint = "/var"

	// EphemeralSelinuxLabel is the label to be assigned to the ephemeral mount.
	EphemeralSelinuxLabel = "system_u:object_r:ephemeral_t:s0"

	// OptSELinuxLabel is the SELinux label to be set for /opt overlay mount.
	OptSELinuxLabel = "system_u:object_r:opt_t:s0"

	// RootMountPoint is the label of the partition to use for mounting at
	// the root path.
	RootMountPoint = "/"

	// ISOFilesystemLabel is the label of the ISO file system for the Talos
	// installer.
	ISOFilesystemLabel = "TALOS"

	// PATH defines all locations where executables are stored.
	PATH = "/usr/bin:/usr/local/sbin:/usr/local/bin:" + cni.DefaultCNIDir

	// KubernetesDefaultCertificateValidityDuration specifies default certificate duration for Kubernetes generated certificates.
	KubernetesDefaultCertificateValidityDuration = time.Hour * 24 * 365

	// KubernetesConfigBaseDir is the path to the base Kubernetes configuration directory.
	KubernetesConfigBaseDir = "/etc/kubernetes"

	// KubernetesConfigSELinuxLabel is the SELinux label to be set for the Kubernetes configuration directory overlay mount.
	KubernetesConfigSELinuxLabel = "system_u:object_r:k8s_conf_t:s0"

	// KubeletPluginsSELinuxLabel is the SELinux label to be set for the Kubernetes plugin directory overlay mount.
	KubeletPluginsSELinuxLabel = "system_u:object_r:k8s_plugin_t:s0"

	// DefaultCertificatesDir is the path the Kubernetes PKI directory.
	DefaultCertificatesDir = KubernetesConfigBaseDir + "/" + "pki"

	// KubernetesCACert is the path to the root CA certificate.
	KubernetesCACert = DefaultCertificatesDir + "/" + "ca.crt"

	// EtcdCACert is the path to the etcd CA certificate.
	EtcdCACert = EtcdPKIPath + "/" + "ca.crt"

	// EtcdCAKey is the path to the etcd CA private key.
	EtcdCAKey = EtcdPKIPath + "/" + "ca.key"

	// EtcdCert is the path to the etcd server certificate.
	EtcdCert = EtcdPKIPath + "/" + "server.crt"

	// EtcdKey is the path to the etcd server private key.
	EtcdKey = EtcdPKIPath + "/" + "server.key"

	// EtcdPeerCert is the path to the etcd peer certificate.
	EtcdPeerCert = EtcdPKIPath + "/" + "peer.crt"

	// EtcdPeerKey is the path to the etcd peer private key.
	EtcdPeerKey = EtcdPKIPath + "/" + "peer.key"

	// EtcdAdminCert is the path to the talos client certificate.
	EtcdAdminCert = EtcdPKIPath + "/" + "admin.crt"

	// EtcdAdminKey is the path to the talos client private key.
	EtcdAdminKey = EtcdPKIPath + "/" + "admin.key"

	// EtcdClientPort defines the port etcd listen on for client traffic.
	EtcdClientPort = 2379

	// EtcdPeerPort defines the port etcd listens on for peer traffic.
	EtcdPeerPort = 2380

	// KubernetesAdminCertCommonName defines CN property of Kubernetes admin certificate.
	KubernetesAdminCertCommonName = "admin"

	// KubernetesTalosAdminCertCommonName defines CN property of Kubernetes admin certificate used by Talos itself.
	KubernetesTalosAdminCertCommonName = "talos:admin"

	// KubernetesAdminCertOrganization defines Organization values of Kubernetes admin certificate.
	KubernetesAdminCertOrganization = "system:masters"

	// KubernetesAPIServerKubeletClientCommonName defines CN property of Kubernetes API server certificate to access kubelet API.
	KubernetesAPIServerKubeletClientCommonName = "apiserver-kubelet-client"

	// KubernetesControllerManagerOrganization defines Organization value of kube-controller-manager client certificate.
	KubernetesControllerManagerOrganization = "system:kube-controller-manager"

	// KubernetesSchedulerOrganization defines Organization value of kube-scheduler client certificate.
	KubernetesSchedulerOrganization = "system:kube-scheduler"

	// KubernetesAdminCertDefaultLifetime defines default lifetime for Kubernetes generated admin certificate.
	KubernetesAdminCertDefaultLifetime = 365 * 24 * time.Hour

	// KubebernetesStaticSecretsDir defines ephemeral directory which contains rendered secrets for controlplane components.
	KubebernetesStaticSecretsDir = "/system/secrets/kubernetes"

	// KubebernetesStaticConfigDir defines ephemeral directory which contains rendered configs for controlplane components.
	KubebernetesStaticConfigDir = "/system/config/kubernetes"

	// KubernetesAuditLogDir defines the ephemeral directory where the kube-apiserver will store its audit logs.
	KubernetesAuditLogDir = EphemeralMountPoint + "/" + "log" + "/" + "audit" + "/" + "kube"

	// KubernetesAPIServerSecretsDir defines directory with kube-apiserver secrets.
	KubernetesAPIServerSecretsDir = KubebernetesStaticSecretsDir + "/" + "kube-apiserver"

	// KubernetesAPIServerSecretsDirSELinuxLabel defines SELinux label for the directory with kube-apiserver secrets.
	KubernetesAPIServerSecretsDirSELinuxLabel = "system_u:object_r:kube_apiserver_secret_t:s0"

	// KubernetesAPIServerConfigDir defines directory with kube-apiserver configs.
	KubernetesAPIServerConfigDir = KubebernetesStaticConfigDir + "/" + "kube-apiserver"

	// KubernetesAPIServerConfigDirSELinuxLabel defines SELinux label for the directory with kube-apiserver configs.
	KubernetesAPIServerConfigDirSELinuxLabel = "system_u:object_r:kube_apiserver_config_t:s0"

	// KubernetesControllerManagerSecretsDir defines ephemeral directory with kube-controller-manager secrets.
	KubernetesControllerManagerSecretsDir = KubebernetesStaticSecretsDir + "/" + "kube-controller-manager"

	// KubernetesControllerManagerSecretsDirSELinuxLabel defines SELinux label for the ephemeral directory with kube-controller-manager secrets.
	KubernetesControllerManagerSecretsDirSELinuxLabel = "system_u:object_r:kube_controller_manager_secret_t:s0"

	// KubernetesSchedulerSecretsDir defines ephemeral directory with kube-scheduler secrets.
	KubernetesSchedulerSecretsDir = KubebernetesStaticSecretsDir + "/" + "kube-scheduler"

	// KubernetesSchedulerSecretsDirSELinuxLabel defines SELinux label for the ephemeral directory with kube-scheduler secrets.
	KubernetesSchedulerSecretsDirSELinuxLabel = "system_u:object_r:kube_scheduler_secret_t:s0"

	// KubernetesSchedulerConfigDir defines ephemeral directory with kube-scheduler configs.
	KubernetesSchedulerConfigDir = KubebernetesStaticConfigDir + "/" + "kube-scheduler"

	// KubernetesSchedulerConfigDirSELinuxLabel defines SELinux label for the ephemeral directory with kube-scheduler configs.
	KubernetesSchedulerConfigDirSELinuxLabel = "system_u:object_r:kube_scheduler_config_t:s0"

	// KubernetesAPIServerRunUser defines UID to the API Server.
	KubernetesAPIServerRunUser = 65534

	// KubernetesAPIServerRunGroup defines GID to run the API Server.
	KubernetesAPIServerRunGroup = 65534

	// KubernetesControllerManagerRunUser defines UID to the Controller Manager.
	KubernetesControllerManagerRunUser = 65535

	// KubernetesControllerManagerRunGroup defines GID to run the Controller Manager.
	KubernetesControllerManagerRunGroup = 65535

	// KubernetesSchedulerRunUser defines UID to the Scheduler.
	KubernetesSchedulerRunUser = 65536

	// KubernetesSchedulerRunGroup defines GID to run the Scheduler.
	KubernetesSchedulerRunGroup = 65536

	// KubeletBootstrapKubeconfig is the path to the kubeconfig required to
	// bootstrap the kubelet.
	KubeletBootstrapKubeconfig = KubernetesConfigBaseDir + "/" + "bootstrap-kubeconfig"

	// KubeletCredentialProviderBinDir is the path to the directory where kubelet credential provider binaries are stored.
	KubeletCredentialProviderBinDir = "/usr/local/lib/kubelet/credentialproviders"

	// KubeletCredentialProviderConfig is the path to the kubelet credential provider config.
	KubeletCredentialProviderConfig = KubernetesConfigBaseDir + "/" + "kubelet-credentialproviderconfig.yaml"

	// KubeletPort is the kubelet port for secure API.
	KubeletPort = 10250

	// KubeletOOMScoreAdj oom_score_adj config.
	KubeletOOMScoreAdj = -450

	// KubeletPKIDir is the path to the directory where kubelet stores issued certificates and keys.
	KubeletPKIDir = "/var/lib/kubelet/pki"

	// SystemKubeletPKIDir is the path to the directory where Talos copies kubelet issued certificates and keys.
	SystemKubeletPKIDir = "/system/secrets/kubelet"

	// KubeletShutdownGracePeriod is the kubelet shutdown grace period.
	KubeletShutdownGracePeriod = 30 * time.Second

	// KubeletShutdownGracePeriodCriticalPods is the kubelet shutdown grace period for critical pods.
	//
	// Should be less than KubeletShutdownGracePeriod.
	KubeletShutdownGracePeriodCriticalPods = 10 * time.Second

	// SeccompProfilesDirectory is the path to the directory where user provided seccomp profiles are mounted inside Kubelet.
	SeccompProfilesDirectory = "/var/lib/kubelet/seccomp/profiles"

	// DefaultKubernetesVersion is the default target version of the control plane.
	// renovate: datasource=github-releases depName=kubernetes/kubernetes
	DefaultKubernetesVersion = "1.33.1"

	// SupportedKubernetesVersions is the number of Kubernetes versions supported by Talos starting from DefaultKubernesVersion going backwards.
	SupportedKubernetesVersions = 6

	// DefaultControlPlanePort is the default port to use for the control plane.
	DefaultControlPlanePort = 6443

	// KubeletImage is the enforced kubelet image to use.
	KubeletImage = "ghcr.io/siderolabs/kubelet"

	// KubeProxyImage is the enforced kube-proxy image to use for the control plane.
	KubeProxyImage = "registry.k8s.io/kube-proxy"

	// KubernetesAPIServerImage is the enforced apiserver image to use for the control plane.
	KubernetesAPIServerImage = "registry.k8s.io/kube-apiserver"

	// KubernetesControllerManagerImage is the enforced controllermanager image to use for the control plane.
	KubernetesControllerManagerImage = "registry.k8s.io/kube-controller-manager"

	// KubernetesSchedulerImage is the enforced scheduler image to use for the control plane.
	KubernetesSchedulerImage = "registry.k8s.io/kube-scheduler"

	// CoreDNSImage is the enforced CoreDNS image to use.
	CoreDNSImage = "registry.k8s.io/coredns/coredns"

	// DefaultCoreDNSVersion is the default version for the CoreDNS.
	// renovate: datasource=docker depName=registry.k8s.io/coredns/coredns
	DefaultCoreDNSVersion = "v1.12.1"

	// LabelNodeRoleControlPlane is the node label required by a control plane node.
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"

	// LabelExcludeFromExternalLB can be set on a node to exclude it from external load balancers.
	LabelExcludeFromExternalLB = "node.kubernetes.io/exclude-from-external-load-balancers"

	// ManifestsDirectory is the directory that contains all static manifests.
	ManifestsDirectory = KubernetesConfigBaseDir + "/" + "manifests"

	// TalosManifestPrefix is the prefix for static pod files created in ManifestsDirectory by Talos.
	TalosManifestPrefix = "talos-"

	// KubeletKubeconfig is the generated kubeconfig for kubelet.
	KubeletKubeconfig = KubernetesConfigBaseDir + "/" + "kubeconfig-kubelet"

	// KubeletSystemReservedCPU cpu system reservation value for kubelet kubeconfig.
	KubeletSystemReservedCPU = "50m"

	// KubeletSystemReservedMemoryControlPlane memory system reservation value for kubelet kubeconfig (controlplane nodes).
	KubeletSystemReservedMemoryControlPlane = "512Mi"

	// KubeletSystemReservedMemoryWorker memory system reservation value for kubelet kubeconfig (worker nodes).
	KubeletSystemReservedMemoryWorker = "384Mi"

	// KubeletSystemReservedPid pid system reservation value for kubelet kubeconfig.
	KubeletSystemReservedPid = "100"

	// KubeletSystemReservedEphemeralStorage ephemeral-storage system reservation value for kubelet kubeconfig.
	KubeletSystemReservedEphemeralStorage = "256Mi"

	// DefaultEtcdVersion is the default target version of etcd.
	// renovate: datasource=github-releases depName=etcd-io/etcd
	DefaultEtcdVersion = "v3.5.21"

	// EtcdRootTalosKey is the root etcd key for Talos-specific storage.
	EtcdRootTalosKey = "talos:v1"

	// EtcdTalosEtcdUpgradeMutex is the etcd mutex prefix to be used to set an etcd upgrade lock.
	EtcdTalosEtcdUpgradeMutex = EtcdRootTalosKey + ":etcdUpgradeMutex"

	// EtcdTalosManifestApplyMutex is the etcd mutex prefix used by manifest apply controller.
	EtcdTalosManifestApplyMutex = EtcdRootTalosKey + ":manifestApplyMutex"

	// EtcdTalosServiceAccountCRDControllerMutex is the etcd mutex prefix used by Talos ServiceAccount crd controller.
	EtcdTalosServiceAccountCRDControllerMutex = EtcdRootTalosKey + ":serviceAccountCRDController"

	// EtcdImage is the reposistory for the etcd image.
	EtcdImage = "gcr.io/etcd-development/etcd"

	// EtcdPKIPath is the path to the etcd PKI directory.
	EtcdPKIPath = "/system/secrets/etcd"

	// EtcdPKISELinuxLabel is the SELinux label for the etcd PKI directory.
	EtcdPKISELinuxLabel = "system_u:object_r:etcd_pki_t:s0"

	// EtcdDataPath is the path where etcd stores its' data.
	EtcdDataPath = "/var/lib/etcd"

	// EtcdDataVolumeID is the ID of the etcd data volume.
	EtcdDataVolumeID = "ETCD"

	// EtcdDataSELinuxLabel is the SELinux label for the etcd data directory.
	EtcdDataSELinuxLabel = "system_u:object_r:etcd_data_t:s0"

	// EtcdRecoverySnapshotPath is the path where etcd snapshot is uploaded for recovery.
	EtcdRecoverySnapshotPath = "/var/lib/etcd.snapshot"

	// EtcdUserID is the user ID for the etcd process.
	EtcdUserID = 60

	// ConfigFilename is the filename of the saved config in STATE partition.
	ConfigFilename = "config.yaml"

	// ConfigTryTimeout is the timeout of the config apply in try mode.
	ConfigTryTimeout = time.Minute

	// MetalConfigISOLabel is the volume label for ISO based configuration.
	MetalConfigISOLabel = "metal-iso"

	// ConfigGuestInfo is the name of the VMware guestinfo config strategy.
	ConfigGuestInfo = "guestinfo"

	// VMwareGuestInfoPrefix is the prefix to extraConfig variables.
	VMwareGuestInfoPrefix = "guestinfo."

	// VMwareGuestInfoConfigKey is the guestinfo key used to provide a config file.
	VMwareGuestInfoConfigKey = "talos.config"

	// VMwareGuestInfoFallbackKey is the fallback guestinfo key used to provide a config file.
	VMwareGuestInfoFallbackKey = "userdata"

	// VMwareGuestInfoMetadataKey is the guestinfo key used to provide metadata.
	VMwareGuestInfoMetadataKey = "metadata"

	// VMwareGuestInfoOvfEnvKey is the guestinfo key used to provide the OVF environment.
	VMwareGuestInfoOvfEnvKey = "ovfenv"

	// AuditPolicyPath is the path to the audit-policy.yaml relative to initramfs.
	AuditPolicyPath = KubernetesConfigBaseDir + "/" + "audit-policy.yaml"

	// EncryptionConfigPath is the path to the EncryptionConfig relative to initramfs.
	EncryptionConfigPath = KubernetesConfigBaseDir + "/" + "encryptionconfig.yaml"

	// EncryptionConfigRootfsPath is the path to the EncryptionConfig relative to rootfs.
	EncryptionConfigRootfsPath = KubernetesConfigBaseDir + "/" + "encryptionconfig.yaml"

	// ApidPort is the port for the apid service.
	ApidPort = 50000

	// ApidUserID is the user ID for apid.
	ApidUserID = 50

	// DashboardUserID is the user ID for dashboard.
	// We use the same user ID as apid so that the dashboard can write to the machined unix socket.
	DashboardUserID = ApidUserID

	// DashboardPriority is the priority for the dashboard service.
	// Higher nice value for the dashboard to give more CPU time to other services when under load.
	DashboardPriority = 10

	// TrustdPort is the port for the trustd service.
	TrustdPort = 50001

	// TrustdUserID is the user ID for trustd.
	TrustdUserID = 51

	// DefaultContainerdVersion is the default container runtime version.
	DefaultContainerdVersion = "2.1.3"

	// SystemContainerdNamespace is the Containerd namespace for Talos services.
	SystemContainerdNamespace = "system"

	// SystemContainerdAddress is the path to the system containerd socket.
	SystemContainerdAddress = SystemRunPath + "/containerd/containerd.sock"

	// K8sContainerdNamespace is the Containerd namespace for CRI pods.
	K8sContainerdNamespace = "k8s.io"

	// CRIContainerdAddress is the path to the CRI containerd socket address.
	CRIContainerdAddress = "/run/containerd/containerd.sock"

	// CRIContainerdConfig is the path to the config for the containerd instance that provides the CRI.
	CRIContainerdConfig = "/etc/cri/containerd.toml"

	// CRIConfdPath is the path to the directory providing parts of CRI plugin configuration.
	CRIConfdPath = "/etc/cri/conf.d"

	// CRIConfig is the path to the CRI merged configuration file relative to /etc.
	CRIConfig = "cri/conf.d/cri.toml"

	// CRIRegistryConfigPart is the path to the CRI generated registry configuration relative to /etc.
	CRIRegistryConfigPart = "cri/conf.d/01-registries.part"

	// CRICustomizationConfigPart is the path to the CRI generated registry configuration relative to /etc.
	CRICustomizationConfigPart = "cri/conf.d/20-customization.part"

	// CRIBaseRuntimeSpec is the path to the base runtime spec for the CRI.
	CRIBaseRuntimeSpec = "cri/conf.d/base-spec.json"

	// TalosConfigEnvVar is the environment variable for setting the Talos configuration file path.
	TalosConfigEnvVar = "TALOSCONFIG"

	// APISocketPath is the path to file socket of apid.
	APISocketPath = SystemRunPath + "/apid/apid.sock"

	// APISocketLabel is the SELinux label for apid socket file.
	APISocketLabel = "system_u:object_r:apid_socket_t:s0"

	// APIRuntimeSocketPath is the path to file socket of runtime server for apid.
	APIRuntimeSocketPath = SystemRunPath + "/apid/runtime.sock"

	// APIRuntimeSocketLabel is the SELinux label for apid runtime socket file.
	APIRuntimeSocketLabel = "system_u:object_r:apid_runtime_socket_t:s0"

	// TrustdRuntimeSocketPath is the path to file socket of runtime server for trustd.
	TrustdRuntimeSocketPath = SystemRunPath + "/trustd/runtime.sock"

	// TrustdRuntimeSocketLabel is the SELinux label for trustd runtime socket file.
	TrustdRuntimeSocketLabel = "system_u:object_r:trustd_runtime_socket_t:s0"

	// MachineSocketPath is the path to file socket of machine API.
	MachineSocketPath = SystemRunPath + "/machined/machine.sock"

	// MachineSocketLabel is the SELinux label for socket of machine API.
	MachineSocketLabel = "system_u:object_r:machine_socket_t:s0"

	// NetworkSocketPath is the path to file socket of network API.
	NetworkSocketPath = SystemRunPath + "/networkd/networkd.sock"

	// ArchVariable is replaced automatically by the target cluster arch.
	ArchVariable = "${ARCH}"

	// KernelAsset defines a well known name for our kernel filename.
	KernelAsset = "vmlinuz"

	// KernelAssetWithArch defines a well known name for our kernel filename with arch variable.
	KernelAssetWithArch = "vmlinuz-" + ArchVariable

	// KernelAssetPath is the path to the kernel on disk.
	KernelAssetPath = "/usr/install/%s/" + KernelAsset

	// InitramfsAsset defines a well known name for our initramfs filename.
	InitramfsAsset = "initramfs.xz"

	// InitramfsAssetWithArch defines a well known name for our initramfs filename with arch variable.
	InitramfsAssetWithArch = "initramfs-" + ArchVariable + ".xz"

	// InitramfsAssetPath is the path to the initramfs on disk.
	InitramfsAssetPath = "/usr/install/%s/" + InitramfsAsset

	// RootfsAsset defines a well known name for our rootfs filename.
	RootfsAsset = "rootfs.sqsh"

	// UKIAsset defines a well known name for our UKI filename.
	UKIAsset = "vmlinuz.efi"

	// UKIAssetPath is the path to the UKI in the installer.
	UKIAssetPath = "/usr/install/%s/" + UKIAsset

	// SDStubAsset defines a well known name for our systemd-stub filename.
	SDStubAsset = "systemd-stub.efi"

	// SDStubAssetPath is the path to the systemd-stub in the installer.
	SDStubAssetPath = "/usr/install/%s/" + SDStubAsset

	// SDBootAsset defines a well known name for our SDBoot filename.
	SDBootAsset = "systemd-boot.efi"

	// SDBootAssetPath is the path to the SDBoot in the installer.
	SDBootAssetPath = "/usr/install/%s/" + SDBootAsset

	// DTBAssetPath is the path to the device tree blobs in the installer.
	DTBAssetPath = "/usr/install/%s/dtb"

	// UBootAssetPath is the path to the u-boot in the installer.
	UBootAssetPath = "/usr/install/%s/u-boot"

	// RPiFirmwareAssetPath is the path to the raspberrypi firmware in the installer.
	RPiFirmwareAssetPath = "/usr/install/%s/raspberrypi-firmware"

	// ImagerOverlayBasePath is the base path for the imager overlay.
	ImagerOverlayBasePath = "/overlay"
	// ImagerOverlayArtifactsPath is the path to the artifacts in the imager overlay.
	ImagerOverlayArtifactsPath = ImagerOverlayBasePath + "/" + "artifacts"
	// ImagerOverlayInstallersPath is the path to the installers in the imager overlay.
	ImagerOverlayInstallersPath = ImagerOverlayBasePath + "/" + "installers"
	// ImagerOverlayProfilesPath is the path to the profiles in the imager overlay.
	ImagerOverlayProfilesPath = ImagerOverlayBasePath + "/" + "profiles"
	// ImagerOverlayInstallerDefault is the default installer name.
	ImagerOverlayInstallerDefault = "default"
	// ImagerOverlayInstallerDefaultPath is the path to the default installer in the imager overlay.
	ImagerOverlayInstallerDefaultPath = ImagerOverlayInstallersPath + "/" + ImagerOverlayInstallerDefault
	// ImagerOverlayExtraOptionsPath is the path to the generated extra options file in the imager overlay.
	ImagerOverlayExtraOptionsPath = ImagerOverlayBasePath + "/" + "extra-options"

	// PlatformKeyAsset defines a well known name for the platform key filename used for auto-enrolling.
	PlatformKeyAsset = "PK.auth"

	// KeyExchangeKeyAsset defines a well known name for the key exchange key filename used for auto-enrolling.
	KeyExchangeKeyAsset = "KEK.auth"

	// SignatureKeyAsset defines a well known name for the signature key filename used for auto-enrolling.
	SignatureKeyAsset = "db.auth"

	// SecureBootSigningKeyAsset defines a well known name for the secure boot signing key filename.
	SecureBootSigningKeyAsset = "uki-signing-key.pem"

	// SecureBootSigningCertAsset defines a well known name for the secure boot signing key filename.
	SecureBootSigningCertAsset = "uki-signing-cert.pem"

	// PCRSigningKeyAsset defines a well known name for the PCR signing key filename.
	PCRSigningKeyAsset = "pcr-signing-key.pem"

	// SDStubDynamicInitrdPath is the path where dynamically generated initrds are placed by systemd-stub.
	// https://www.mankier.com/7/systemd-stub#Description
	SDStubDynamicInitrdPath = "/.extra"

	// PCRSignatureJSON is the path to the PCR signature JSON file.
	// https://www.mankier.com/7/systemd-stub#Initrd_Resources
	PCRSignatureJSON = SDStubDynamicInitrdPath + "/" + "tpm2-pcr-signature.json"

	// PCRPublicKey is the path to the PCR public key file.
	// https://www.mankier.com/7/systemd-stub#Initrd_Resources
	PCRPublicKey = SDStubDynamicInitrdPath + "/" + "tpm2-pcr-public-key.pem"

	// UKIPCR is the PCR number where systemd-stub measures the UKI.
	UKIPCR = 11

	// DefaultCertificateValidityDuration is the default duration for a certificate.
	DefaultCertificateValidityDuration = x509.DefaultCertificateValidityDuration

	// SystemPath is the path to write temporary runtime system related files
	// and directories.
	SystemPath = "/system"

	// SystemSelinuxLabel is the SELinux label for runtime system related files and directories.
	SystemSelinuxLabel = "system_u:object_r:system_t:s0"

	// RunPath is the path to the system run directory.
	RunPath = "/run"

	// RunSelinuxLabel is the SELinux label for the run directory.
	RunSelinuxLabel = "system_u:object_r:run_t:s0"

	// VarSystemOverlaysPath is the path where overlay mounts are created.
	VarSystemOverlaysPath = "/var/system/overlays"

	// SystemRunPath is the path to the system run directory.
	SystemRunPath = SystemPath + "/run"

	// SystemVarPath is the path to the system var directory.
	SystemVarPath = SystemPath + "/var"

	// SystemVarSelinuxLabel is the SELinux label for the system var directory.
	SystemVarSelinuxLabel = "system_u:object_r:system_var_t:s0"

	// SystemEtcPath is the path to the system etc directory.
	SystemEtcPath = SystemPath + "/etc"

	// EtcSelinuxLabel is the SELinux label for the /etc and /system/etc directories.
	EtcSelinuxLabel = "system_u:object_r:etc_t:s0"

	// SystemLibexecPath is the path to the system libexec directory.
	SystemLibexecPath = SystemPath + "/libexec"

	// SystemOverlaysPath is the path to the system overlay directory.
	SystemOverlaysPath = SystemPath + "/overlays"

	// CgroupMountPath is the default mount path for unified cgroupsv2 setup.
	CgroupMountPath = "/sys/fs/cgroup"

	// CgroupInit is the cgroup name for init process.
	CgroupInit = "/init"

	// CgroupInitReservedMemory is the hard memory protection for the init process.
	CgroupInitReservedMemory = 96 * 1024 * 1024

	// CgroupInitMillicores is the CPU weight for the init process.
	CgroupInitMillicores = 2000

	// CgroupSystem is the cgroup name for system processes.
	CgroupSystem = "/system"

	// CgroupSystemMillicores is the CPU weight for the system cgroup.
	CgroupSystemMillicores = 1500

	// CgroupSystemReservedMemory is the hard memory protection for the system processes.
	CgroupSystemReservedMemory = 96 * 1024 * 1024

	// CgroupSystemRuntime is the cgroup name for containerd runtime processes.
	CgroupSystemRuntime = CgroupSystem + "/runtime"

	// CgroupSystemRuntimeReservedMemory is the hard memory protection for the system containerd process.
	CgroupSystemRuntimeReservedMemory = 48 * 1024 * 1024

	// CgroupSystemRuntimeMillicores is the CPU weight for the system containerd process.
	CgroupSystemRuntimeMillicores = 500

	// SelinuxLabelMachined is the SELinux label for machined.
	SelinuxLabelMachined = "system_u:system_r:init_t:s0"

	// SelinuxLabelInstaller is the SELinux label for the installer.
	SelinuxLabelInstaller = "system_u:system_r:installer_t:s0"

	// SelinuxLabelUnconfinedSysContainer is the SELinux label for system containers without label set (normally extensions).
	SelinuxLabelUnconfinedSysContainer = "system_u:system_r:unconfined_container_t:s0"

	// SelinuxLabelUnconfinedService is the SELinux label for process without label set (normally should not occur).
	SelinuxLabelUnconfinedService = "system_u:system_r:unconfined_service_t:s0"

	// SelinuxLabelSystemRuntime is the SELinux label for containerd runtime processes.
	SelinuxLabelSystemRuntime = "system_u:system_r:sys_containerd_t:s0"

	// CgroupApid is the cgroup name for apid runtime processes.
	CgroupApid = CgroupSystem + "/apid"

	// CgroupApidReservedMemory is the hard memory protection for the apid processes.
	CgroupApidReservedMemory = 16 * 1024 * 1024

	// CgroupApidMaxMemory is the hard memory limit for the apid process.
	CgroupApidMaxMemory = 128 * 1024 * 1024

	// CgroupApidMillicores is the CPU weight for the apid process.
	CgroupApidMillicores = 500

	// SelinuxLabelApid is the SELinux label for apid runtime processes.
	SelinuxLabelApid = "system_u:system_r:apid_t:s0"

	// CgroupTrustd is the cgroup name for trustd runtime processes.
	CgroupTrustd = CgroupSystem + "/trustd"

	// CgroupTrustdReservedMemory is the hard memory protection for the trustd processes.
	CgroupTrustdReservedMemory = 8 * 1024 * 1024

	// CgroupTrustdMaxMemory is the hard memory limit for the trustd process.
	CgroupTrustdMaxMemory = 24 * 1024 * 1024

	// CgroupTrustdMillicores is the CPU weight for the trustd process.
	CgroupTrustdMillicores = 250

	// SelinuxLabelTrustd is the SELinux label for trustd runtime processes.
	SelinuxLabelTrustd = "system_u:system_r:trustd_t:s0"

	// CgroupUdevd is the cgroup name for udevd runtime processes.
	CgroupUdevd = CgroupSystem + "/udevd"

	// CgroupUdevdReservedMemory is the hard memory protection for the udevd processes.
	CgroupUdevdReservedMemory = 8 * 1024 * 1024

	// CgroupUdevdMillicores is the CPU weight for the udevd process.
	CgroupUdevdMillicores = 250

	// SelinuxLabelUdevd is the SELinux label for udevd runtime processes.
	SelinuxLabelUdevd = "system_u:system_r:udev_t:s0"

	// CgroupExtensions is the cgroup name for system extension processes.
	CgroupExtensions = CgroupSystem + "/extensions"

	// CgroupDashboard is the cgroup name for dashboard process.
	CgroupDashboard = CgroupSystem + "/dashboard"

	// SelinuxLabelDashboard is the SELinux label for dashboard process.
	SelinuxLabelDashboard = "system_u:system_r:dashboard_t:s0"

	// CgroupPodRuntimeRoot is the cgroup containing Kubernetes runtime components.
	CgroupPodRuntimeRoot = "/podruntime"

	// CgroupPodRuntimeRootMillicores is the CPU weight for the pod runtime cgroup.
	CgroupPodRuntimeRootMillicores = 4000

	// CgroupPodRuntime is the cgroup name for kubernetes containerd runtime processes.
	CgroupPodRuntime = CgroupPodRuntimeRoot + "/runtime"

	// CgroupPodRuntimeMillicores is the CPU weight for the pod runtime cgroup.
	CgroupPodRuntimeMillicores = 1000

	// SelinuxLabelPodRuntime is the SELinux label for kubernetes containerd runtime processes.
	SelinuxLabelPodRuntime = "system_u:system_r:pod_containerd_t:s0"

	// CgroupPodRuntimeReservedMemory is the hard memory protection for the cri runtime processes.
	CgroupPodRuntimeReservedMemory = 196 * 1024 * 1024

	// CgroupEtcd is the cgroup name for etcd process.
	CgroupEtcd = CgroupPodRuntimeRoot + "/etcd"

	// CgroupEtcdReservedMemory is the soft memory protection for the etcd processes.
	CgroupEtcdReservedMemory = 256 * 1024 * 1024

	// CgroupEtcdMillicores is the CPU weight for the etcd process.
	CgroupEtcdMillicores = 2000

	// SELinuxLabelEtcd is the SELinux label for etcd process.
	SELinuxLabelEtcd = "system_u:system_r:etcd_t:s0"

	// CgroupKubelet is the cgroup name for kubelet process.
	CgroupKubelet = CgroupPodRuntimeRoot + "/kubelet"

	// SelinuxLabelKubelet is the SELinux label for kubelet process.
	SelinuxLabelKubelet = "system_u:system_r:kubelet_t:s0"

	// CgroupKubeletReservedMemory is the hard memory protection for the kubelet processes.
	CgroupKubeletReservedMemory = 96 * 1024 * 1024

	// CgroupKubeletMillicores is the CPU weight for the kubelet process.
	CgroupKubeletMillicores = 1000

	// CgroupDashboardMaxMemory is the hard memory limit for the dashboard process.
	CgroupDashboardMaxMemory = 196 * 1024 * 1024

	// CgroupDashboardMillicores is the CPU weight for the dashboard process.
	CgroupDashboardMillicores = 200

	// FlannelCNI is the string to use Tanos-managed Flannel CNI (default).
	FlannelCNI = "flannel"

	// CustomCNI is the string to use custom CNI managed by Tanos with extra manifests.
	CustomCNI = "custom"

	// NoneCNI is the string to indicate that CNI will not be managed by Talos.
	NoneCNI = "none"

	// CNISELinuxLabel is the SELinux label to be set for CNI configuration overlay mount.
	CNISELinuxLabel = "system_u:object_r:cni_conf_t:s0"

	// DefaultIPv4PodNet is the IPv4 network to be used for kubernetes Pods.
	DefaultIPv4PodNet = "10.244.0.0/16"

	// DefaultIPv4ServiceNet is the IPv4 network to be used for kubernetes Services.
	DefaultIPv4ServiceNet = "10.96.0.0/12"

	// DefaultIPv6PodNet is the IPv6 network to be used for kubernetes Pods.
	DefaultIPv6PodNet = "fc00:db8:10::/56"

	// DefaultIPv6ServiceNet is the IPv6 network to be used for kubernetes Services.
	DefaultIPv6ServiceNet = "fc00:db8:20::/112"

	// DefaultDNSDomain is the default DNS domain.
	DefaultDNSDomain = "cluster.local"

	// ConfigLoadTimeout is the timeout to wait for the config to be loaded from an external source.
	ConfigLoadTimeout = 3 * time.Hour

	// ConfigLoadAttemptTimeout is the timeout for a single attempt to download config.
	ConfigLoadAttemptTimeout = 3 * time.Minute

	// BootTimeout is the timeout to run all services.
	BootTimeout = 70 * time.Minute

	// FailurePauseTimeout is the timeout for the sequencer failures which can be fixed by updating the machine config.
	FailurePauseTimeout = 35 * time.Minute

	// EtcdJoinTimeout is the timeout for etcd to join the existing cluster.
	//
	// BootTimeout should be higher than EtcdJoinTimeout.
	EtcdJoinTimeout = 30 * time.Minute

	// NodeReadyTimeout is the timeout to wait for the node to be ready (CNI to be running).
	// For bootstrap API, this includes time to run bootstrap.
	NodeReadyTimeout = BootTimeout

	// AnnotationCordonedKey is the annotation key for the nodes cordoned by Talos.
	AnnotationCordonedKey = "talos.dev/cordoned"

	// AnnotationCordonedValue is the annotation key for the nodes cordoned by Talos.
	AnnotationCordonedValue = "true"

	// AnnotationStaticPodSecretsVersion is the annotation key for the static pod secret version.
	AnnotationStaticPodSecretsVersion = "talos.dev/secrets-version"

	// AnnotationStaticPodConfigVersion is the annotation key for the static pod config version.
	AnnotationStaticPodConfigVersion = "talos.dev/config-version"

	// AnnotationStaticPodConfigFileVersion is the annotation key for the static pod configuration file version.
	AnnotationStaticPodConfigFileVersion = "talos.dev/config-file-version"

	// AnnotationOwnedLabels is the annotation key for the list of node labels owned by Talos.
	AnnotationOwnedLabels = "talos.dev/owned-labels"

	// AnnotationOwnedAnnotations is the annotation key for the list of node annotations owned by Talos.
	AnnotationOwnedAnnotations = "talos.dev/owned-annotations"

	// AnnotationOwnedTaints is the annotation key for the list of node taints owned by Talos.
	AnnotationOwnedTaints = "talos.dev/owned-taints"

	// K8sExtensionPrefix is the prefix for node labels/annotations listing extensions.
	K8sExtensionPrefix = "extensions.talos.dev/"

	// DefaultNTPServer is the NTP server to use if not configured explicitly.
	DefaultNTPServer = "time.cloudflare.com"

	// DefaultPrimaryResolver is the default primary DNS server.
	DefaultPrimaryResolver = "1.1.1.1"

	// DefaultSecondaryResolver is the default secondary DNS server.
	DefaultSecondaryResolver = "8.8.8.8"

	// DefaultClusterIDSize is the default size in bytes for the cluster ID token.
	DefaultClusterIDSize = 32

	// DefaultClusterSecretSize is the default size in bytes for the cluster secret.
	DefaultClusterSecretSize = 32

	// DefaultNodeIdentitySize is the default size in bytes for the node ID.
	DefaultNodeIdentitySize = 32

	// NodeIdentityFilename is the filename to cache node identity across reboots.
	NodeIdentityFilename = "node-identity.yaml"

	// DefaultDiscoveryServiceEndpoint is the default endpoint for Talos discovery service.
	DefaultDiscoveryServiceEndpoint = "https://discovery.talos.dev/"

	// KubeSpanIdentityFilename is the filename to cache KubeSpan identity across reboots.
	KubeSpanIdentityFilename = "kubespan-identity.yaml"

	// KubeSpanDefaultPort is the default Wireguard listening port for incoming connections.
	KubeSpanDefaultPort = 51820

	// KubeSpanDefaultRoutingTable is the default routing table for KubeSpan LAN targets.
	//
	// This specifies the routing table which will be used for Wireguard-available destinations.
	KubeSpanDefaultRoutingTable = 180

	// KubeSpanDefaultFirewallMark is the default firewall mark to use for Wireguard encrypted egress packets.
	//
	// Normal Wireguard configurations will NOT use this firewall mark.
	KubeSpanDefaultFirewallMark = 0x20

	// KubeSpanDefaultForceFirewallMark is the default firewall mark to use for packets destined to IPs serviced by KubeSpan.
	//
	// It is used to signal that matching packets should be forced into the Wireguard interface.
	KubeSpanDefaultForceFirewallMark = 0x40

	// KubeSpanDefaultFirewallMask is the mask applied to the packet mark when matching and setting the mark.
	//
	// This mask signals the bits of the firewall mark used by KubeSpan.
	KubeSpanDefaultFirewallMask = KubeSpanDefaultFirewallMark | KubeSpanDefaultForceFirewallMark

	// KubeSpanDefaultPeerKeepalive is the interval at which Wireguard Peer Keepalives should be sent.
	KubeSpanDefaultPeerKeepalive = 25 * time.Second

	// NetworkSelfIPsAnnotation is the node annotation used to list the (comma-separated) IP addresses of the host, as discovered by Talos tooling.
	NetworkSelfIPsAnnotation = "networking.talos.dev/self-ips"

	// NetworkAPIServerPortAnnotation is the node annotation used to report the control plane API server port.
	NetworkAPIServerPortAnnotation = "networking.talos.dev/api-server-port"

	// ClusterNodeIDAnnotation is the node annotation used to represent node ID.
	ClusterNodeIDAnnotation = "cluster.talos.dev/node-id"

	// KubeSpanIPAnnotation is the node annotation to be used for indicating the Wireguard IP of the node.
	KubeSpanIPAnnotation = "networking.talos.dev/kubespan-ip"

	// KubeSpanPublicKeyAnnotation is the node annotation to be used for indicating the Wireguard Public Key of the node.
	KubeSpanPublicKeyAnnotation = "networking.talos.dev/kubespan-public-key"

	// KubeSpanAssignedPrefixesAnnotation is the node annotation use to list the (comma-separated) set of IP prefixes for which the annotated node should be responsible.
	KubeSpanAssignedPrefixesAnnotation = "networking.talos.dev/assigned-prefixes"

	// KubeSpanKnownEndpointsAnnotation is the node annotation used to list the (comma-separated) known-good Wireguard endpoints for the node, as seen by other peers.
	KubeSpanKnownEndpointsAnnotation = "networking.talos.dev/kubespan-endpoints"

	// KubeSpanLinkName is the link name for the KubeSpan Wireguard interface.
	KubeSpanLinkName = "kubespan"

	// KubeSpanLinkMTU is the default link MTU size for the KubeSpan Wireguard interface.
	KubeSpanLinkMTU = 1420

	// KubeSpanLinkMinimumMTU is the minimum link MTU size for the KubeSpan Wireguard interface.
	//
	// This is the minimum MTU size for the Wireguard interface with IPv6 enabled.
	// See: https://lore.kernel.org/wireguard/20190321033638.1ff82682@natsu/t/
	KubeSpanLinkMinimumMTU = 1280

	// UdevDir is the path to the udev directory.
	UdevDir = "/usr/lib/udev"

	// UdevRulesPath rules file path.
	UdevRulesPath = UdevDir + "/" + "rules.d/99-talos.rules"

	// UdevRulesLabel rules file SELinux label.
	UdevRulesLabel = "system_u:object_r:udev_rules_t:s0"

	// LoggingFormatJSONLines represents "JSON lines" logging format.
	LoggingFormatJSONLines = "json_lines"

	// SideroLinkName is the interface name for SideroLink.
	SideroLinkName = "siderolink"

	// SideroLinkTunnelName is the tunnel name for SideroLink in tunnel (Wireguard-over-GRPC) mode.
	SideroLinkTunnelName = "siderolinktun"

	// SideroLinkDefaultPeerKeepalive is the interval at which Wireguard Peer Keepalives should be sent.
	SideroLinkDefaultPeerKeepalive = 25 * time.Second

	// PlatformNetworkConfigFilename is the filename to cache platform network configuration reboots.
	PlatformNetworkConfigFilename = "platform-network.yaml"

	// ExtensionServiceConfigPath is the directory path which contains  configuration files of extension services.
	//
	// See pkg/machinery/extensions/services for the file format.
	ExtensionServiceConfigPath = "/usr/local/etc/containers"

	// ExtensionServiceRootfsPath is the path to the extracted rootfs files of extension services.
	ExtensionServiceRootfsPath = "/usr/local/lib/containers"

	// ExtensionServiceUserConfigPath is the path to the user provided extension services config directory.
	ExtensionServiceUserConfigPath = SystemOverlaysPath + "/extensions"

	// DBusServiceSocketPath is the path to the D-Bus socket for the logind mock to connect to.
	DBusServiceSocketPath = SystemRunPath + "/dbus/service.socket"

	// DBusServiceSocketLabel is the SELinux label for the D-Bus socket for the logind mock to connect to.
	DBusServiceSocketLabel = "system_u:object_r:dbus_service_socket_t:s0"

	// DBusClientSocketPath is the path to the D-Bus socket for the kubelet to connect to.
	DBusClientSocketPath = "/run/dbus/system_bus_socket"

	// DBusClientSocketLabel is the SELinux label for the D-Bus socket for the kubelet to connect to.
	DBusClientSocketLabel = "system_u:object_r:dbus_client_socket_t:s0"

	// GoVersion is the version of Go compiler this release was built with.
	GoVersion = "go1.24.4"

	// KubernetesTalosAPIServiceName is the name of the Kubernetes service to access Talos API.
	KubernetesTalosAPIServiceName = "talos"

	// KubernetesTalosAPIServiceNamespace is the namespace of the Kubernetes service to access Talos API.
	KubernetesTalosAPIServiceNamespace = "default"

	// TalosDir is the default name of the Talos directory under user home.
	TalosDir = ".talos"

	// TalosconfigFilename is the file name of Talosconfig under TalosDir or under ServiceAccountMountPath inside a pod.
	TalosconfigFilename = "config"

	// KubernetesTalosProvider is the name of the Talos provider as a Kubernetes label.
	KubernetesTalosProvider = "talos.dev"

	// ServiceAccountResourceGroup is the group name of the Talos service account CRD.
	ServiceAccountResourceGroup = "talos.dev"

	// ServiceAccountResourceVersion is the version of the Talos service account CRD.
	ServiceAccountResourceVersion = "v1alpha1"

	// ServiceAccountResourceKind is the kind name of the Talos service account CRD.
	ServiceAccountResourceKind = "ServiceAccount"

	// ServiceAccountResourceSingular is the singular name of the Talos service account CRD.
	ServiceAccountResourceSingular = "serviceaccount"

	// ServiceAccountResourceShortName is the short name of the service account CRD.
	ServiceAccountResourceShortName = "tsa"

	// ServiceAccountResourcePlural is the plural name of the service account CRD.
	ServiceAccountResourcePlural = ServiceAccountResourceSingular + "s"

	// ServiceAccountMountPath is the path of the directory in which the Talos service account secrets are mounted.
	ServiceAccountMountPath = "/var/run/secrets/talos.dev"

	// DefaultTrustedRelativeCAFile is the default path to the trusted CA file relative to the /etc.
	DefaultTrustedRelativeCAFile = "ssl/certs/ca-certificates.crt"

	// DefaultTrustedCAFile is the default path to the trusted CA file.
	DefaultTrustedCAFile = "/etc/" + DefaultTrustedRelativeCAFile

	// MachinedMaxProcs is the maximum number of GOMAXPROCS for machined.
	MachinedMaxProcs = 4

	// ApidMaxProcs is the maximum number of GOMAXPROCS for apid.
	ApidMaxProcs = 2

	// TrustdMaxProcs is the maximum number of GOMAXPROCS for trustd.
	TrustdMaxProcs = 2

	// DashboardMaxProcs is the maximum number of GOMAXPROCS for dashboard.
	DashboardMaxProcs = 2

	// APIAuthzRoleMetadataKey is the gRPC metadata key used to submit a role with os:impersonator.
	APIAuthzRoleMetadataKey = "talos-role"

	// KernelLogsTTY is the number of the TTY device (/dev/ttyN) to redirect Kernel logs to.
	KernelLogsTTY = 1

	// DashboardTTY is the number of the TTY device (/dev/ttyN) for dashboard.
	DashboardTTY = 2

	// FlannelVersion is the version of flannel to use.
	FlannelVersion = "v0.27.0"

	// PlatformMetal is the name of the metal platform.
	PlatformMetal = "metal"

	// MetaValuesEnvVar is the name of the environment variable to store encoded meta values for the disk image (installer).
	MetaValuesEnvVar = "INSTALLER_META_BASE64"

	// MaintenanceServiceCommonName is the CN of the maintenance service server certificate.
	MaintenanceServiceCommonName = "maintenance-service.talos.dev"

	// GRPCMaxMessageSize is the maximum message size for Talos API.
	GRPCMaxMessageSize = 32 * 1024 * 1024

	// TcellMinimizeEnvironment is the environment variable to minimize tcell library memory usage (skips rune width calculation).
	TcellMinimizeEnvironment = "TCELL_MINIMIZE=1"

	// DefaultKubePrismPort is the default port for the KubePrism loadbalancer.
	DefaultKubePrismPort = 7445

	// KubePrismDialTimeout is the timeout for the KubePrism loadbalancer dialing an endpoint.
	KubePrismDialTimeout = 15 * time.Second

	// KubePrismKeepAlivePeriod is the TCP keepalive period for the KubePrism loadbalancer.
	KubePrismKeepAlivePeriod = 30 * time.Second

	// KubePrismTCPUserTimeout is the TCP user timeout for the KubePrism loadbalancer.
	KubePrismTCPUserTimeout = 30 * time.Second

	// KubePrismHealthCheckInterval is the interval between health checks for the KubePrism loadbalancer.
	KubePrismHealthCheckInterval = 20 * time.Second

	// KubePrismHealthCheckTimeout is the timeout for health checks for the KubePrism loadbalancer.
	KubePrismHealthCheckTimeout = 15 * time.Second

	// TalosAPIDefaultCertificateValidityDuration specifies default certificate duration for Talos API generated client certificates.
	TalosAPIDefaultCertificateValidityDuration = time.Hour * 24 * 365

	// DefaultNfTablesTableName is the default name of the nftables table created by Talos.
	DefaultNfTablesTableName = "talos"

	// PodResolvConfPath is the path to the pod resolv.conf file.
	PodResolvConfPath = "/system/resolved/resolv.conf"

	// SyslogListenSocketPath is the path to the syslog socket.
	SyslogListenSocketPath = "/dev/log"

	// MinimumGOAMD64Level is the minimum x86_64 microarchitecture level required by Talos.
	MinimumGOAMD64Level = 2

	// ConsoleLogErrorSuppressThreshold is the threshold for suppressing console log errors.
	ConsoleLogErrorSuppressThreshold = 4

	// HostDNSAddress is the address of the host DNS server.
	//
	// Note: 116 = 't' and 108 = 'l' in ASCII.
	HostDNSAddress = "169.254.116.108"

	// MetalAgentModeFlagPath is the path to the file indicating if the node is running in Metal Agent mode.
	MetalAgentModeFlagPath = "/usr/local/etc/is-metal-agent"

	// ImageCachePartitionLabel is the label for the image cache partition.
	ImageCachePartitionLabel = "IMAGECACHE"

	// ImageCacheISOMountPoint is the mount point for the image cache ISO.
	ImageCacheISOMountPoint = "/system/imagecache/iso"

	// ImageCacheDiskMountPoint is the mount point for the image cache partition.
	ImageCacheDiskMountPoint = "/system/imagecache/disk"

	// RegistrydListenAddress is the address to listen on for the registryd service.
	RegistrydListenAddress = "127.0.0.1:3172"

	// KubernetesInformerDefaultResyncPeriod is the default resync period for Kubernetes informers.
	KubernetesInformerDefaultResyncPeriod = 30 * time.Second

	// UserVolumeMountPoint is the path to the volume mount point for the user volumes.
	UserVolumeMountPoint = "/var/mnt"

	// UserVolumePrefix is the prefix for the user volumes.
	UserVolumePrefix = "u-"

	// SwapVolumePrefix is the prefix for the swap volumes.
	SwapVolumePrefix = "s-"

	// PartitionLabelLength is the length of the partition label.
	//
	// See https://en.wikipedia.org/wiki/GUID_Partition_Table#Partition_entries_(LBA_2%E2%80%9333)
	PartitionLabelLength = 36
)

// See https://linux.die.net/man/3/klogctl
//
//nolint:stylecheck,revive
const (
	// SYSLOG_ACTION_SIZE_BUFFER is a named type argument to klogctl.
	//nolint:golint
	SYSLOG_ACTION_SIZE_BUFFER = 10

	// SYSLOG_ACTION_READ_ALL is a named type argument to klogctl.
	//nolint:golint
	SYSLOG_ACTION_READ_ALL = 3
)

// names of variable that can be substituted in the talos.config kernel parameter.
const (
	UUIDKey         = "uuid"
	SerialNumberKey = "serial"
	HostnameKey     = "hostname"
	MacKey          = "mac"
	CodeKey         = "code"
)

// SELinuxLabeledPath is an object used to describe overlay mounts with SELinux labels applied on creation.
type SELinuxLabeledPath struct {
	Path  string
	Label string
}

// Overlays is the set of paths to create overlay mounts for.
var Overlays = []SELinuxLabeledPath{
	{"/etc/cni", CNISELinuxLabel},
	{KubernetesConfigBaseDir, KubernetesConfigSELinuxLabel},
	{"/usr/libexec/kubernetes", KubeletPluginsSELinuxLabel},
	{"/opt", OptSELinuxLabel},
}

// DefaultDroppedCapabilities is the default set of capabilities to drop.
var DefaultDroppedCapabilities = map[string]struct{}{
	"cap_sys_boot":   {},
	"cap_sys_module": {},
}

// UdevdDroppedCapabilities is the set of capabilities to drop for udevd.
var UdevdDroppedCapabilities = map[string]struct{}{
	"cap_sys_boot": {},
}

// ValidEffects is the set of valid taint effects.
var ValidEffects = []string{
	"NoSchedule",
	"PreferNoSchedule",
	"NoExecute",
}

// OSReleaseTemplate is the template for /etc/os-release.
const OSReleaseTemplate = `NAME="{{ .Name }}"
ID={{ .ID }}
VERSION_ID={{ .Version }}
PRETTY_NAME="{{ .Name }} ({{ .Version }})"
HOME_URL="https://www.talos.dev/"
BUG_REPORT_URL="https://github.com/siderolabs/talos/issues"
VENDOR_NAME="Sidero Labs"
VENDOR_URL="https://www.siderolabs.com/"
`
