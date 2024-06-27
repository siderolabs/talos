// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

/*
Package v1alpha1 contains definition of the `v1alpha1` configuration document.

Even though the machine configuration in Talos Linux is multi-document, at the moment
this configuration document contains most of the configuration options.

It is expected that new configuration options will be added as new documents, and existing ones
migrated to their own documents.
*/
//nolint:lll,revive,stylecheck
package v1alpha1

//go:generate deepcopy-gen --go-header-file ../../../../../hack/boilerplate.txt --bounding-dirs ../v1alpha1 --output-file zz_generated.deepcopy

//docgen:jsonschema

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-blockdevice/blockdevice/util/disk"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
)

func init() {
	registry.Register("v1alpha1", func(version string) config.Document {
		return &Config{}
	})
}

// Config represents the v1alpha1 configuration file.
//
//docgen:configuration
type Config struct {
	// Decodes the contents using the specified schema.
	ConfigVersion string `yaml:"version" docgen:"{'values':['v1alpha1'],'in':'1.5'}"`

	// Enables verbose logging to the console.
	// All system container logs flow into the serial console.
	//
	// Note: Enable this option only if the serial console can handle high message throughput to avoid disrupting the Talos bootstrap flow.
	ConfigDebug *bool `yaml:"debug,omitempty" docgen:"{'in':'1.5'}"`

	// Determines whether to pull the machine config on every boot.
	ConfigPersist *bool `yaml:"persist,omitempty" docgen:"{'deprecated':'1.6','in':'1.5'}"`

	// Specifies machine specific configuration options.
	MachineConfig *MachineConfig `yaml:"machine" docgen:"{'in':'1.5'}"`

	// Specifies cluster specific configuration options.
	ClusterConfig *ClusterConfig `yaml:"cluster" docgen:"{'in':'1.5'}"`
}

var _ config.MachineConfig = (*MachineConfig)(nil)

// MachineConfig represents the machine-specific config values.
//
//docgen:configuration
type MachineConfig struct {
	// Specifies the machine's role within the cluster. The roles can be either "controlplane" or "worker".
	// The "controlplane" role hosts etcd and the Kubernetes control plane components such as API Server, Controller Manager, Scheduler.
	// The "worker" role is available for scheduling workloads.
	MachineType string `yaml:"type" docgen:"{'values':['controlplane','worker'],'in':'1.5'}"`

	// Utilizes the `token` for a machine to join the cluster's PKI.
	// A machine creates a certificate signing request (CSR) using this token and requests a certificate to be used as its identity.
	MachineToken string `yaml:"token" docgen:"{'in':'1.5'}"`

	// Represents the root certificate authority of the PKI, composed of a base64 encoded `crt` and `key`.
	MachineCA *x509.PEMEncodedCertificateAndKey `yaml:"ca,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the certificates issued by certificate authorities that are accepted in addition to the issuing `ca`, composed of a base64 encoded `crt`.
	MachineAcceptedCAs []*x509.PEMEncodedCertificate `yaml:"acceptedCAs,omitempty" docgen:"{'in':'1.7'}"`

	// Adds extra certificate subject alternative names for the machine's certificate.
	// By default, all non-loopback interface IPs are automatically added to the certificate's SANs.
	MachineCertSANs []string `yaml:"certSANs" docgen:"{'in':'1.5'}"`

	// Provides machine specific control plane configuration options.
	MachineControlPlane *MachineControlPlaneConfig `yaml:"controlPlane,omitempty" docgen:"{'in':'1.5'}"`

	// Provides additional options to the kubelet.
	MachineKubelet *KubeletConfig `yaml:"kubelet,omitempty" docgen:"{'in':'1.5'}"`

	// Provides static pod definitions to be run by the kubelet directly, bypassing the kube-apiserver.
	// Static pods can be used to run components which should be started before the Kubernetes control plane is up.
	// Talos doesn't validate the pod definition.
	// Updates to this field can be applied without a reboot.
	// See https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/.
	MachinePods []Unstructured `yaml:"pods,omitempty" docgen:"{'in':'1.5'}"`

	// Provides machine specific network configuration options.
	MachineNetwork *NetworkConfig `yaml:"network,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the partitioning, formatting, and mounting of additional disks.
	// Since the rootfs is read-only with the exception of `/var`, mounts are only valid if they are under `/var`.
	// Note that the partitioning and formatting is done only once, if and only if no existing XFS partitions are found.
	// If `size:` is omitted, the partition is sized to occupy the full disk.
	MachineDisks []*MachineDisk `yaml:"disks,omitempty" docgen:"{'in':'1.5'}"`

	// Provides instructions for installations.
	// Note that this configuration section gets silently ignored by Talos images that are considered pre-installed.
	// To ensure Talos installs according to the provided configuration, boot Talos with ISO or PXE-booted.
	MachineInstall *InstallConfig `yaml:"install,omitempty" docgen:"{'in':'1.5'}"`

	// Allows the addition of user specified files.
	// The value of `op` can be `create`, `overwrite`, or `append`.
	// In the case of `create`, `path` must not exist.
	// In the case of `overwrite`, and `append`, `path` must be a valid file.
	// If an `op` value of `append` is used, the existing file will be appended.
	// Note that the file contents are not required to be base64 encoded.
	MachineFiles []*MachineFile `yaml:"files,omitempty" docgen:"{'in':'1.5'}"`

	// Adds environment variables.
	// All environment variables are set on PID 1 in addition to every service.
	MachineEnv Env `yaml:"env,omitempty" docgen:"{'values':['GRPC_GO_LOG_VERBOSITY_LEVEL','GRPC_GO_LOG_SEVERITY_LEVEL','http_proxy','no_proxy'],'in':'1.5'}"`

	// Configures the machine's time settings.
	MachineTime *TimeConfig `yaml:"time,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the machine's sysctls.
	MachineSysctls map[string]string `yaml:"sysctls,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the machine's sysfs.
	MachineSysfs map[string]string `yaml:"sysfs,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the machine's container image registry mirrors.
	// Automatically generates matching CRI configuration for registry mirrors.
	// The `mirrors` section allows to redirect requests for images to a non-default registry,
	// which might be a local registry or a caching mirror.
	// The `config` section provides a way to authenticate to the registry with TLS client
	// identity, provide registry CA, or authentication information.
	// Authentication information has same meaning with the corresponding field in [`.docker/config.json`](https://docs.docker.com/engine/api/v1.41/#section/Authentication).
	// See also matching configuration for [CRI containerd plugin](https://github.com/containerd/cri/blob/master/docs/registry.md).
	MachineRegistries RegistriesConfig `yaml:"registries,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the machine system disk encryption.
	// Defines each system partition encryption parameters.
	MachineSystemDiskEncryption *SystemDiskEncryptionConfig `yaml:"systemDiskEncryption,omitempty" docgen:"{'in':'1.5'}"`

	// Describes individual Talos features that can be switched on or off.
	MachineFeatures *FeaturesConfig `yaml:"features,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the udev system.
	MachineUdev *UdevConfig `yaml:"udev,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the logging system.
	MachineLogging *LoggingConfig `yaml:"logging,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the kernel.
	MachineKernel *KernelConfig `yaml:"kernel,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the seccomp profiles for the machine.
	MachineSeccompProfiles []*MachineSeccompProfile `yaml:"seccompProfiles,omitempty" docgen:"{'in':'1.5'}" talos:"omitonlyifnil"`

	// Configures the node labels for the machine.
	MachineNodeLabels map[string]string `yaml:"nodeLabels,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the node taints for the machine.
	MachineNodeTaints map[string]string `yaml:"nodeTaints,omitempty" docgen:"{'optional':true, 'in':'1.6'}"`
}

// MachineSeccompProfile defines seccomp profiles for the machine.
//
//docgen:configuration
type MachineSeccompProfile struct {
	// Provides the file name of the seccomp profile.
	MachineSeccompProfileName string `yaml:"name" docgen:"{'in':'1.5'}"`

	// Provides the seccomp profile.
	MachineSeccompProfileValue Unstructured `yaml:"value" docgen:"{'in':'1.5'}"`
}

var (
	_ config.ClusterConfig  = (*ClusterConfig)(nil)
	_ config.ClusterNetwork = (*ClusterConfig)(nil)
	_ config.Token          = (*ClusterConfig)(nil)
)

// ClusterConfig represents the cluster-wide config values.
//
//docgen:configuration
type ClusterConfig struct {
	// Specifies a globally unique identifier for this cluster (base64 encoded random 32 bytes).
	ClusterID string `yaml:"id,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a shared secret of the cluster (base64 encoded random 32 bytes).
	// The secret should never be sent over the network.
	ClusterSecret string `yaml:"secret,omitempty" docgen:"{'in':'1.5'}"`

	// Provides control plane specific configuration options.
	ControlPlane *ControlPlaneConfig `yaml:"controlPlane" docgen:"{'in':'1.5'}"`

	// Specifies the cluster's name.
	ClusterName string `yaml:"clusterName,omitempty" docgen:"{'in':'1.5'}"`

	// Provides cluster specific network configuration options.
	ClusterNetwork *ClusterNetworkConfig `yaml:"network,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the bootstrap token used to join the cluster.
	BootstrapToken string `yaml:"token,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a key for the encryption of secret data at rest using AESCBC.
	ClusterAESCBCEncryptionSecret string `yaml:"aescbcEncryptionSecret,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a key for the encryption of secret data at rest using secretbox.
	// Secretbox has precedence over AESCBC.
	ClusterSecretboxEncryptionSecret string `yaml:"secretboxEncryptionSecret,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the root certificate authority used by Kubernetes.
	ClusterCA *x509.PEMEncodedCertificateAndKey `yaml:"ca,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the certificates issued by certificate authorities used by Kubernetes that are accepted in addition to the issuing `ca`, composed of a base64 encoded `crt`.
	ClusterAcceptedCAs []*x509.PEMEncodedCertificate `yaml:"acceptedCAs,omitempty" docgen:"{'in':'1.7'}"`

	// Specifies the aggregator certificate authority used by Kubernetes for front-proxy certificate generation.
	ClusterAggregatorCA *x509.PEMEncodedCertificateAndKey `yaml:"aggregatorCA,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the private key for service account token generation.
	ClusterServiceAccount *x509.PEMEncodedKey `yaml:"serviceAccount,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies API server specific configuration options.
	APIServerConfig *APIServerConfig `yaml:"apiServer,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies controller manager server specific configuration options.
	ControllerManagerConfig *ControllerManagerConfig `yaml:"controllerManager,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies kube-proxy server-specific configuration options.
	ProxyConfig *ProxyConfig `yaml:"proxy,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies scheduler server specific configuration options.
	SchedulerConfig *SchedulerConfig `yaml:"scheduler,omitempty" docgen:"{'in':'1.5'}"`

	// Configures cluster member discovery.
	ClusterDiscoveryConfig *ClusterDiscoveryConfig `yaml:"discovery,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies etcd specific configuration options.
	EtcdConfig *EtcdConfig `yaml:"etcd,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies Core DNS specific configuration options.
	CoreDNSConfig *CoreDNS `yaml:"coreDNS,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies external cloud provider configuration.
	ExternalCloudProviderConfig *ExternalCloudProviderConfig `yaml:"externalCloudProvider,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a list of urls that point to additional manifests.
	ExtraManifests []string `yaml:"extraManifests,omitempty" docgen:"{'in':'1.5'}" talos:"omitonlyifnil"`

	// Specifies a map of key value pairs for fetching the extraManifests.
	ExtraManifestHeaders map[string]string `yaml:"extraManifestHeaders,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a list of inline Kubernetes manifests.
	ClusterInlineManifests ClusterInlineManifests `yaml:"inlineManifests,omitempty" docgen:"{'in':'1.5'}" talos:"omitonlyifnil"`

	// Specifies settings for admin kubeconfig generation.
	AdminKubeconfigConfig *AdminKubeconfigConfig `yaml:"adminKubeconfig,omitempty" docgen:"{'in':'1.5'}"`

	// Allows running workload on control-plane nodes.
	AllowSchedulingOnMasters *bool `yaml:"allowSchedulingOnMasters,omitempty" docgen:"{'deprecated':'1.6','in':'1.5'}"`

	// Allows running workload on control-plane nodes.
	AllowSchedulingOnControlPlanes *bool `yaml:"allowSchedulingOnControlPlanes,omitempty" docgen:"{'in':'1.5'}"`
}

// LinuxIDMapping represents the Linux ID mapping.
type LinuxIDMapping struct {
	//   description: |
	//     ContainerID is the starting UID/GID in the container.
	ContainerID uint32 `yaml:"containerID"`
	//   description: |
	//     HostID is the starting UID/GID on the host to be mapped to 'ContainerID'.
	HostID uint32 `yaml:"hostID"`
	//   description: |
	//     Size is the number of IDs to be mapped.
	Size uint32 `yaml:"size"`
}

// ExtraMount wraps OCI Mount specification.
type ExtraMount struct {
	//   description: |
	//     Destination is the absolute path where the mount will be placed in the container.
	Destination string `yaml:"destination"`
	//   description: |
	//     Type specifies the mount kind.
	Type string `yaml:"type,omitempty"`
	//   description: |
	//     Source specifies the source path of the mount.
	Source string `yaml:"source,omitempty"`
	//   description: |
	//     Options are fstab style mount options.
	Options []string `yaml:"options,omitempty"`

	//   description: |
	//     UID/GID mappings used for changing file owners w/o calling chown, fs should support it.
	//
	//     Every mount point could have its own mapping.
	UIDMappings []LinuxIDMapping `yaml:"uidMappings,omitempty"`
	//   description: |
	//     UID/GID mappings used for changing file owners w/o calling chown, fs should support it.
	//
	//     Every mount point could have its own mapping.
	GIDMappings []LinuxIDMapping `yaml:"gidMappings,omitempty"`
}

// MachineControlPlaneConfig defines machine specific configuration options.
//
//docgen:configuration
type MachineControlPlaneConfig struct {
	// Specifies controller manager machine specific configuration options.
	MachineControllerManager *MachineControllerManagerConfig `yaml:"controllerManager,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies scheduler machine specific configuration options.
	MachineScheduler *MachineSchedulerConfig `yaml:"scheduler,omitempty" docgen:"{'in':'1.5'}"`
}

// MachineControllerManagerConfig represents the machine specific ControllerManager config values.
//
//docgen:configuration
type MachineControllerManagerConfig struct {
	// Specifies whether to disable the kube-controller-manager on the node.
	MachineControllerManagerDisabled *bool `yaml:"disabled,omitempty" docgen:"{'in':'1.5'}"`
}

// MachineSchedulerConfig represents the machine specific Scheduler config values.
//
//docgen:configuration
type MachineSchedulerConfig struct {
	// Specifies whether to disable the kube-scheduler on the node.
	MachineSchedulerDisabled *bool `yaml:"disabled,omitempty" docgen:"{'in':'1.5'}"`
}

// KubeletConfig represents the kubelet config values.
//
//docgen:configuration
type KubeletConfig struct {
	// Specifies an optional reference to an alternative kubelet image.
	KubeletImage string `yaml:"image,omitempty" docgen:"{'optional':true,'in':'1.5'}"`

	// Specifies an optional reference to an alternative kubelet clusterDNS IP list.
	KubeletClusterDNS []string `yaml:"clusterDNS,omitempty" docgen:"{'optional':true,'in':'1.5'}"`

	// Provides additional flags to the kubelet.
	KubeletExtraArgs map[string]string `yaml:"extraArgs,omitempty" docgen:"{'in':'1.5'}"`

	// Adds additional mounts to the kubelet container.
	KubeletExtraMounts []ExtraMount `yaml:"extraMounts,omitempty" docgen:"{'in':'1.5'}"`

	// Provides kubelet configuration overrides.
	KubeletExtraConfig Unstructured `yaml:"extraConfig,omitempty" docgen:"{'in':'1.5'}"`

	// Provide kubelet credential configuration.
	KubeletCredentialProviderConfig Unstructured `yaml:"credentialProviderConfig,omitempty" docgen:"{'in':'1.6'}"`

	// Enables the container runtime default Seccomp profile.
	KubeletDefaultRuntimeSeccompProfileEnabled *bool `yaml:"defaultRuntimeSeccompProfileEnabled,omitempty" docgen:"{'in':'1.5'}"`

	// Forces the kubelet to use the node FQDN for registration.
	KubeletRegisterWithFQDN *bool `yaml:"registerWithFQDN,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the `--node-ip` flag for the kubelet.
	KubeletNodeIP *KubeletNodeIPConfig `yaml:"nodeIP,omitempty" docgen:"{'in':'1.5'}"`

	// Runs the kubelet without registering with the apiserver.
	KubeletSkipNodeRegistration *bool `yaml:"skipNodeRegistration,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the kubelet to get static pod manifests from the /etc/kubernetes/manifests directory.
	KubeletDisableManifestsDirectory *bool `yaml:"disableManifestsDirectory,omitempty" docgen:"{'in':'1.5'}"`
}

// KubeletNodeIPConfig represents the kubelet node IP configuration.
//
//docgen:configuration
type KubeletNodeIPConfig struct {
	// Configures the networks to pick kubelet node IP from.
	KubeletNodeIPValidSubnets []string `yaml:"validSubnets,omitempty" docgen:"{'in':'1.5'}"`
}

// NetworkConfig represents the machine's networking config values.
//
//docgen:configuration
type NetworkConfig struct {
	// Specifies a static hostname for the machine.
	NetworkHostname string `yaml:"hostname,omitempty" docgen:"{'in':'1.5'}"`

	// Defines the network interface configuration.
	NetworkInterfaces NetworkDeviceList `yaml:"interfaces,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies static nameservers for the machine.
	NameServers []string `yaml:"nameservers,omitempty" docgen:"{'in':'1.5'}"`

	// Allows adding extra entries to the `/etc/hosts` file.
	ExtraHostEntries []*ExtraHost `yaml:"extraHostEntries,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the KubeSpan feature.
	NetworkKubeSpan *NetworkKubeSpan `yaml:"kubespan,omitempty" docgen:"{'in':'1.5'}"`

	// Disables generating a default search domain in /etc/resolv.conf based on the machine hostname.
	NetworkDisableSearchDomain *bool `yaml:"disableSearchDomain,omitempty" docgen:"{'in':'1.5'}"`
}

// NetworkDeviceList is a list of *Device structures with overridden merge process.
type NetworkDeviceList []*Device

// Merge the network interface configuration intelligently.
func (devices *NetworkDeviceList) Merge(other interface{}) error {
	otherDevices, ok := other.(NetworkDeviceList)
	if !ok {
		return fmt.Errorf("unexpected type for device merge %T", other)
	}

	for _, device := range otherDevices {
		if err := devices.mergeDevice(device); err != nil {
			return err
		}
	}

	return nil
}

func (devices *NetworkDeviceList) mergeDevice(device *Device) error {
	var existing *Device

	switch {
	case device.DeviceInterface != "":
		for _, d := range *devices {
			if d.DeviceInterface == device.DeviceInterface {
				existing = d

				break
			}
		}
	case device.DeviceSelector != nil:
		for _, d := range *devices {
			if d.DeviceSelector != nil && *d.DeviceSelector == *device.DeviceSelector {
				existing = d

				break
			}
		}
	}

	if existing != nil {
		return merge.Merge(existing, device)
	}

	*devices = append(*devices, device)

	return nil
}

// InstallConfig represents the installation options for preparing a node.
//
//docgen:configuration
type InstallConfig struct {
	// Specifies the disk used for installations.
	InstallDisk string `yaml:"disk,omitempty" docgen:"{'in':'1.5'}"`

	// Allows for disk lookup using disk attributes such as model, size, serial, etc.
	InstallDiskSelector *InstallDiskSelector `yaml:"diskSelector,omitempty" docgen:"{'in':'1.5'}"`

	// Supplies extra kernel arguments via the bootloader.
	InstallExtraKernelArgs []string `yaml:"extraKernelArgs,omitempty" docgen:"{'in':'1.5'}"`

	// Supplies the image used for the installation.
	InstallImage string `yaml:"image,omitempty" docgen:"{'in':'1.5'}"`

	// Supplies additional system extension images to install on top of the base Talos image.
	InstallExtensions []InstallExtensionConfig `yaml:"extensions,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies if a bootloader should be installed.
	InstallBootloader *bool `yaml:"bootloader,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies if the installation disk should be wiped at installation time.
	InstallWipe *bool `yaml:"wipe" docgen:"{'in':'1.5'}"`

	// Specifies if the MBR partition should be marked as bootable (active).
	InstallLegacyBIOSSupport *bool `yaml:"legacyBIOSSupport,omitempty" docgen:"{'in':'1.5'}"`
}

// InstallDiskSizeMatcher disk size condition parser.
type InstallDiskSizeMatcher struct {
	MatchData InstallDiskSizeMatchData
	condition string
}

// MarshalYAML is a custom marshaller for `InstallDiskSizeMatcher`.
func (m *InstallDiskSizeMatcher) MarshalYAML() (interface{}, error) {
	return m.condition, nil
}

// UnmarshalYAML is a custom unmarshaller for `InstallDiskSizeMatcher`.
func (m *InstallDiskSizeMatcher) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&m.condition); err != nil {
		return err
	}

	m.condition = strings.TrimSpace(m.condition)

	re := regexp.MustCompile(`(>=|<=|>|<|==)?\b*(.*)$`)

	parts := re.FindStringSubmatch(m.condition)
	if len(parts) < 2 {
		return fmt.Errorf("failed to parse the condition: expected [>=|<=|>|<|==]<size>[units], got %s", m.condition)
	}

	var op string

	switch parts[1] {
	case ">=", "<=", ">", "<", "", "==":
		op = parts[1]
	default:
		return fmt.Errorf("unknown binary operator %s", parts[1])
	}

	size, err := humanize.ParseBytes(strings.TrimSpace(parts[2]))
	if err != nil {
		return fmt.Errorf("failed to parse disk size %s: %s", parts[2], err)
	}

	m.MatchData = InstallDiskSizeMatchData{
		Op:   op,
		Size: size,
	}

	return nil
}

// Matcher is a method that can handle some custom disk matching logic.
func (m *InstallDiskSizeMatcher) Matcher(d *disk.Disk) bool {
	return m.MatchData.Compare(d)
}

// InstallDiskSizeMatchData contains data for comparison - Op and Size.
type InstallDiskSizeMatchData struct {
	Op   string
	Size uint64
}

// Compare is the method to compare disk size.
func (in *InstallDiskSizeMatchData) Compare(d *disk.Disk) bool {
	switch in.Op {
	case ">=":
		return d.Size >= in.Size
	case "<=":
		return d.Size <= in.Size
	case ">":
		return d.Size > in.Size
	case "<":
		return d.Size < in.Size
	case "":
		fallthrough
	case "==":
		return d.Size == in.Size
	default:
		return false
	}
}

// InstallDiskType custom type for disk type selector.
type InstallDiskType disk.Type

// MarshalYAML is a custom marshaller for `InstallDiskSizeMatcher`.
func (it InstallDiskType) MarshalYAML() (interface{}, error) {
	return disk.Type(it).String(), nil
}

// UnmarshalYAML is a custom unmarshaler for `InstallDiskType`.
func (it *InstallDiskType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var (
		t   string
		err error
	)

	if err = unmarshal(&t); err != nil {
		return err
	}

	if dt, err := disk.ParseType(t); err == nil {
		*it = InstallDiskType(dt)
	} else {
		return err
	}

	return nil
}

// InstallDiskSelector represents disk query parameters for the install disk lookup.
//
//docgen:configuration
type InstallDiskSelector struct {
	// Specifies the disk size.
	Size *InstallDiskSizeMatcher `yaml:"size,omitempty" docgen:"{'in':'1.5'}"`

	// Refers to the disk name `/sys/block/<dev>/device/name`.
	Name string `yaml:"name,omitempty" docgen:"{'in':'1.5'}"`

	// Refers to the disk model `/sys/block/<dev>/device/model`.
	Model string `yaml:"model,omitempty" docgen:"{'in':'1.5'}"`

	// Refers to the disk serial number `/sys/block/<dev>/serial`.
	Serial string `yaml:"serial,omitempty" docgen:"{'in':'1.5'}"`

	// Refers to the disk modalias `/sys/block/<dev>/device/modalias`.
	Modalias string `yaml:"modalias,omitempty" docgen:"{'in':'1.5'}"`

	// Refers to the disk UUID `/sys/block/<dev>/uuid`.
	UUID string `yaml:"uuid,omitempty" docgen:"{'in':'1.5'}"`

	// Refers to the disk WWID `/sys/block/<dev>/wwid`.
	WWID string `yaml:"wwid,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the disk type.
	Type InstallDiskType `yaml:"type,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the disk bus path.
	BusPath string `yaml:"busPath,omitempty" docgen:"{'in':'1.5'}"`
}

// InstallExtensionConfig represents a configuration for a system extension.
//
//docgen:configuration
type InstallExtensionConfig struct {
	// Specifies the system extension image.
	ExtensionImage string `yaml:"image" docgen:"{'in':'1.5'}"`
}

// TimeConfig represents the options for configuring time on a machine.
//
//docgen:configuration
type TimeConfig struct {
	// Indicates if the time service is disabled for the machine.
	TimeDisabled *bool `yaml:"disabled,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies time (NTP) servers to use for setting the system time.
	TimeServers []string `yaml:"servers,omitempty" docgen:"{'in':'1.5', 'default': 'time.cloudflare.com'}"`

	// Specifies the timeout when the node time is considered to be in sync unlocking the boot sequence.
	TimeBootTimeout time.Duration `yaml:"bootTimeout,omitempty" docgen:"{'in':'1.5'}"`
}

// RegistriesConfig represents the image pull options.
//
//docgen:configuration
type RegistriesConfig struct {
	// Provides mirror configuration for each registry host namespace.
	RegistryMirrors map[string]*RegistryMirrorConfig `yaml:"mirrors,omitempty" docgen:"{'in':'1.5'}"`

	// Provides TLS & auth configuration for HTTPS image registries.
	RegistryConfig map[string]*RegistryConfig `yaml:"config,omitempty" docgen:"{'in':'1.5'}"`
}

// CoreDNS represents the CoreDNS config values.
//
//docgen:configuration
type CoreDNS struct {
	// Indicates if coredns deployment is disabled on cluster bootstrap.
	CoreDNSDisabled *bool `yaml:"disabled,omitempty" docgen:"{'in':'1.5'}"`

	// Overrides the default coredns image.
	CoreDNSImage string `yaml:"image,omitempty" docgen:"{'in':'1.5'}"`
}

// Endpoint represents the endpoint URL parsed out of the machine config.
type Endpoint struct {
	*url.URL
}

// UnmarshalYAML is a custom unmarshaller for `Endpoint`.
func (e *Endpoint) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var endpoint string

	if err := unmarshal(&endpoint); err != nil {
		return err
	}

	url, err := url.Parse(endpoint)
	if err != nil {
		return err
	}

	*e = Endpoint{url}

	return nil
}

// MarshalYAML is a custom marshaller for `Endpoint`.
func (e *Endpoint) MarshalYAML() (interface{}, error) {
	return e.URL.String(), nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (e *Endpoint) DeepCopyInto(out *Endpoint) {
	*out = *e

	if e.URL != nil {
		in, out := &e.URL, &out.URL
		*out = new(url.URL)
		*out = *in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Endpoint.
func (e *Endpoint) DeepCopy() *Endpoint {
	if e == nil {
		return nil
	}

	out := new(Endpoint)
	e.DeepCopyInto(out)

	return out
}

// ControlPlaneConfig represents the control plane configuration options.
//
//docgen:configuration
type ControlPlaneConfig struct {
	// Specifies the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
	Endpoint *Endpoint `yaml:"endpoint" docgen:"{'in':'1.5'}"`

	// Specifies the port that the API server listens on internally.
	LocalAPIServerPort int `yaml:"localAPIServerPort,omitempty" docgen:"{'in':'1.5'}"`
}

var _ config.APIServer = (*APIServerConfig)(nil)

// APIServerConfig represents the kube apiserver configuration options.
//
//docgen:configuration
type APIServerConfig struct {
	// Specifies the container image used in the API server manifest.
	ContainerImage string `yaml:"image,omitempty" docgen:"{'in':'1.5'}"`

	// Provides extra arguments to the API server.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies extra volumes to mount to the API server static pod.
	ExtraVolumesConfig []VolumeMountConfig `yaml:"extraVolumes,omitempty" docgen:"{'in':'1.5'}"`

	// Allows for the addition of environment variables for the control plane component.
	EnvConfig Env `yaml:"env,omitempty" docgen:"{'in':'1.5'}"`

	// Provides extra certificate subject alternative names for the API server's certificate.
	CertSANs []string `yaml:"certSANs,omitempty" docgen:"{'in':'1.5'}"`

	// Indicates if PodSecurityPolicy is disabled in the API server and default manifests.
	DisablePodSecurityPolicyConfig *bool `yaml:"disablePodSecurityPolicy,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the API server admission plugins.
	AdmissionControlConfig AdmissionPluginConfigList `yaml:"admissionControl,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the API server audit policy.
	AuditPolicyConfig Unstructured `yaml:"auditPolicy,omitempty" merge:"replace" docgen:"{'in':'1.5'}"`

	// Configures the API server resources.
	ResourcesConfig *ResourcesConfig `yaml:"resources,omitempty" docgen:"{'in':'1.5'}"`
}

// AdmissionPluginConfigList represents the admission plugin configuration list.
type AdmissionPluginConfigList []*AdmissionPluginConfig

// Merge the admission plugin configuration intelligently.
func (configs *AdmissionPluginConfigList) Merge(other interface{}) error {
	otherConfigs, ok := other.(AdmissionPluginConfigList)
	if !ok {
		return fmt.Errorf("unexpected type for device merge %T", other)
	}

	for _, config := range otherConfigs {
		if err := configs.mergeConfig(config); err != nil {
			return err
		}
	}

	return nil
}

func (configs *AdmissionPluginConfigList) mergeConfig(config *AdmissionPluginConfig) error {
	var existing *AdmissionPluginConfig

	for _, c := range *configs {
		if c.PluginName == config.PluginName {
			existing = c

			break
		}
	}

	if existing != nil {
		return merge.Merge(existing, config)
	}

	*configs = append(*configs, config)

	return nil
}

// AdmissionPluginConfig represents the API server admission plugin configuration.
//
//docgen:configuration
type AdmissionPluginConfig struct {
	// Specifies the name of the admission controller.
	PluginName string `yaml:"name" docgen:"{'in':'1.5'}"`

	// Specifies an embedded configuration object to be used as the plugin's configuration.
	PluginConfiguration Unstructured `yaml:"configuration" docgen:"{'in':'1.5'}"`
}

var _ config.ControllerManager = (*ControllerManagerConfig)(nil)

// ControllerManagerConfig represents the kube controller manager configuration options.
//
//docgen:configuration
type ControllerManagerConfig struct {
	// Specifies the container image used in the controller manager manifest.
	ContainerImage string `yaml:"image,omitempty" docgen:"{'in':'1.5'}"`

	// Provides extra arguments to the controller manager.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty" docgen:"{'in':'1.5'}"`

	// Lists extra volumes to mount to the controller manager static pod.
	ExtraVolumesConfig []VolumeMountConfig `yaml:"extraVolumes,omitempty" docgen:"{'in':'1.5'}"`

	// Allows the addition of environment variables for the control plane component.
	EnvConfig Env `yaml:"env,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the controller manager resources.
	ResourcesConfig *ResourcesConfig `yaml:"resources,omitempty" docgen:"{'in':'1.5'}"`
}

// ProxyConfig represents the kube proxy configuration options.
//
//docgen:configuration
type ProxyConfig struct {
	// Indicates if the kube-proxy deployment on cluster bootstrap is disabled.
	Disabled *bool `yaml:"disabled,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the container image used in the kube-proxy manifest.
	ContainerImage string `yaml:"image,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the proxy mode of kube-proxy. The default is 'iptables'.
	ModeConfig string `yaml:"mode,omitempty" docgen:"{'in':'1.5'}"`

	// Provides extra arguments to kube-proxy.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty" docgen:"{'in':'1.5'}"`
}

// SchedulerConfig represents the kube scheduler configuration options.
//
//docgen:configuration
type SchedulerConfig struct {
	// Specifies the container image used in the scheduler manifest.
	ContainerImage string `yaml:"image,omitempty" docgen:"{'in':'1.5'}"`

	// Provides extra arguments to the scheduler.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty" docgen:"{'in':'1.5'}"`

	// Lists extra volumes to mount to the scheduler static pod.
	ExtraVolumesConfig []VolumeMountConfig `yaml:"extraVolumes,omitempty" docgen:"{'in':'1.5'}"`

	// Allows the addition of environment variables for the control plane component.
	EnvConfig Env `yaml:"env,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the scheduler resources.
	ResourcesConfig *ResourcesConfig `yaml:"resources,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies custom kube-scheduler configuration.
	SchedulerConfig Unstructured `yaml:"config,omitempty" docgen:"{'in':'1.6'}"`
}

// Represents the etcd configuration options.
//
//docgen:configuration
type EtcdConfig struct {
	// Specifies the container image for the etcd service.
	ContainerImage string `yaml:"image,omitempty" docgen:"{'in':'1.5'}"`

	// Denotes the root certificate authority of the PKI, comprised of a base64 encoded `crt` and `key`.
	RootCA *x509.PEMEncodedCertificateAndKey `yaml:"ca" docgen:"{'in':'1.5'}"`

	// Defines additional arguments for etcd, with certain args prohibited.
	EtcdExtraArgs map[string]string `yaml:"extraArgs,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the network from which to select etcd advertised IP.
	EtcdSubnet string `yaml:"subnet,omitempty" docgen:"{'in':'1.5', 'deprecated':'1.6'}"`

	// Configures the networks from which to select etcd advertised IP.
	EtcdAdvertisedSubnets []string `yaml:"advertisedSubnets,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the networks for etcd to listen for peer and client connections.
	// If not specified, defaults are applied based on `advertisedSubnets`.
	EtcdListenSubnets []string `yaml:"listenSubnets,omitempty" docgen:"{'in':'1.5'}"`
}

// ClusterNetworkConfig represents kube networking configuration options.
//
//docgen:configuration
type ClusterNetworkConfig struct {
	// Specifies the CNI used.
	CNI *CNIConfig `yaml:"cni,omitempty" docgen:"{'in':'1.5'}"`

	// Defines the domain used by Kubernetes DNS.
	DNSDomain string `yaml:"dnsDomain" docgen:"{'in':'1.5'}"`

	// Indicates the pod subnet CIDR.
	PodSubnet []string `yaml:"podSubnets" merge:"replace" docgen:"{'in':'1.5'}"`

	// Indicates the service subnet CIDR.
	ServiceSubnet []string `yaml:"serviceSubnets" merge:"replace" docgen:"{'in':'1.5'}"`
}

// CNIConfig represents the CNI configuration options.
//
//docgen:configuration
type CNIConfig struct {
	// Specifies the name of CNI to use.
	CNIName string `yaml:"name,omitempty" docgen:"{'in':'1.5'}"`

	// Lists URLs containing manifests to apply for the CNI.
	CNIUrls []string `yaml:"urls,omitempty" docgen:"{'in':'1.5'}"`

	// Flannel configuration options.
	CNIFlannel *FlannelCNIConfig `yaml:"flannel,omitempty" docgen:"{'in':'1.6'}"`
}

// FlannelCNIConfig represents the Flannel CNI configuration options.
type FlannelCNIConfig struct {
	// Extra arguments for `flanneld`.
	FlanneldExtraArgs []string `yaml:"extraArgs,omitempty" docgen:"{'in':'1.6'}"`
}

// ExternalCloudProviderConfig contains external cloud provider configuration.
//
//docgen:configuration
type ExternalCloudProviderConfig struct {
	// Indicates if the external cloud provider is enabled.
	ExternalEnabled *bool `yaml:"enabled,omitempty" docgen:"{'in':'1.5'}"`

	// Lists URLs that point to additional manifests for an external cloud provider.
	ExternalManifests []string `yaml:"manifests,omitempty" docgen:"{'in':'1.5'}"`
}

// AdminKubeconfigConfig contains admin kubeconfig settings.
//
//docgen:configuration
type AdminKubeconfigConfig struct {
	// Specifies the admin kubeconfig certificate lifetime.
	AdminKubeconfigCertLifetime time.Duration `yaml:"certLifetime,omitempty" docgen:"{'in':'1.5'}"`
}

// MachineDisk represents the options available for partitioning, formatting, and
// mounting extra disks.
//
//docgen:configuration
type MachineDisk struct {
	// Specifies the name of the disk to use.
	DeviceName string `yaml:"device,omitempty" docgen:"{'in':'1.5'}"`

	// Lists partitions to create on the disk.
	DiskPartitions []*DiskPartition `yaml:"partitions,omitempty" docgen:"{'in':'1.5'}"`
}

// DiskSize partition size in bytes.
type DiskSize uint64

// MarshalYAML write as human readable string.
func (ds DiskSize) MarshalYAML() (interface{}, error) {
	if ds%DiskSize(1000) == 0 {
		bytesString := humanize.Bytes(uint64(ds))
		// ensure that stringifying bytes as human readable string
		// doesn't lose precision
		parsed, err := humanize.ParseBytes(bytesString)
		if err == nil && parsed == uint64(ds) {
			return bytesString, nil
		}
	}

	return uint64(ds), nil
}

// UnmarshalYAML read from human readable string.
func (ds *DiskSize) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var size string

	if err := unmarshal(&size); err != nil {
		return err
	}

	s, err := humanize.ParseBytes(size)
	if err != nil {
		return err
	}

	*ds = DiskSize(s)

	return nil
}

// DiskPartition represents the options for a disk partition.
//
//docgen:configuration
type DiskPartition struct {
	// Specifies the size of the partition.
	DiskSize DiskSize `yaml:"size,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies where to mount the partition.
	DiskMountPoint string `yaml:"mountpoint,omitempty" docgen:"{'in':'1.5'}"`
}

// EncryptionConfig represents partition encryption settings.
//
//docgen:configuration
type EncryptionConfig struct {
	// Specifies the encryption provider to use.
	EncryptionProvider string `yaml:"provider" docgen:"{'in':'1.5'}"`

	// Defines the encryption keys generation and storage method.
	EncryptionKeys []*EncryptionKey `yaml:"keys" docgen:"{'in':'1.5'}"`

	// Specifies the cipher kind to use for the encryption.
	EncryptionCipher string `yaml:"cipher,omitempty" docgen:"{'in':'1.5'}"`

	// Defines the encryption key length.
	EncryptionKeySize uint `yaml:"keySize,omitempty" docgen:"{'in':'1.5'}"`

	// Defines the encryption sector size.
	EncryptionBlockSize uint64 `yaml:"blockSize,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies additional --perf parameters for the LUKS2 encryption.
	EncryptionPerfOptions []string `yaml:"options,omitempty" docgen:"{'in':'1.5'}"`
}

// EncryptionKey represents configuration for disk encryption key.
//
//docgen:configuration
type EncryptionKey struct {
	// Specifies the key which value is stored in the configuration file.
	KeyStatic *EncryptionKeyStatic `yaml:"static,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies deterministically generated key from the node UUID and PartitionLabel.
	KeyNodeID *EncryptionKeyNodeID `yaml:"nodeID,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies KMS managed encryption key.
	KeyKMS *EncryptionKeyKMS `yaml:"kms,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies key slot number for LUKS2 encryption.
	KeySlot int `yaml:"slot" docgen:"{'in':'1.5'}"`

	// Specifies if TPM based disk encryption is enabled.
	KeyTPM *EncryptionKeyTPM `yaml:"tpm,omitempty" docgen:"{'in':'1.5'}"`
}

// EncryptionKeyStatic represents throw away key type.
//
//docgen:configuration
type EncryptionKeyStatic struct {
	// Defines the static passphrase value.
	KeyData string `yaml:"passphrase,omitempty" docgen:"{'in':'1.5'}"`
}

// EncryptionKeyKMS represents a key that is generated and then sealed/unsealed by the KMS server.
type EncryptionKeyKMS struct {
	// Specifies the KMS endpoint to Seal/Unseal the key.
	KMSEndpoint string `yaml:"endpoint" docgen:"{'in':'1.5'}"`
}

// EncryptionKeyTPM represents a key that is generated and then sealed/unsealed by the TPM.
type EncryptionKeyTPM struct{}

// EncryptionKeyNodeID represents deterministically generated key from the node UUID and PartitionLabel.
type EncryptionKeyNodeID struct{}

// Env represents a set of environment variables.
type Env = map[string]string

// ResourcesConfig represents the pod resources.
//
//docgen:configuration
type ResourcesConfig struct {
	// Configures the reserved cpu/memory resources.
	Requests Unstructured `yaml:"requests,omitempty" docgen:"{'in':'1.5'}"`

	// Configures the maximum cpu/memory resources a container can use.
	Limits Unstructured `yaml:"limits,omitempty" docgen:"{'in':'1.5'}"`
}

// FileMode represents file's permissions.
type FileMode os.FileMode

// String convert file mode to octal string.
func (fm FileMode) String() string {
	return "0o" + strconv.FormatUint(uint64(fm), 8)
}

// MarshalYAML encodes as an octal value.
func (fm FileMode) MarshalYAML() (interface{}, error) {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: fm.String(),
	}, nil
}

// MachineFile represents a file to write to disk.
//
//docgen:configuration
type MachineFile struct {
	// Specifies the contents of the file.
	FileContent string `yaml:"content" docgen:"{'in':'1.5'}"`

	// Specifies the file's permissions in octal.
	FilePermissions FileMode `yaml:"permissions" docgen:"{'in':'1.5'}"`

	// Specifies the path of the file.
	FilePath string `yaml:"path" docgen:"{'in':'1.5'}"`

	// Specifies the operation to use.
	FileOp string `yaml:"op" docgen:"{'in':'1.5'}"`
}

// ExtraHost represents a host entry in /etc/hosts.
//
//docgen:configuration
type ExtraHost struct {
	// Specifies the IP of the host.
	HostIP string `yaml:"ip" docgen:"{'in':'1.5'}"`

	// Specifies the host alias.
	HostAliases []string `yaml:"aliases" docgen:"{'in':'1.5'}"`
}

// Represents a network interface.
//
//docgen:configuration
type Device struct {
	// Specifies the interface name, mutually exclusive with `deviceSelector`.
	DeviceInterface string `yaml:"interface,omitempty" docgen:"{'in':'1.5'}"`

	// Selects a network device using the selector, mutually exclusive with `interface`.
	DeviceSelector *NetworkDeviceSelector `yaml:"deviceSelector,omitempty" docgen:"{'in':'1.5'}"`

	// Assigns static IP addresses to the interface in CIDR notation or as a standalone address.
	DeviceAddresses []string `yaml:"addresses,omitempty" docgen:"{'in':'1.5'}"`

	DeviceCIDR string `yaml:"cidr,omitempty" docgen:"{'in':'1.5'}"`

	// Defines a list of routes associated with the interface, appended to routes returned by DHCP if used.
	DeviceRoutes []*Route `yaml:"routes,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies bond specific options.
	DeviceBond *Bond `yaml:"bond,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies bridge specific options.
	DeviceBridge *Bridge `yaml:"bridge,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies VLAN specific options.
	DeviceVlans VlanList `yaml:"vlans,omitempty" docgen:"{'in':'1.5'}"`

	// Defines the interface's MTU, overwrites any MTU settings returned from DHCP if used.
	DeviceMTU int `yaml:"mtu,omitempty" docgen:"{'in':'1.5'}"`

	// Indicates if DHCP should be used to configure the interface.
	DeviceDHCP *bool `yaml:"dhcp,omitempty" docgen:"{'in':'1.5'}"`

	// Indicates if the interface configuration should be ignored.
	DeviceIgnore *bool `yaml:"ignore,omitempty" docgen:"{'in':'1.5'}"`

	// Indicates if the interface is a virtual-only, dummy interface.
	DeviceDummy *bool `yaml:"dummy,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies DHCP specific options, effective only when `dhcp` is true.
	DeviceDHCPOptions *DHCPOptions `yaml:"dhcpOptions,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies Wireguard specific configuration.
	DeviceWireguardConfig *DeviceWireguardConfig `yaml:"wireguard,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies virtual (shared) IP address configuration.
	DeviceVIPConfig *DeviceVIPConfig `yaml:"vip,omitempty" docgen:"{'in':'1.5'}"`
}

// DHCPOptions contains options for configuring the DHCP settings for a given interface.
//
//docgen:configuration
type DHCPOptions struct {
	// Specifies the priority of all routes received via DHCP.
	DHCPRouteMetric uint32 `yaml:"routeMetric" docgen:"{'in':'1.5'}"`

	// Enables DHCPv4 protocol for the interface (default is enabled).
	DHCPIPv4 *bool `yaml:"ipv4,omitempty" docgen:"{'in':'1.5'}"`

	// Enables DHCPv6 protocol for the interface (default is disabled).
	DHCPIPv6 *bool `yaml:"ipv6,omitempty" docgen:"{'in':'1.5'}"`

	// Set client DUID (hex string).
	DHCPDUIDv6 string `yaml:"duidv6,omitempty" docgen:"{'in':'1.5'}"`
}

// DeviceWireguardConfig contains settings for configuring Wireguard network interface.
//
//docgen:configuration
type DeviceWireguardConfig struct {
	// Specifies a private key configuration (base64 encoded).
	WireguardPrivateKey string `yaml:"privateKey,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a device's listening port.
	WireguardListenPort int `yaml:"listenPort,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a device's firewall mark.
	WireguardFirewallMark int `yaml:"firewallMark,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a list of peer configurations to apply to a device.
	WireguardPeers []*DeviceWireguardPeer `yaml:"peers,omitempty" docgen:"{'in':'1.5'}"`
}

// DeviceWireguardPeer a WireGuard device peer configuration.
//
//docgen:configuration
type DeviceWireguardPeer struct {
	// Specifies the public key of this peer.
	WireguardPublicKey string `yaml:"publicKey,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the endpoint of this peer entry.
	WireguardEndpoint string `yaml:"endpoint,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the persistent keepalive interval for this peer.
	WireguardPersistentKeepaliveInterval time.Duration `yaml:"persistentKeepaliveInterval,omitempty" docgen:"{'in':'1.5'}"`

	// AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
	WireguardAllowedIPs []string `yaml:"allowedIPs,omitempty" docgen:"{'in':'1.5'}"`
}

// DeviceVIPConfig contains settings for configuring a Virtual Shared IP on an interface.
//
//docgen:configuration
type DeviceVIPConfig struct {
	// Specifies the IP address to be used.
	SharedIP string `yaml:"ip,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the Equinix Metal API settings to assign VIP to the node.
	EquinixMetalConfig *VIPEquinixMetalConfig `yaml:"equinixMetal,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the Hetzner Cloud API settings to assign VIP to the node.
	HCloudConfig *VIPHCloudConfig `yaml:"hcloud,omitempty" docgen:"{'in':'1.5'}"`
}

// VIPEquinixMetalConfig contains settings for Equinix Metal VIP management.
//
//docgen:configuration
type VIPEquinixMetalConfig struct {
	// Specifies the Equinix Metal API Token.
	EquinixMetalAPIToken string `yaml:"apiToken" docgen:"{'in':'1.5'}"`
}

// VIPHCloudConfig contains settings for Hetzner Cloud VIP management.
//
//docgen:configuration
type VIPHCloudConfig struct {
	// Specifies the Hetzner Cloud API Token.
	HCloudAPIToken string `yaml:"apiToken" docgen:"{'in':'1.5'}"`
}

// Represents options for configuring a bonded interface.
//
//docgen:configuration
type Bond struct {
	// Comprises the interfaces making up the bond.
	BondInterfaces []string `yaml:"interfaces" docgen:"{'in':'1.5'}"`

	// Selects a network device using the selector, mutually exclusive with `interfaces`.
	BondDeviceSelectors []NetworkDeviceSelector `yaml:"deviceSelectors,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation). Not supported currently.
	BondARPIPTarget []string `yaml:"arpIPTarget,omitempty" docgen:"{'in':'1.5'}"`

	// Defines a bond mode (see official kernel documentation).
	BondMode string `yaml:"mode" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondHashPolicy string `yaml:"xmitHashPolicy,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondLACPRate string `yaml:"lacpRate,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation). Not supported currently.
	BondADActorSystem string `yaml:"adActorSystem,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondARPValidate string `yaml:"arpValidate,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondARPAllTargets string `yaml:"arpAllTargets,omitempty" docgen:"{'in':'1.5'}"`

	// Defines a primary bond (see official kernel documentation).
	BondPrimary string `yaml:"primary,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondPrimaryReselect string `yaml:"primaryReselect,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondFailOverMac string `yaml:"failOverMac,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondADSelect string `yaml:"adSelect,omitempty" docgen:"{'in':'1.5'}"`

	// Defines an MII monitor bond option (see official kernel documentation).
	BondMIIMon uint32 `yaml:"miimon,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondUpDelay uint32 `yaml:"updelay,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondDownDelay uint32 `yaml:"downdelay,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondARPInterval uint32 `yaml:"arpInterval,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondResendIGMP uint32 `yaml:"resendIgmp,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondMinLinks uint32 `yaml:"minLinks,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondLPInterval uint32 `yaml:"lpInterval,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondPacketsPerSlave uint32 `yaml:"packetsPerSlave,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondNumPeerNotif uint8 `yaml:"numPeerNotif,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondTLBDynamicLB uint8 `yaml:"tlbDynamicLb,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondAllSlavesActive uint8 `yaml:"allSlavesActive,omitempty" docgen:"{'in':'1.5'}"`

	// Indicates if a bond option should use a carrier (see official kernel documentation).
	BondUseCarrier *bool `yaml:"useCarrier,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondADActorSysPrio uint16 `yaml:"adActorSysPrio,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondADUserPortKey uint16 `yaml:"adUserPortKey,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies a bond option (see official kernel documentation).
	BondPeerNotifyDelay uint32 `yaml:"peerNotifyDelay,omitempty" docgen:"{'in':'1.5'}"`
}

// STP contains the various options for configuring the STP properties of a bridge interface.
//
//docgen:configuration
type STP struct {
	// Specifies whether Spanning Tree Protocol (STP) is enabled.
	STPEnabled *bool `yaml:"enabled,omitempty" docgen:"{'in':'1.5'}"`
}

// Bridge contains the various options for configuring a bridge interface.
//
//docgen:configuration
type Bridge struct {
	// Lists the interfaces that make up the bridge.
	BridgedInterfaces []string `yaml:"interfaces" docgen:"{'in':'1.5'}"`

	// A bridge option.
	BridgeSTP *STP `yaml:"stp,omitempty" docgen:"{'in':'1.5'}"`
}

// VlanList is a list of *Vlan structures with overridden merge process.
type VlanList []*Vlan

// Merge the network interface configuration intelligently.
func (vlans *VlanList) Merge(other interface{}) error {
	otherVlans, ok := other.(VlanList)
	if !ok {
		return fmt.Errorf("unexpected type for vlan merge %T", other)
	}

	for _, vlan := range otherVlans {
		if err := vlans.mergeVlan(vlan); err != nil {
			return err
		}
	}

	return nil
}

func (vlans *VlanList) mergeVlan(vlan *Vlan) error {
	var existing *Vlan

	for _, v := range *vlans {
		if v.VlanID == vlan.VlanID {
			existing = v

			break
		}
	}

	if existing != nil {
		return merge.Merge(existing, vlan)
	}

	*vlans = append(*vlans, vlan)

	return nil
}

// Represents VLAN settings for a device.
//
//docgen:configuration
type Vlan struct {
	// Specifies the addresses in CIDR notation or as plain IPs.
	VlanAddresses []string `yaml:"addresses,omitempty" docgen:"{'in':'1.5'}"`

	VlanCIDR string `yaml:"cidr,omitempty" docgen:"{'in':'1.5'}"`

	// Provides a list of routes associated with the VLAN.
	VlanRoutes []*Route `yaml:"routes" docgen:"{'in':'1.5'}"`

	// Indicates whether DHCP should be used.
	VlanDHCP *bool `yaml:"dhcp,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the VLAN's ID.
	VlanID uint16 `yaml:"vlanId" docgen:"{'in':'1.5'}"`

	// Specifies the VLAN's MTU.
	VlanMTU uint32 `yaml:"mtu,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the VLAN's virtual IP address configuration.
	VlanVIP *DeviceVIPConfig `yaml:"vip,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies DHCP specific options, effective only when `dhcp` is true.
	VlanDHCPOptions *DHCPOptions `yaml:"dhcpOptions,omitempty" docgen:"{'in':'1.5'}"`
}

// Route represents a network route.
//
//docgen:configuration
type Route struct {
	// The route's network (destination).
	RouteNetwork string `yaml:"network" docgen:"{'in':'1.5'}"`

	// The route's gateway.
	RouteGateway string `yaml:"gateway" docgen:"{'in':'1.5'}"`

	// The route's source address.
	RouteSource string `yaml:"source,omitempty" docgen:"{'in':'1.5'}"`

	// The optional metric for the route.
	RouteMetric uint32 `yaml:"metric,omitempty" docgen:"{'optional':true, 'in':'1.5'}"`

	// The optional MTU for the route.
	RouteMTU uint32 `yaml:"mtu,omitempty" docgen:"{'optional':true, 'in':'1.5'}"`
}

// RegistryMirrorConfig represents mirror configuration for a registry.
//
//docgen:configuration
type RegistryMirrorConfig struct {
	// List of endpoints for registry mirrors to use.
	MirrorEndpoints []string `yaml:"endpoints" docgen:"{'in':'1.5'}"`

	// Use the exact path specified for the endpoint.
	MirrorOverridePath *bool `yaml:"overridePath,omitempty" docgen:"{'in':'1.5'}"`
}

// RegistryConfig specifies auth & TLS config per registry.
//
//docgen:configuration
type RegistryConfig struct {
	// The TLS configuration for the registry.
	RegistryTLS *RegistryTLSConfig `yaml:"tls,omitempty" docgen:"{'in':'1.5'}"`

	// The auth configuration for this registry.
	RegistryAuth *RegistryAuthConfig `yaml:"auth,omitempty" docgen:"{'optional':true, 'in':'1.5'}"`
}

// RegistryAuthConfig specifies authentication configuration for a registry.
//
//docgen:configuration
type RegistryAuthConfig struct {
	// Optional registry authentication.
	RegistryUsername string `yaml:"username,omitempty" docgen:"{'in':'1.5'}"`

	// Optional registry authentication.
	RegistryPassword string `yaml:"password,omitempty" docgen:"{'in':'1.5'}"`

	// Optional registry authentication.
	RegistryAuth string `yaml:"auth,omitempty" docgen:"{'in':'1.5'}"`

	// Optional registry authentication.
	RegistryIdentityToken string `yaml:"identityToken,omitempty" docgen:"{'in':'1.5'}"`
}

// RegistryTLSConfig specifies TLS config for HTTPS registries.
//
//docgen:configuration
type RegistryTLSConfig struct {
	// Enable mutual TLS authentication with the registry.
	TLSClientIdentity *x509.PEMEncodedCertificateAndKey `yaml:"clientIdentity,omitempty" docgen:"{'in':'1.5'}"`

	// CA registry certificate to add the list of trusted certificates.
	TLSCA Base64Bytes `yaml:"ca,omitempty" docgen:"{'in':'1.5'}"`

	// Skip TLS server certificate verification.
	TLSInsecureSkipVerify *bool `yaml:"insecureSkipVerify,omitempty" docgen:"{'in':'1.5'}"`
}

// SystemDiskEncryptionConfig specifies system disk partitions encryption settings.
//
//docgen:configuration
type SystemDiskEncryptionConfig struct {
	// State partition encryption.
	StatePartition *EncryptionConfig `yaml:"state,omitempty" docgen:"{'in':'1.5'}"`

	// Ephemeral partition encryption.
	EphemeralPartition *EncryptionConfig `yaml:"ephemeral,omitempty" docgen:"{'in':'1.5'}"`
}

var _ config.Features = (*FeaturesConfig)(nil)

// FeaturesConfig describes individual Talos features that can be switched on or off.
//
//docgen:configuration
type FeaturesConfig struct {
	// Enable role-based access control (RBAC).
	RBAC *bool `yaml:"rbac,omitempty" docgen:"{'in':'1.5'}"`

	// Enable stable default hostname.
	StableHostname *bool `yaml:"stableHostname,omitempty" docgen:"{'in':'1.5'}"`

	// Configure Talos API access from Kubernetes pods.
	KubernetesTalosAPIAccessConfig *KubernetesTalosAPIAccessConfig `yaml:"kubernetesTalosAPIAccess,omitempty" docgen:"{'in':'1.5'}"`

	// Enable checks for extended key usage of client certificates in apid.
	ApidCheckExtKeyUsage *bool `yaml:"apidCheckExtKeyUsage,omitempty" docgen:"{'in':'1.5'}"`

	// Enable XFS project quota support for EPHEMERAL partition and user disks.
	DiskQuotaSupport *bool `yaml:"diskQuotaSupport,omitempty" docgen:"{'in':'1.5'}"`

	// KubePrism - local proxy/load balancer on defined port that will distribute
	// requests to all API servers in the cluster.
	KubePrismSupport *KubePrism `yaml:"kubePrism,omitempty" docgen:"{'in':'1.5'}"`

	// Configures host DNS caching resolver.
	HostDNSSupport *HostDNSConfig `yaml:"hostDNS,omitempty" docgen:"{'in':'1.7'}"`
}

// KubePrism describes the configuration for the KubePrism load balancer.
//
//docgen:configuration
type KubePrism struct {
	// Enable KubePrism support - will start local load balancing proxy.
	ServerEnabled *bool `yaml:"enabled,omitempty" docgen:"{'in':'1.5'}"`

	// KubePrism port.
	ServerPort int `yaml:"port,omitempty" docgen:"{'in':'1.5'}"`
}

// KubernetesTalosAPIAccessConfig describes the configuration for the Talos API access from Kubernetes pods.
//
//docgen:configuration
type KubernetesTalosAPIAccessConfig struct {
	// Enable Talos API access from Kubernetes pods.
	AccessEnabled *bool `yaml:"enabled,omitempty" docgen:"{'in':'1.5'}"`

	// The list of Talos API roles which can be granted for access from Kubernetes pods.
	AccessAllowedRoles []string `yaml:"allowedRoles,omitempty" docgen:"{'in':'1.5'}"`

	// The list of Kubernetes namespaces Talos API access is available from.
	AccessAllowedKubernetesNamespaces []string `yaml:"allowedKubernetesNamespaces,omitempty" docgen:"{'in':'1.5'}"`
}

// HostDNSConfig describes the configuration for the host DNS resolver.
type HostDNSConfig struct {
	//   description: |
	//     Enable host DNS caching resolver.
	HostDNSEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     Use the host DNS resolver as upstream for Kubernetes CoreDNS pods.
	//
	//     When enabled, CoreDNS pods use host DNS server as the upstream DNS (instead of
	//     using configured upstream DNS resolvers directly).
	HostDNSForwardKubeDNSToHost *bool `yaml:"forwardKubeDNSToHost,omitempty"`
	//   description: |
	//     Resolve member hostnames using the host DNS resolver.
	//
	//     When enabled, cluster member hostnames and node names are resolved using the host DNS resolver.
	//     This requires service discovery to be enabled.
	HostDNSResolveMemberNames *bool `yaml:"resolveMemberNames,omitempty"`
}

// VolumeMountConfig struct describes extra volume mount for the static pods.
//
//docgen:configuration
type VolumeMountConfig struct {
	// Path on the host.
	VolumeHostPath string `yaml:"hostPath" docgen:"{'in':'1.5'}"`

	// Path in the container.
	VolumeMountPath string `yaml:"mountPath" docgen:"{'in':'1.5'}"`

	// Mount the volume read-only.
	VolumeReadOnly bool `yaml:"readonly,omitempty" docgen:"{'in':'1.5'}"`
}

// ClusterInlineManifests is a list of ClusterInlineManifest.
//
//docgen:alias
type ClusterInlineManifests []ClusterInlineManifest

// UnmarshalYAML implements yaml.Unmarshaler.
func (manifests *ClusterInlineManifests) UnmarshalYAML(value *yaml.Node) error {
	var result []ClusterInlineManifest

	if err := value.Decode(&result); err != nil {
		return err
	}

	for i := range result {
		result[i].InlineManifestContents = strings.TrimLeft(result[i].InlineManifestContents, "\t\n\v\f\r")
	}

	*manifests = result

	return nil
}

// ClusterInlineManifest struct describes inline bootstrap manifests for the user.
//
//docgen:configuration
type ClusterInlineManifest struct {
	// Specifies the name of the manifest. Name should be unique.
	InlineManifestName string `yaml:"name" docgen:"{'in':'1.5'}"`

	// Manifest contents as a string.
	InlineManifestContents string `yaml:"contents" docgen:"{'in':'1.5'}"`
}

// NetworkKubeSpan struct describes KubeSpan configuration.
//
//docgen:configuration
type NetworkKubeSpan struct {
	// Determines whether to enable the KubeSpan feature.
	KubeSpanEnabled *bool `yaml:"enabled,omitempty" docgen:"{'in':'1.5'}"`

	// Controls whether Kubernetes pod CIDRs are announced over KubeSpan from the node.
	KubeSpanAdvertiseKubernetesNetworks *bool `yaml:"advertiseKubernetesNetworks,omitempty" docgen:"{'in':'1.5'}"`

	// Determines whether to skip sending traffic via KubeSpan if the peer connection state is not up.
	KubeSpanAllowDownPeerBypass *bool `yaml:"allowDownPeerBypass,omitempty" docgen:"{'in':'1.5'}"`
	// description: |

	// FIXME!!!
	KubeSpanHarvestExtraEndpoints *bool `yaml:"harvestExtraEndpoints,omitempty" docgen:"{'in':'1.6'}"`

	// KubeSpan link MTU size.
	KubeSpanMTU *uint32 `yaml:"mtu,omitempty" docgen:"{'in':'1.5'}"`

	// KubeSpan advanced filtering of network addresses.
	KubeSpanFilters *KubeSpanFilters `yaml:"filters,omitempty" docgen:"{'in':'1.5'}"`
}

// KubeSpanFilters struct describes KubeSpan advanced network addresses filtering.
//
//docgen:configuration
type KubeSpanFilters struct {
	// Filters node addresses which will be advertised as KubeSpan endpoints for peer-to-peer Wireguard connections.
	KubeSpanFiltersEndpoints []string `yaml:"endpoints,omitempty" docgen:"{'in':'1.5'}"`
}

// NetworkDeviceSelector struct describes network device selector.
//
//docgen:configuration
type NetworkDeviceSelector struct {
	// PCI, USB bus prefix, supports matching by wildcard.
	NetworkDeviceBus string `yaml:"busPath,omitempty" docgen:"{'in':'1.5'}"`

	// Device hardware address, supports matching by wildcard.
	NetworkDeviceHardwareAddress string `yaml:"hardwareAddr,omitempty" docgen:"{'in':'1.5'}"`

	// PCI ID (vendor ID, product ID), supports matching by wildcard.
	NetworkDevicePCIID string `yaml:"pciID,omitempty" docgen:"{'in':'1.5'}"`

	// Kernel driver, supports matching by wildcard.
	NetworkDeviceKernelDriver string `yaml:"driver,omitempty" docgen:"{'in':'1.5'}"`

	// Select only physical devices.
	NetworkDevicePhysical *bool `yaml:"physical,omitempty" docgen:"{'in':'1.6'}"`
}

// ClusterDiscoveryConfig struct configures cluster membership discovery.
//
//docgen:configuration
type ClusterDiscoveryConfig struct {
	// Enables the cluster membership discovery feature.
	DiscoveryEnabled *bool `yaml:"enabled,omitempty" docgen:"{'in':'1.5'}"`

	// Configures registries used for cluster member discovery.
	DiscoveryRegistries DiscoveryRegistriesConfig `yaml:"registries" docgen:"{'in':'1.5'}"`
}

// DiscoveryRegistriesConfig struct configures cluster membership discovery.
//
//docgen:configuration
type DiscoveryRegistriesConfig struct {
	// Configures the Kubernetes discovery registry.
	RegistryKubernetes RegistryKubernetesConfig `yaml:"kubernetes" docgen:"{'in':'1.5'}"`

	// Configures the external service discovery registry.
	RegistryService RegistryServiceConfig `yaml:"service" docgen:"{'in':'1.5'}"`
}

// RegistryKubernetesConfig struct configures Kubernetes discovery registry.
//
//docgen:configuration
type RegistryKubernetesConfig struct {
	// Disables the Kubernetes discovery registry.
	RegistryDisabled *bool `yaml:"disabled,omitempty" docgen:"{'in':'1.5'}"`
}

// RegistryServiceConfig struct configures Kubernetes discovery registry.
//
//docgen:configuration
type RegistryServiceConfig struct {
	// Disables the external service discovery registry.
	RegistryDisabled *bool `yaml:"disabled,omitempty" docgen:"{'in':'1.5'}"`

	// Specifies the external service endpoint.
	RegistryEndpoint string `yaml:"endpoint,omitempty" docgen:"{'in':'1.5'}"`
}

// UdevConfig describes how the udev system should be configured.
//
//docgen:configuration
type UdevConfig struct {
	// Lists udev rules to apply to the udev system.
	UdevRules []string `yaml:"rules,omitempty" docgen:"{'in':'1.5'}"`
}

// LoggingConfig struct configures Talos logging.
//
//docgen:configuration
type LoggingConfig struct {
	// Specifies logging destinations.
	LoggingDestinations []LoggingDestination `yaml:"destinations" docgen:"{'in':'1.5'}"`
}

// LoggingDestination struct configures Talos logging destination.
//
//docgen:configuration
type LoggingDestination struct {
	// Determines where to send logs.
	LoggingEndpoint *Endpoint `yaml:"endpoint" docgen:"{'in':'1.5'}"`

	// Specifies the logs format.
	LoggingFormat string `yaml:"format" docgen:"{'in':'1.5'}"`

	// Specifies exta tags (key-value) pairs to attach to every log message sent.
	LoggingExtraTags map[string]string `yaml:"extraTags,omitempty" docgen:"{'in':'1.7'}"`
}

// KernelConfig struct configures Talos Linux kernel.
//
//docgen:configuration
type KernelConfig struct {
	// Lists kernel modules to load.
	KernelModules []*KernelModuleConfig `yaml:"modules,omitempty" docgen:"{'in':'1.5'}"`
}

// KernelModuleConfig struct configures Linux kernel modules to load.
//
//docgen:configuration
type KernelModuleConfig struct {
	// Specifies the module name.
	ModuleName string `yaml:"name" docgen:"{'in':'1.5'}"`

	// Lists module parameters, changes applied after reboot.
	ModuleParameters []string `yaml:"parameters,omitempty" docgen:"{'in':'1.5'}"`
}
