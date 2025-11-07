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
package v1alpha1

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output ./v1alpha1_types_doc.go ./v1alpha1_types.go

//go:generate go tool k8s.io/code-generator/cmd/deepcopy-gen --go-header-file ../../../../../hack/boilerplate.txt --bounding-dirs ../v1alpha1 --output-file zz_generated.deepcopy

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
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/merge"
)

func init() {
	registry.Register("v1alpha1", func(version string) config.Document {
		return &Config{}
	})
}

// Config defines the v1alpha1.Config Talos machine configuration document.
//
//	examples:
//	   - value: configExample()
//	schemaRoot: true
type Config struct {
	//   description: |
	//     Indicates the schema used to decode the contents.
	//   values:
	//     - "v1alpha1"
	ConfigVersion string `yaml:"version"`
	//   description: |
	//     Enable verbose logging to the console.
	//     All system containers logs will flow into serial console.
	//
	//     **Note:** To avoid breaking Talos bootstrap flow enable this option only if serial console can handle high message throughput.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	ConfigDebug *bool `yaml:"debug,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: Not supported anymore.
	ConfigPersist *bool `yaml:"persist,omitempty"`
	//   description: |
	//     Provides machine specific configuration options.
	MachineConfig *MachineConfig `yaml:"machine"`
	//   description: |
	//     Provides cluster specific configuration options.
	ClusterConfig *ClusterConfig `yaml:"cluster"`
}

var _ config.MachineConfig = (*MachineConfig)(nil)

// MachineConfig represents the machine-specific config values.
//
//	examples:
//	   - value: machineConfigExample()
type MachineConfig struct {
	//   description: |
	//     Defines the role of the machine within the cluster.
	//
	//     **Control Plane**
	//
	//     Control Plane node type designates the node as a control plane member.
	//     This means it will host etcd along with the Kubernetes controlplane components such as API Server, Controller Manager, Scheduler.
	//
	//     **Worker**
	//
	//     Worker node type designates the node as a worker node.
	//     This means it will be an available compute node for scheduling workloads.
	//
	//     This node type was previously known as "join"; that value is still supported but deprecated.
	//   values:
	//     - "controlplane"
	//     - "worker"
	MachineType string `yaml:"type"`
	//   description: |
	//     The `token` is used by a machine to join the PKI of the cluster.
	//     Using this token, a machine will create a certificate signing request (CSR), and request a certificate that will be used as its' identity.
	//   examples:
	//     - name: example token
	//       value: "\"328hom.uqjzh6jnn2eie9oi\""
	MachineToken string `yaml:"token"` // Warning: It is important to ensure that this token is correct since a machine's certificate has a short TTL by default.
	//   description: |
	//     The root certificate authority of the PKI.
	//     It is composed of a base64 encoded `crt` and `key`.
	//   examples:
	//     - value: pemEncodedCertificateExample()
	//       name: machine CA example
	//   schema:
	//     type: object
	//     additionalProperties: false
	//     properties:
	//       crt:
	//         type: string
	//       key:
	//         type: string
	MachineCA *x509.PEMEncodedCertificateAndKey `yaml:"ca,omitempty"`
	//   description: |
	//     The certificates issued by certificate authorities are accepted in addition to issuing 'ca'.
	//     It is composed of a base64 encoded `crt``.
	//   schema:
	//     type: object
	//     additionalProperties: false
	//     properties:
	//       crt:
	//         type: string
	MachineAcceptedCAs []*x509.PEMEncodedCertificate `yaml:"acceptedCAs,omitempty"`
	//   description: |
	//     Extra certificate subject alternative names for the machine's certificate.
	//     By default, all non-loopback interface IPs are automatically added to the certificate's SANs.
	//   examples:
	//     - name: Uncomment this to enable SANs.
	//       value: '[]string{"10.0.0.10", "172.16.0.10", "192.168.0.10"}'
	MachineCertSANs []string `yaml:"certSANs"`
	//   description: |
	//     Provides machine specific control plane configuration options.
	//   examples:
	//     - name: ControlPlane definition example.
	//       value: machineControlplaneExample()
	MachineControlPlane *MachineControlPlaneConfig `yaml:"controlPlane,omitempty"`
	//   description: |
	//     Used to provide additional options to the kubelet.
	//   examples:
	//     - name: Kubelet definition example.
	//       value: machineKubeletExample()
	MachineKubelet *KubeletConfig `yaml:"kubelet,omitempty"`
	//   description: |
	//     Used to provide static pod definitions to be run by the kubelet directly bypassing the kube-apiserver.
	//
	//     Static pods can be used to run components which should be started before the Kubernetes control plane is up.
	//     Talos doesn't validate the pod definition.
	//     Updates to this field can be applied without a reboot.
	//
	//     See https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/.
	//   examples:
	//     - name: nginx static pod.
	//       value: machinePodsExample()
	//   schema:
	//     type: array
	//     items:
	//       type: object
	MachinePods []Unstructured `yaml:"pods,omitempty"`
	//   description: |
	//     Provides machine specific network configuration options.
	//   examples:
	//     - name: Network definition example.
	//       value: machineNetworkConfigExample()
	MachineNetwork *NetworkConfig `yaml:"network,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: Use 'UserVolumeConfig' instead.
	MachineDisks []*MachineDisk `yaml:"disks,omitempty"` // Note: `size` is in units of bytes.
	//   description: |
	//     Used to provide instructions for installations.
	//
	//     Note that this configuration section gets silently ignored by Talos images that are considered pre-installed.
	//     To make sure Talos installs according to the provided configuration, Talos should be booted with ISO or PXE-booted.
	//   examples:
	//     - name: MachineInstall config usage example.
	//       value: machineInstallExample()
	MachineInstall *InstallConfig `yaml:"install,omitempty"`
	//   description: |
	//     Allows the addition of user specified files.
	//     The value of `op` can be `create`, `overwrite`, or `append`.
	//     In the case of `create`, `path` must not exist.
	//     In the case of `overwrite`, and `append`, `path` must be a valid file.
	//     If an `op` value of `append` is used, the existing file will be appended.
	//     Note that the file contents are not required to be base64 encoded.
	//   examples:
	//      - name: MachineFiles usage example.
	//        value: machineFilesExample()
	MachineFiles []*MachineFile `yaml:"files,omitempty"` // Note: The specified `path` is relative to `/var`.
	//   description: |
	//     The `env` field allows for the addition of environment variables.
	//     All environment variables are set on PID 1 in addition to every service.
	//   values:
	//     - "`GRPC_GO_LOG_VERBOSITY_LEVEL`"
	//     - "`GRPC_GO_LOG_SEVERITY_LEVEL`"
	//     - "`http_proxy`"
	//     - "`https_proxy`"
	//     - "`no_proxy`"
	//   examples:
	//     - name: Environment variables definition examples.
	//       value: machineEnvExamples0()
	//     - value: machineEnvExamples1()
	//     - value: machineEnvExamples2()
	//   schema:
	//     type: object
	//     patternProperties:
	//       ".*":
	//         type: string
	MachineEnv Env `yaml:"env,omitempty"`
	//   description: |
	//     Used to configure the machine's time settings.
	//   examples:
	//     - name: Example configuration for cloudflare ntp server.
	//       value: machineTimeExample()
	MachineTime *TimeConfig `yaml:"time,omitempty"`
	//   description: |
	//     Used to configure the machine's sysctls.
	//   examples:
	//     - name: MachineSysctls usage example.
	//       value: machineSysctlsExample()
	MachineSysctls map[string]string `yaml:"sysctls,omitempty"`
	//   description: |
	//     Used to configure the machine's sysfs.
	//   examples:
	//     - name: MachineSysfs usage example.
	//       value: machineSysfsExample()
	MachineSysfs map[string]string `yaml:"sysfs,omitempty"`
	//   description: |
	//     Used to configure the machine's container image registry mirrors.
	//
	//     Automatically generates matching CRI configuration for registry mirrors.
	//
	//     The `mirrors` section allows to redirect requests for images to a non-default registry,
	//     which might be a local registry or a caching mirror.
	//
	//     The `config` section provides a way to authenticate to the registry with TLS client
	//     identity, provide registry CA, or authentication information.
	//     Authentication information has same meaning with the corresponding field in [`.docker/config.json`](https://docs.docker.com/engine/api/v1.41/#section/Authentication).
	//
	//     See also matching configuration for [CRI containerd plugin](https://github.com/containerd/cri/blob/master/docs/registry.md).
	//   examples:
	//     - value: machineConfigRegistriesExample()
	MachineRegistries RegistriesConfig `yaml:"registries,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: Use `VolumeConfig` instead.
	MachineSystemDiskEncryption *SystemDiskEncryptionConfig `yaml:"systemDiskEncryption,omitempty"`
	//   description: |
	//     Features describe individual Talos features that can be switched on or off.
	//   examples:
	//     - value: machineFeaturesExample()
	MachineFeatures *FeaturesConfig `yaml:"features,omitempty"`
	//   description: |
	//     Configures the udev system.
	//   examples:
	//     - value: machineUdevExample()
	MachineUdev *UdevConfig `yaml:"udev,omitempty"`
	//   description: |
	//     Configures the logging system.
	//   examples:
	//     - value: machineLoggingExample()
	MachineLogging *LoggingConfig `yaml:"logging,omitempty"`
	//   description: |
	//     Configures the kernel.
	//   examples:
	//     - value: machineKernelExample()
	MachineKernel *KernelConfig `yaml:"kernel,omitempty"`
	//  description: |
	//    Configures the seccomp profiles for the machine.
	//  examples:
	//    - value: machineSeccompExample()
	MachineSeccompProfiles []*MachineSeccompProfile `yaml:"seccompProfiles,omitempty" talos:"omitonlyifnil"`
	//  description: |
	//    Override (patch) settings in the default OCI runtime spec for CRI containers.
	//
	//    It can be used to set some default container settings which are not configurable in Kubernetes,
	//    for example default ulimits.
	//    Note: this change applies to all newly created containers, and it requires a reboot to take effect.
	//  examples:
	//    - name: override default open file limit
	//      value: machineBaseRuntimeSpecOverridesExample()
	//  schema:
	//    type: object
	MachineBaseRuntimeSpecOverrides Unstructured `yaml:"baseRuntimeSpecOverrides,omitempty"`
	//  description: |
	//    Configures the node labels for the machine.
	//
	//    Note: In the default Kubernetes configuration, worker nodes are restricted to set
	//    labels with some prefixes (see [NodeRestriction](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction) admission plugin).
	//  examples:
	//    - name: node labels example.
	//      value: 'map[string]string{"exampleLabel": "exampleLabelValue"}'
	MachineNodeLabels map[string]string `yaml:"nodeLabels,omitempty"`
	//  description: |
	//    Configures the node annotations for the machine.
	//  examples:
	//    - name: node annotations example.
	//      value: 'map[string]string{"customer.io/rack": "r13a25"}'
	MachineNodeAnnotations map[string]string `yaml:"nodeAnnotations,omitempty"`
	//  description: |
	//    Configures the node taints for the machine. Effect is optional.
	//
	//    Note: In the default Kubernetes configuration, worker nodes are not allowed to
	//    modify the taints (see [NodeRestriction](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction) admission plugin).
	//  examples:
	//    - name: node taints example.
	//      value: 'map[string]string{"exampleTaint": "exampleTaintValue:NoSchedule"}'
	MachineNodeTaints map[string]string `yaml:"nodeTaints,omitempty"`
}

// MachineSeccompProfile defines seccomp profiles for the machine.
type MachineSeccompProfile struct {
	//  description: |
	//    The `name` field is used to provide the file name of the seccomp profile.
	MachineSeccompProfileName string `yaml:"name"`
	// description: |
	//   The `value` field is used to provide the seccomp profile.
	// schema:
	//   type: object
	MachineSeccompProfileValue Unstructured `yaml:"value"`
}

var (
	_ config.ClusterConfig  = (*ClusterConfig)(nil)
	_ config.ClusterNetwork = (*ClusterConfig)(nil)
	_ config.Token          = (*ClusterConfig)(nil)
)

// ClusterConfig represents the cluster-wide config values.
//
//	examples:
//	   - value: clusterConfigExample()
type ClusterConfig struct {
	//   description: |
	//     Globally unique identifier for this cluster (base64 encoded random 32 bytes).
	ClusterID string `yaml:"id,omitempty"`
	//   description: |
	//     Shared secret of cluster (base64 encoded random 32 bytes).
	//     This secret is shared among cluster members but should never be sent over the network.
	ClusterSecret string `yaml:"secret,omitempty"`
	//   description: |
	//     Provides control plane specific configuration options.
	//   examples:
	//     - name: Setting controlplane endpoint address to 1.2.3.4 and port to 443 example.
	//       value: clusterControlPlaneExample()
	ControlPlane *ControlPlaneConfig `yaml:"controlPlane"`
	//   description: |
	//     Configures the cluster's name.
	ClusterName string `yaml:"clusterName,omitempty"`
	//   description: |
	//     Provides cluster specific network configuration options.
	//   examples:
	//     - name: Configuring with flannel CNI and setting up subnets.
	//       value:  clusterNetworkExample()
	ClusterNetwork *ClusterNetworkConfig `yaml:"network,omitempty"`
	//   description: |
	//     The [bootstrap token](https://kubernetes.io/docs/reference/access-authn-authz/bootstrap-tokens/) used to join the cluster.
	//   examples:
	//     - name: Bootstrap token example (do not use in production!).
	//       value: '"wlzjyw.bei2zfylhs2by0wd"'
	BootstrapToken string `yaml:"token,omitempty"`
	//   description: |
	//     A key used for the [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).
	//     Enables encryption with AESCBC.
	//   examples:
	//     - name: Decryption secret example (do not use in production!).
	//       value: '"z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM="'
	ClusterAESCBCEncryptionSecret string `yaml:"aescbcEncryptionSecret,omitempty"`
	//   description: |
	//     A key used for the [encryption of secret data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).
	//     Enables encryption with secretbox.
	//     Secretbox has precedence over AESCBC.
	//   examples:
	//     - name: Decryption secret example (do not use in production!).
	//       value: '"z01mye6j16bspJYtTB/5SFX8j7Ph4JXxM2Xuu4vsBPM="'
	ClusterSecretboxEncryptionSecret string `yaml:"secretboxEncryptionSecret,omitempty"`
	//   description: |
	//     The base64 encoded root certificate authority used by Kubernetes.
	//   examples:
	//     - name: ClusterCA example.
	//       value: pemEncodedCertificateExample()
	//   schema:
	//     type: object
	//     additionalProperties: false
	//     properties:
	//       crt:
	//         type: string
	//       key:
	//         type: string
	ClusterCA *x509.PEMEncodedCertificateAndKey `yaml:"ca,omitempty"`
	//   description: |
	//     The list of base64 encoded accepted certificate authorities used by Kubernetes.
	//   schema:
	//     type: object
	//     additionalProperties: false
	//     properties:
	//       crt:
	//         type: string
	ClusterAcceptedCAs []*x509.PEMEncodedCertificate `yaml:"acceptedCAs,omitempty"`
	//   description: |
	//     The base64 encoded aggregator certificate authority used by Kubernetes for front-proxy certificate generation.
	//
	//     This CA can be self-signed.
	//   examples:
	//     - name: AggregatorCA example.
	//       value: pemEncodedCertificateExample()
	//   schema:
	//     type: object
	//     additionalProperties: false
	//     properties:
	//       crt:
	//         type: string
	//       key:
	//         type: string
	ClusterAggregatorCA *x509.PEMEncodedCertificateAndKey `yaml:"aggregatorCA,omitempty"`
	//   description: |
	//     The base64 encoded private key for service account token generation.
	//   examples:
	//     - name: AggregatorCA example.
	//       value: pemEncodedKeyExample()
	//   schema:
	//     type: object
	//     additionalProperties: false
	//     properties:
	//       key:
	//         type: string
	//         additionalProperties: false
	ClusterServiceAccount *x509.PEMEncodedKey `yaml:"serviceAccount,omitempty"`
	//   description: |
	//     API server specific configuration options.
	//   examples:
	//     - value: clusterAPIServerExample()
	APIServerConfig *APIServerConfig `yaml:"apiServer,omitempty"`
	//   description: |
	//     Controller manager server specific configuration options.
	//   examples:
	//     - value: clusterControllerManagerExample()
	ControllerManagerConfig *ControllerManagerConfig `yaml:"controllerManager,omitempty"`
	//   description: |
	//     Kube-proxy server-specific configuration options
	//   examples:
	//     - value: clusterProxyExample()
	ProxyConfig *ProxyConfig `yaml:"proxy,omitempty"`
	//   description: |
	//     Scheduler server specific configuration options.
	//   examples:
	//     - value: clusterSchedulerExample()
	SchedulerConfig *SchedulerConfig `yaml:"scheduler,omitempty"`
	//   description: |
	//     Configures cluster member discovery.
	//   examples:
	//     - value: clusterDiscoveryExample()
	ClusterDiscoveryConfig *ClusterDiscoveryConfig `yaml:"discovery,omitempty"`
	//   description: |
	//     Etcd specific configuration options.
	//   examples:
	//     - value: clusterEtcdExample()
	EtcdConfig *EtcdConfig `yaml:"etcd,omitempty"`
	//   description: |
	//     Core DNS specific configuration options.
	//   examples:
	//     - value: clusterCoreDNSExample()
	CoreDNSConfig *CoreDNS `yaml:"coreDNS,omitempty"`
	//   description: |
	//     External cloud provider configuration.
	//   examples:
	//     - value: clusterExternalCloudProviderConfigExample()
	ExternalCloudProviderConfig *ExternalCloudProviderConfig `yaml:"externalCloudProvider,omitempty"`
	//   description: |
	//     A list of urls that point to additional manifests.
	//     These will get automatically deployed as part of the bootstrap.
	//   examples:
	//     - value: >
	//        []string{
	//         "https://www.example.com/manifest1.yaml",
	//         "https://www.example.com/manifest2.yaml",
	//        }
	ExtraManifests []string `yaml:"extraManifests,omitempty" talos:"omitonlyifnil"`
	//   description: |
	//     A map of key value pairs that will be added while fetching the extraManifests.
	//   examples:
	//     - value: >
	//         map[string]string{
	//           "Token": "1234567",
	//           "X-ExtraInfo": "info",
	//         }
	ExtraManifestHeaders map[string]string `yaml:"extraManifestHeaders,omitempty"`
	//   description: |
	//     A list of inline Kubernetes manifests.
	//     These will get automatically deployed as part of the bootstrap.
	//   examples:
	//     - value: clusterInlineManifestsExample()
	//   schema:
	//     type: array
	//     items:
	//       $ref: "#/$defs/v1alpha1.ClusterInlineManifest"
	ClusterInlineManifests ClusterInlineManifests `yaml:"inlineManifests,omitempty" talos:"omitonlyifnil"`
	//   description: |
	//     Settings for admin kubeconfig generation.
	//     Certificate lifetime can be configured.
	//   examples:
	//     - value: clusterAdminKubeconfigExample()
	AdminKubeconfigConfig *AdminKubeconfigConfig `yaml:"adminKubeconfig,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: Use `AllowSchedulingOnControlPlanes` instead.
	AllowSchedulingOnMasters *bool `yaml:"allowSchedulingOnMasters,omitempty"`
	//   description: |
	//     Allows running workload on control-plane nodes.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	//   examples:
	//     - value: true
	AllowSchedulingOnControlPlanes *bool `yaml:"allowSchedulingOnControlPlanes,omitempty"`
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

// MachineControlPlaneConfig machine specific configuration options.
type MachineControlPlaneConfig struct {
	//   description: |
	//     Controller manager machine specific configuration options.
	MachineControllerManager *MachineControllerManagerConfig `yaml:"controllerManager,omitempty"`
	//   description: |
	//     Scheduler machine specific configuration options.
	MachineScheduler *MachineSchedulerConfig `yaml:"scheduler,omitempty"`
}

// MachineControllerManagerConfig represents the machine specific ControllerManager config values.
type MachineControllerManagerConfig struct {
	//   description: |
	//     Disable kube-controller-manager on the node.
	MachineControllerManagerDisabled *bool `yaml:"disabled,omitempty"`
}

// MachineSchedulerConfig represents the machine specific Scheduler config values.
type MachineSchedulerConfig struct {
	//   description: |
	//     Disable kube-scheduler on the node.
	MachineSchedulerDisabled *bool `yaml:"disabled,omitempty"`
}

// KubeletConfig represents the kubelet config values.
type KubeletConfig struct {
	//   description: |
	//     The `image` field is an optional reference to an alternative kubelet image.
	//   examples:
	//     - value: kubeletImageExample()
	KubeletImage string `yaml:"image,omitempty"`
	//   description: |
	//     The `ClusterDNS` field is an optional reference to an alternative kubelet clusterDNS ip list.
	//   examples:
	//     - value: '[]string{"10.96.0.10", "169.254.2.53"}'
	KubeletClusterDNS []string `yaml:"clusterDNS,omitempty"`
	//   description: |
	//     The `extraArgs` field is used to provide additional flags to the kubelet.
	//   examples:
	//     - value: >
	//         map[string]string{
	//           "key": "value",
	//         }
	KubeletExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
	//   description: |
	//     The `extraMounts` field is used to add additional mounts to the kubelet container.
	//     Note that either `bind` or `rbind` are required in the `options`.
	//   examples:
	//     - value: kubeletExtraMountsExample()
	KubeletExtraMounts []ExtraMount `yaml:"extraMounts,omitempty"`
	//   description: |
	//     The `extraConfig` field is used to provide kubelet configuration overrides.
	//
	//     Some fields are not allowed to be overridden: authentication and authorization, cgroups
	//     configuration, ports, etc.
	//   examples:
	//     - value: kubeletExtraConfigExample()
	//   schema:
	//     type: object
	KubeletExtraConfig Unstructured `yaml:"extraConfig,omitempty"`
	//  description: |
	//   The `KubeletCredentialProviderConfig` field is used to provide kubelet credential configuration.
	//  examples:
	//    - value: kubeletCredentialProviderConfigExample()
	//  schema:
	//    type: object
	KubeletCredentialProviderConfig Unstructured `yaml:"credentialProviderConfig,omitempty"`
	//  description: |
	//    Enable container runtime default Seccomp profile.
	//  values:
	//    - true
	//    - yes
	//    - false
	//    - no
	KubeletDefaultRuntimeSeccompProfileEnabled *bool `yaml:"defaultRuntimeSeccompProfileEnabled,omitempty"`
	//   description: |
	//     The `registerWithFQDN` field is used to force kubelet to use the node FQDN for registration.
	//     This is required in clouds like AWS.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	KubeletRegisterWithFQDN *bool `yaml:"registerWithFQDN,omitempty"`
	//   description: |
	//     The `nodeIP` field is used to configure `--node-ip` flag for the kubelet.
	//     This is used when a node has multiple addresses to choose from.
	//   examples:
	//     - value: kubeletNodeIPExample()
	KubeletNodeIP *KubeletNodeIPConfig `yaml:"nodeIP,omitempty"`
	//   description: |
	//      The `skipNodeRegistration` is used to run the kubelet without registering with the apiserver.
	//      This runs kubelet as standalone and only runs static pods.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	KubeletSkipNodeRegistration *bool `yaml:"skipNodeRegistration,omitempty"`
	//   description: |
	//     The `disableManifestsDirectory` field configures the kubelet to get static pod manifests from the /etc/kubernetes/manifests directory.
	//     It's recommended to configure static pods with the "pods" key instead.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	KubeletDisableManifestsDirectory *bool `yaml:"disableManifestsDirectory,omitempty"`
}

// KubeletNodeIPConfig represents the kubelet node IP configuration.
type KubeletNodeIPConfig struct {
	//  description: |
	//    The `validSubnets` field configures the networks to pick kubelet node IP from.
	//    For dual stack configuration, there should be two subnets: one for IPv4, another for IPv6.
	//    IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.
	//    Negative subnet matches should be specified last to filter out IPs picked by positive matches.
	//    If not specified, node IP is picked based on cluster podCIDRs: IPv4/IPv6 address or both.
	KubeletNodeIPValidSubnets []string `yaml:"validSubnets,omitempty"`
}

// NetworkConfig represents the machine's networking config values.
type NetworkConfig struct {
	// docgen:nodoc
	//
	// Deprecated: use `HostnameConfig` instead.
	NetworkHostname string `yaml:"hostname,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: use multi-doc network config.
	NetworkInterfaces NetworkDeviceList `yaml:"interfaces,omitempty"`
	//   description: |
	//     Used to statically set the nameservers for the machine.
	//     Defaults to `1.1.1.1` and `8.8.8.8`
	//   examples:
	//     - value: '[]string{"8.8.8.8", "1.1.1.1"}'
	NameServers []string `yaml:"nameservers,omitempty"`
	//   description: |
	//     Used to statically set arbitrary search domains.
	//   examples:
	//     - value: '[]string{"example.org", "example.com"}'
	Searches []string `yaml:"searchDomains,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: Use `StatisHostConfig` instead.
	ExtraHostEntries []*ExtraHost `yaml:"extraHostEntries,omitempty"`
	//   description: |
	//     Configures KubeSpan feature.
	//   examples:
	//     - value: networkKubeSpanExample()
	NetworkKubeSpan *NetworkKubeSpan `yaml:"kubespan,omitempty"`
	//   description: |
	//     Disable generating a default search domain in /etc/resolv.conf
	//     based on the machine hostname.
	//     Defaults to `false`.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	NetworkDisableSearchDomain *bool `yaml:"disableSearchDomain,omitempty"`
}

// NetworkDeviceList is a list of *Device structures with overridden merge process.
//
// docgen:nodoc
type NetworkDeviceList []*Device

// Merge the network interface configuration intelligently.
func (devices *NetworkDeviceList) Merge(other any) error {
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
type InstallConfig struct {
	//   description: |
	//     The disk used for installations.
	//   examples:
	//     - value: '"/dev/sda"'
	//     - value: '"/dev/nvme0"'
	InstallDisk string `yaml:"disk,omitempty"`
	//   description: |
	//     Look up disk using disk attributes like model, size, serial and others.
	//     Always has priority over `disk`.
	//   examples:
	//     - value: machineInstallDiskSelectorExample()
	InstallDiskSelector *InstallDiskSelector `yaml:"diskSelector,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: Use Image Factory/imager instead to build a proper installer.
	InstallExtraKernelArgs []string `yaml:"extraKernelArgs,omitempty"`
	//   description: |
	//     Allows for supplying the image used to perform the installation.
	//     Image reference for each Talos release can be found on
	//     [GitHub releases page](https://github.com/siderolabs/talos/releases).
	//   examples:
	//     - value: '"ghcr.io/siderolabs/installer:latest"'
	InstallImage string `yaml:"image,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: Use custom `InstallImage` instead.
	InstallExtensions []InstallExtensionConfig `yaml:"extensions,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: It never worked.
	InstallBootloader *bool `yaml:"bootloader,omitempty"`
	//   description: |
	//     Indicates if the installation disk should be wiped at installation time.
	//     Defaults to `true`.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	InstallWipe *bool `yaml:"wipe"`
	//   description: |
	//     Indicates if MBR partition should be marked as bootable (active).
	//     Should be enabled only for the systems with legacy BIOS that doesn't support GPT partitioning scheme.
	InstallLegacyBIOSSupport *bool `yaml:"legacyBIOSSupport,omitempty"`
	//   description: |
	//     Indicates if legacy GRUB bootloader should use kernel cmdline from the UKI instead of building it on the host.
	//     This changes the way cmdline is managed with GRUB bootloader to be more consistent with UKI/systemd-boot.
	InstallGrubUseUKICmdline *bool `yaml:"grubUseUKICmdline,omitempty"`
}

// InstallDiskSizeMatcher disk size condition parser.
// docgen:nodoc
type InstallDiskSizeMatcher struct {
	MatchData InstallDiskSizeMatchData
	condition string
}

// MarshalYAML is a custom marshaller for `InstallDiskSizeMatcher`.
func (m *InstallDiskSizeMatcher) MarshalYAML() (any, error) {
	return m.condition, nil
}

// UnmarshalYAML is a custom unmarshaller for `InstallDiskSizeMatcher`.
func (m *InstallDiskSizeMatcher) UnmarshalYAML(unmarshal func(any) error) error {
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

// InstallDiskSizeMatchData contains data for comparison - Op and Size.
//
//docgen:nodoc
type InstallDiskSizeMatchData struct {
	Op   string
	Size uint64
}

// InstallDiskType custom type for disk type selector.
type InstallDiskType string

// InstallDiskSelector represents a disk query parameters for the install disk lookup.
type InstallDiskSelector struct {
	//   description: Disk size.
	//   examples:
	//     - name: Select a disk which size is equal to 4GB.
	//       value: machineInstallDiskSizeMatcherExamples0()
	//     - name: Select a disk which size is greater than 1TB.
	//       value: machineInstallDiskSizeMatcherExamples1()
	//     - name: Select a disk which size is less or equal than 2TB.
	//       value: machineInstallDiskSizeMatcherExamples2()
	//   schema:
	//     type: string
	Size *InstallDiskSizeMatcher `yaml:"size,omitempty"`
	//   description: Disk name `/sys/block/<dev>/device/name`.
	Name string `yaml:"name,omitempty"`
	//   description: Disk model `/sys/block/<dev>/device/model`.
	Model string `yaml:"model,omitempty"`
	//   description: Disk serial number `/sys/block/<dev>/serial`.
	Serial string `yaml:"serial,omitempty"`
	//   description: Disk modalias `/sys/block/<dev>/device/modalias`.
	Modalias string `yaml:"modalias,omitempty"`
	//   description: Disk UUID `/sys/block/<dev>/uuid`.
	UUID string `yaml:"uuid,omitempty"`
	//   description: Disk WWID `/sys/block/<dev>/wwid`.
	WWID string `yaml:"wwid,omitempty"`
	//   description: Disk Type.
	//   values:
	//     - ssd
	//     - hdd
	//     - nvme
	//     - sd
	Type InstallDiskType `yaml:"type,omitempty"`
	//   description: |
	//      Disk bus path.
	//   examples:
	//     - value: '"/pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0"'
	//     - value: '"/pci0000:00/*"'
	BusPath string `yaml:"busPath,omitempty"`
}

// InstallExtensionConfig represents a configuration for a system extension.
//
// docgen:nodoc
type InstallExtensionConfig struct {
	//   description: System extension image.
	ExtensionImage string `yaml:"image"`
}

// TimeConfig represents the options for configuring time on a machine.
type TimeConfig struct {
	//   description: |
	//     Indicates if the time service is disabled for the machine.
	//     Defaults to `false`.
	TimeDisabled *bool `yaml:"disabled,omitempty"`
	//   description: |
	//     Specifies time (NTP) servers to use for setting the system time.
	//     Defaults to `time.cloudflare.com`.
	//
	//	   Talos can also sync to the PTP time source (e.g provided by the hypervisor),
	//     provide the path to the PTP device as "/dev/ptp0" or "/dev/ptp_kvm".
	TimeServers []string `yaml:"servers,omitempty"`
	//   description: |
	//     Specifies the timeout when the node time is considered to be in sync unlocking the boot sequence.
	//     NTP sync will be still running in the background.
	//     Defaults to "infinity" (waiting forever for time sync)
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	TimeBootTimeout time.Duration `yaml:"bootTimeout,omitempty"`
}

// RegistriesConfig represents the image pull options.
type RegistriesConfig struct {
	//   description: |
	//     Specifies mirror configuration for each registry host namespace.
	//     This setting allows to configure local pull-through caching registires,
	//     air-gapped installations, etc.
	//
	//     For example, when pulling an image with the reference `example.com:123/image:v1`,
	//     the `example.com:123` key will be used to lookup the mirror configuration.
	//
	//     Optionally the `*` key can be used to configure a fallback mirror.
	//
	//     Registry name is the first segment of image identifier, with 'docker.io'
	//     being default one.
	//   examples:
	//     - value: machineConfigRegistryMirrorsExample()
	RegistryMirrors map[string]*RegistryMirrorConfig `yaml:"mirrors,omitempty"`
	//   description: |
	//     Specifies TLS & auth configuration for HTTPS image registries.
	//     Mutual TLS can be enabled with 'clientIdentity' option.
	//
	//     The full hostname and port (if not using a default port 443)
	//     should be used as the key.
	//     The fallback key `*` can't be used for TLS configuration.
	//
	//     TLS configuration can be skipped if registry has trusted
	//     server certificate.
	//   examples:
	//     - value: machineConfigRegistryConfigExample()
	RegistryConfig map[string]*RegistryConfig `yaml:"config,omitempty"`
}

// PodCheckpointer represents the pod-checkpointer config values.
//
//docgen:nodoc
type PodCheckpointer struct {
	//   description: |
	//     The `image` field is an override to the default pod-checkpointer image.
	PodCheckpointerImage string `yaml:"image,omitempty"`
}

// CoreDNS represents the CoreDNS config values.
type CoreDNS struct {
	//   description: |
	//     Disable coredns deployment on cluster bootstrap.
	CoreDNSDisabled *bool `yaml:"disabled,omitempty"`
	//   description: |
	//     The `image` field is an override to the default coredns image.
	CoreDNSImage string `yaml:"image,omitempty"`
}

// Endpoint represents the endpoint URL parsed out of the machine config.
type Endpoint struct {
	*url.URL
}

// UnmarshalYAML is a custom unmarshaller for `Endpoint`.
func (e *Endpoint) UnmarshalYAML(unmarshal func(any) error) error {
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
func (e *Endpoint) MarshalYAML() (any, error) {
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
type ControlPlaneConfig struct {
	//   description: |
	//     Endpoint is the canonical controlplane endpoint, which can be an IP address or a DNS hostname.
	//     It is single-valued, and may optionally include a port number.
	//   examples:
	//     - value: clusterEndpointExample1()
	//     - value: clusterEndpointExample2()
	//   schema:
	//     type: string
	//     format: uri
	//     pattern: "^https://"
	Endpoint *Endpoint `yaml:"endpoint"`
	//   description: |
	//     The port that the API server listens on internally.
	//     This may be different than the port portion listed in the endpoint field above.
	//     The default is `6443`.
	LocalAPIServerPort int `yaml:"localAPIServerPort,omitempty"`
}

var _ config.APIServer = (*APIServerConfig)(nil)

// APIServerConfig represents the kube apiserver configuration options.
type APIServerConfig struct {
	//   description: |
	//     The container image used in the API server manifest.
	//   examples:
	//     - value: clusterAPIServerImageExample()
	ContainerImage string `yaml:"image,omitempty"`
	//   description: |
	//     Extra arguments to supply to the API server.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty"`
	//   description: |
	//     Extra volumes to mount to the API server static pod.
	ExtraVolumesConfig []VolumeMountConfig `yaml:"extraVolumes,omitempty"`
	//   description: |
	//     The `env` field allows for the addition of environment variables for the control plane component.
	//   schema:
	//     type: object
	//     patternProperties:
	//       ".*":
	//         type: string
	EnvConfig Env `yaml:"env,omitempty"`
	//   description: |
	//     Extra certificate subject alternative names for the API server's certificate.
	CertSANs []string `yaml:"certSANs,omitempty"`
	// docgen:nodoc
	DisablePodSecurityPolicyConfig *bool `yaml:"disablePodSecurityPolicy,omitempty"`
	//   description: |
	//     Configure the API server admission plugins.
	//   examples:
	//     - value: admissionControlConfigExample()
	AdmissionControlConfig AdmissionPluginConfigList `yaml:"admissionControl,omitempty"`
	//   description: |
	//     Configure the API server audit policy.
	//   examples:
	//     - value: APIServerDefaultAuditPolicy
	//   schema:
	//     type: object
	AuditPolicyConfig Unstructured `yaml:"auditPolicy,omitempty" merge:"replace"`
	//   description: |
	//     Configure the API server resources.
	//   schema:
	//     type: object
	ResourcesConfig *ResourcesConfig `yaml:"resources,omitempty"`
	//   description: |
	//     Configure the API server authorization config. Node and RBAC authorizers are always added irrespective of the configuration.
	//   examples:
	//     - value: authorizationConfigExample()
	AuthorizationConfigConfig AuthorizationConfigAuthorizerConfigList `yaml:"authorizationConfig,omitempty"`
}

// AdmissionPluginConfigList represents the admission plugin configuration list.
//
//docgen:alias
type AdmissionPluginConfigList []*AdmissionPluginConfig

// Merge the admission plugin configuration intelligently.
func (configs *AdmissionPluginConfigList) Merge(other any) error {
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
type AdmissionPluginConfig struct {
	//   description: |
	//     Name is the name of the admission controller.
	//     It must match the registered admission plugin name.
	PluginName string `yaml:"name"`
	//   description: |
	//     Configuration is an embedded configuration object to be used as the plugin's
	//     configuration.
	//   schema:
	//     type: object
	PluginConfiguration Unstructured `yaml:"configuration"`
}

// AuthorizationConfigAuthorizerConfigList represents the authorization config authorizer configuration list.
//
//docgen:alias
type AuthorizationConfigAuthorizerConfigList []*AuthorizationConfigAuthorizerConfig

// AuthorizationConfigAuthorizerConfig represents the API server authorization config authorizer configuration.
type AuthorizationConfigAuthorizerConfig struct {
	//   description: |
	//     Type is the name of the authorizer. Allowed values are `Node`, `RBAC`, and `Webhook`.
	AuthorizerType string `yaml:"type"`
	//   description: |
	//     Name is used to describe the authorizer.
	AuthorizerName string `yaml:"name"`
	//   description: |
	//     webhook is the configuration for the webhook authorizer.
	//   schema:
	//     type: object
	AuthorizerWebhook Unstructured `yaml:"webhook,omitempty"`
}

var _ config.ControllerManager = (*ControllerManagerConfig)(nil)

// ControllerManagerConfig represents the kube controller manager configuration options.
type ControllerManagerConfig struct {
	//   description: |
	//     The container image used in the controller manager manifest.
	//   examples:
	//     - value: clusterControllerManagerImageExample()
	ContainerImage string `yaml:"image,omitempty"`
	//   description: |
	//     Extra arguments to supply to the controller manager.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty"`
	//   description: |
	//     Extra volumes to mount to the controller manager static pod.
	ExtraVolumesConfig []VolumeMountConfig `yaml:"extraVolumes,omitempty"`
	//   description: |
	//     The `env` field allows for the addition of environment variables for the control plane component.
	//   schema:
	//     type: object
	//     patternProperties:
	//       ".*":
	//         type: string
	EnvConfig Env `yaml:"env,omitempty"`
	//   description: |
	//     Configure the controller manager resources.
	//   schema:
	//     type: object
	ResourcesConfig *ResourcesConfig `yaml:"resources,omitempty"`
}

// ProxyConfig represents the kube proxy configuration options.
type ProxyConfig struct {
	//   description: |
	//     Disable kube-proxy deployment on cluster bootstrap.
	//   examples:
	//     - value: pointer.To(false)
	Disabled *bool `yaml:"disabled,omitempty"`
	//   description: |
	//     The container image used in the kube-proxy manifest.
	//   examples:
	//     - value: clusterProxyImageExample()
	ContainerImage string `yaml:"image,omitempty"`
	//   description: |
	//     proxy mode of kube-proxy.
	//     The default is 'iptables'.
	ModeConfig string `yaml:"mode,omitempty"`
	//   description: |
	//     Extra arguments to supply to kube-proxy.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty"`
}

var _ config.Scheduler = (*SchedulerConfig)(nil)

// SchedulerConfig represents the kube scheduler configuration options.
type SchedulerConfig struct {
	//   description: |
	//     The container image used in the scheduler manifest.
	//   examples:
	//     - value: clusterSchedulerImageExample()
	ContainerImage string `yaml:"image,omitempty"`
	//   description: |
	//     Extra arguments to supply to the scheduler.
	ExtraArgsConfig map[string]string `yaml:"extraArgs,omitempty"`
	//   description: |
	//     Extra volumes to mount to the scheduler static pod.
	ExtraVolumesConfig []VolumeMountConfig `yaml:"extraVolumes,omitempty"`
	//   description: |
	//     The `env` field allows for the addition of environment variables for the control plane component.
	//   schema:
	//     type: object
	//     patternProperties:
	//       ".*":
	//         type: string
	EnvConfig Env `yaml:"env,omitempty"`
	//   description: |
	//     Configure the scheduler resources.
	//   schema:
	//     type: object
	ResourcesConfig *ResourcesConfig `yaml:"resources,omitempty"`
	//   description: |
	//     Specify custom kube-scheduler configuration.
	//   schema:
	//     type: object
	SchedulerConfig Unstructured `yaml:"config,omitempty"`
}

var _ config.Etcd = (*EtcdConfig)(nil)

// EtcdConfig represents the etcd configuration options.
type EtcdConfig struct {
	//   description: |
	//     The container image used to create the etcd service.
	//   examples:
	//     - value: clusterEtcdImageExample()
	ContainerImage string `yaml:"image,omitempty"`
	//   description: |
	//     The `ca` is the root certificate authority of the PKI.
	//     It is composed of a base64 encoded `crt` and `key`.
	//   examples:
	//     - value: pemEncodedCertificateExample()
	//   schema:
	//     type: object
	//     additionalProperties: false
	//     properties:
	//       crt:
	//         type: string
	//       key:
	//         type: string
	RootCA *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
	//   description: |
	//     Extra arguments to supply to etcd.
	//     Note that the following args are not allowed:
	//
	//     - `name`
	//     - `data-dir`
	//     - `initial-cluster-state`
	//     - `listen-peer-urls`
	//     - `listen-client-urls`
	//     - `cert-file`
	//     - `key-file`
	//     - `trusted-ca-file`
	//     - `peer-client-cert-auth`
	//     - `peer-cert-file`
	//     - `peer-trusted-ca-file`
	//     - `peer-key-file`
	//   examples:
	//     - values: >
	//         map[string]string{
	//           "initial-cluster": "https://1.2.3.4:2380",
	//           "advertise-client-urls": "https://1.2.3.4:2379",
	//         }
	EtcdExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: use EtcdAdvertistedSubnets
	EtcdSubnet string `yaml:"subnet,omitempty"`
	//  description: |
	//    The `advertisedSubnets` field configures the networks to pick etcd advertised IP from.
	//
	//    IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.
	//    Negative subnet matches should be specified last to filter out IPs picked by positive matches.
	//    If not specified, advertised IP is selected as the first routable address of the node.
	//
	//  examples:
	//    - value: clusterEtcdAdvertisedSubnetsExample()
	EtcdAdvertisedSubnets []string `yaml:"advertisedSubnets,omitempty"`
	//  description: |
	//    The `listenSubnets` field configures the networks for the etcd to listen for peer and client connections.
	//
	//    If `listenSubnets` is not set, but `advertisedSubnets` is set, `listenSubnets` defaults to
	//    `advertisedSubnets`.
	//
	//    If neither `advertisedSubnets` nor `listenSubnets` is set, `listenSubnets` defaults to listen on all addresses.
	//
	//    IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.
	//    Negative subnet matches should be specified last to filter out IPs picked by positive matches.
	//    If not specified, advertised IP is selected as the first routable address of the node.
	EtcdListenSubnets []string `yaml:"listenSubnets,omitempty"`
}

// ClusterNetworkConfig represents kube networking configuration options.
type ClusterNetworkConfig struct {
	//   description: |
	//     The CNI used.
	//     Composed of "name" and "urls".
	//     The "name" key supports the following options: "flannel", "custom", and "none".
	//     "flannel" uses Talos-managed Flannel CNI, and that's the default option.
	//     "custom" uses custom manifests that should be provided in "urls".
	//     "none" indicates that Talos will not manage any CNI installation.
	//   examples:
	//     - value: clusterCustomCNIExample()
	CNI *CNIConfig `yaml:"cni,omitempty"`
	//   description: |
	//     The domain used by Kubernetes DNS.
	//     The default is `cluster.local`
	//   examples:
	//     - value: '"cluster.local"'
	DNSDomain string `yaml:"dnsDomain"`
	//   description: |
	//     The pod subnet CIDR.
	//   examples:
	//     -  value: >
	//          []string{"10.244.0.0/16"}
	PodSubnet []string `yaml:"podSubnets" merge:"replace"`
	//   description: |
	//     The service subnet CIDR.
	//   examples:
	//     -  value: >
	//          []string{"10.96.0.0/12"}
	ServiceSubnet []string `yaml:"serviceSubnets" merge:"replace"`
}

// CNIConfig represents the CNI configuration options.
type CNIConfig struct {
	//   description: |
	//     Name of CNI to use.
	//   values:
	//     - flannel
	//     - custom
	//     - none
	CNIName string `yaml:"name,omitempty"`
	//   description: |
	//     URLs containing manifests to apply for the CNI.
	//     Should be present for "custom", must be empty for "flannel" and "none".
	CNIUrls []string `yaml:"urls,omitempty"`
	//   description: |
	//		Flannel configuration options.
	CNIFlannel *FlannelCNIConfig `yaml:"flannel,omitempty"`
}

// FlannelCNIConfig represents the Flannel CNI configuration options.
type FlannelCNIConfig struct {
	//   description: |
	//     Extra arguments for 'flanneld'.
	//   examples:
	//     - value: >
	//         []string{"--iface-can-reach=192.168.1.1"}
	FlanneldExtraArgs []string `yaml:"extraArgs,omitempty"`
}

var _ config.ExternalCloudProvider = (*ExternalCloudProviderConfig)(nil)

// ExternalCloudProviderConfig contains external cloud provider configuration.
type ExternalCloudProviderConfig struct {
	//   description: |
	//     Enable external cloud provider.
	//   values:
	//     - true
	//     - yes
	//     - false
	//     - no
	ExternalEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     A list of urls that point to additional manifests for an external cloud provider.
	//     These will get automatically deployed as part of the bootstrap.
	//   examples:
	//     - value: >
	//        []string{
	//         "https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/rbac.yaml",
	//         "https://raw.githubusercontent.com/kubernetes/cloud-provider-aws/v1.20.0-alpha.0/manifests/aws-cloud-controller-manager-daemonset.yaml",
	//        }
	ExternalManifests []string `yaml:"manifests,omitempty"`
}

// AdminKubeconfigConfig contains admin kubeconfig settings.
type AdminKubeconfigConfig struct {
	//   description: |
	//     Admin kubeconfig certificate lifetime (default is 1 year).
	//     Field format accepts any Go time.Duration format ('1h' for one hour, '10m' for ten minutes).
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	AdminKubeconfigCertLifetime time.Duration `yaml:"certLifetime,omitempty"`
}

// MachineDisk represents the options available for partitioning, formatting, and
// mounting extra disks.
//
// docgen:nodoc
type MachineDisk struct {
	//   description: The name of the disk to use.
	DeviceName string `yaml:"device,omitempty"`
	//   description: A list of partitions to create on the disk.
	DiskPartitions []*DiskPartition `yaml:"partitions,omitempty"`
}

// DiskSize partition size in bytes.
//
// docgen:nodoc
type DiskSize uint64

// MarshalYAML write as human readable string.
func (ds DiskSize) MarshalYAML() (any, error) {
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
func (ds *DiskSize) UnmarshalYAML(unmarshal func(any) error) error {
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
// docgen:nodoc
type DiskPartition struct {
	//   description: >
	//     The size of partition: either bytes or human readable representation. If `size:`
	//     is omitted, the partition is sized to occupy the full disk.
	//   examples:
	//     - name: Human readable representation.
	//       value: DiskSize(100000000)
	//     - name: Precise value in bytes.
	//       value: 1024 * 1024 * 1024
	//   schema:
	//     type: integer
	DiskSize DiskSize `yaml:"size,omitempty"`
	//   description:
	//     Where to mount the partition.
	DiskMountPoint string `yaml:"mountpoint,omitempty"`
}

// EncryptionConfig represents partition encryption settings.
//
//docgen:nodoc
type EncryptionConfig struct {
	//   description: >
	//     Encryption provider to use for the encryption.
	//   examples:
	//     - value: '"luks2"'
	EncryptionProvider string `yaml:"provider"`
	//   description: >
	//     Defines the encryption keys generation and storage method.
	EncryptionKeys []*EncryptionKey `yaml:"keys"`
	//   description: >
	//     Cipher kind to use for the encryption.
	//     Depends on the encryption provider.
	//   values:
	//     - aes-xts-plain64
	//     - xchacha12,aes-adiantum-plain64
	//     - xchacha20,aes-adiantum-plain64
	//   examples:
	//     - value: '"aes-xts-plain64"'
	EncryptionCipher string `yaml:"cipher,omitempty"`
	//   description: >
	//     Defines the encryption key length.
	EncryptionKeySize uint `yaml:"keySize,omitempty"`
	//   description: >
	//     Defines the encryption sector size.
	//   examples:
	//     - value: '4096'
	EncryptionBlockSize uint64 `yaml:"blockSize,omitempty"`
	//   description: >
	//     Additional --perf parameters for the LUKS2 encryption.
	//   values:
	//     - no_read_workqueue
	//     - no_write_workqueue
	//     - same_cpu_crypt
	//   examples:
	//     -  value: >
	//          []string{"no_read_workqueue","no_write_workqueue"}
	EncryptionPerfOptions []string `yaml:"options,omitempty"`
}

// EncryptionKey represents configuration for disk encryption key.
//
//docgen:nodoc
type EncryptionKey struct {
	//   description: >
	//     Key which value is stored in the configuration file.
	KeyStatic *EncryptionKeyStatic `yaml:"static,omitempty"`
	//   description: >
	//     Deterministically generated key from the node UUID and PartitionLabel.
	KeyNodeID *EncryptionKeyNodeID `yaml:"nodeID,omitempty"`
	//   description: >
	//     KMS managed encryption key.
	//   examples:
	//     - value: kmsKeyExample()
	KeyKMS *EncryptionKeyKMS `yaml:"kms,omitempty"`
	//   description: >
	//     Key slot number for LUKS2 encryption.
	KeySlot int `yaml:"slot"`
	//   description: >
	//     Enable TPM based disk encryption.
	KeyTPM *EncryptionKeyTPM `yaml:"tpm,omitempty"`
}

// EncryptionKeyStatic represents throw away key type.
//
//docgen:nodoc
type EncryptionKeyStatic struct {
	//   description: >
	//     Defines the static passphrase value.
	KeyData string `yaml:"passphrase,omitempty"`
}

// EncryptionKeyKMS represents a key that is generated and then sealed/unsealed by the KMS server.
//
//docgen:nodoc
type EncryptionKeyKMS struct {
	//   description: >
	//     KMS endpoint to Seal/Unseal the key.
	KMSEndpoint string `yaml:"endpoint"`
}

// EncryptionKeyTPM represents a key that is generated and then sealed/unsealed by the TPM.
//
//docgen:nodoc
type EncryptionKeyTPM struct {
	//   description: >
	//     Check that Secureboot is enabled in the EFI firmware.
	//
	//     If Secureboot is not enabled, the enrollment of the key will fail.
	//     As the TPM key is anyways bound to the value of PCR 7,
	//     changing Secureboot status or configuration
	//     after the initial enrollment will make the key unusable.
	TPMCheckSecurebootStatusOnEnroll *bool `yaml:"checkSecurebootStatusOnEnroll,omitempty"`
}

// EncryptionKeyNodeID represents deterministically generated key from the node UUID and PartitionLabel.
//
//docgen:nodoc
type EncryptionKeyNodeID struct{}

// Env represents a set of environment variables.
type Env = map[string]string

// ResourcesConfig represents the pod resources.
type ResourcesConfig struct {
	//   description: |
	//     Requests configures the reserved cpu/memory resources.
	//   examples:
	//     - name: resources requests.
	//       value: resourcesConfigRequestsExample()
	//   schema:
	//     type: object
	Requests Unstructured `yaml:"requests,omitempty"`
	//   description: |
	//     Limits configures the maximum cpu/memory resources a container can use.
	//   examples:
	//     - name: resources requests.
	//       value: resourcesConfigLimitsExample()
	//   schema:
	//     type: object
	Limits Unstructured `yaml:"limits,omitempty"`
}

// FileMode represents file's permissions.
type FileMode os.FileMode

// String convert file mode to octal string.
func (fm FileMode) String() string {
	return "0o" + strconv.FormatUint(uint64(fm), 8)
}

// MarshalYAML encodes as an octal value.
func (fm FileMode) MarshalYAML() (any, error) {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: fm.String(),
	}, nil
}

// MachineFile represents a file to write to disk.
type MachineFile struct {
	//   description: The contents of the file.
	FileContent string `yaml:"content"`
	//   description: The file's permissions in octal.
	//   schema:
	//     type: integer
	FilePermissions FileMode `yaml:"permissions"`
	//   description: The path of the file.
	FilePath string `yaml:"path"`
	//   description: The operation to use
	//   values:
	//     - create
	//     - append
	//     - overwrite
	FileOp string `yaml:"op"`
}

// ExtraHost represents a host entry in /etc/hosts.
//
// docgen:nodoc
type ExtraHost struct {
	//   description: The IP of the host.
	HostIP string `yaml:"ip"`
	//   description: The host alias.
	HostAliases []string `yaml:"aliases"`
}

// Device represents a network interface.
//
// docgen:nodoc
type Device struct {
	//   description: |
	//     The interface name.
	//     Mutually exclusive with `deviceSelector`.
	//   examples:
	//     - value: '"enp0s3"'
	DeviceInterface string `yaml:"interface,omitempty"`
	//   description: |
	//     Picks a network device using the selector.
	//     Mutually exclusive with `interface`.
	//     Supports partial match using wildcard syntax.
	//   examples:
	//     - name: select a device with bus prefix 00:*.
	//       value: networkDeviceSelectorExamples()[0]
	//     - name: select a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
	//       value: networkDeviceSelectorExamples()[1]
	DeviceSelector *NetworkDeviceSelector `yaml:"deviceSelector,omitempty"`
	//   description: |
	//     Assigns static IP addresses to the interface.
	//     An address can be specified either in proper CIDR notation or as a standalone address (netmask of all ones is assumed).
	//   examples:
	//     - value: '[]string{"10.5.0.0/16", "192.168.3.7"}'
	DeviceAddresses []string `yaml:"addresses,omitempty"`
	// docgen:nodoc
	DeviceCIDR string `yaml:"cidr,omitempty"`
	//   description: |
	//     A list of routes associated with the interface.
	//     If used in combination with DHCP, these routes will be appended to routes returned by DHCP server.
	//   examples:
	//     - value: networkConfigRoutesExample()
	DeviceRoutes []*Route `yaml:"routes,omitempty"`
	//   description: Bond specific options.
	//   examples:
	//     - value: networkConfigBondExample()
	DeviceBond *Bond `yaml:"bond,omitempty"`
	//   description: Bridge specific options.
	//   examples:
	//     - value: networkConfigBridgeExample()
	DeviceBridge *Bridge `yaml:"bridge,omitempty"`
	//   description: |
	//     Configure this device as a bridge port.
	//     This can be used to dynamically assign network interfaces to a bridge.
	//   examples:
	//     - value: networkConfigDynamicBridgePortsExample()
	DeviceBridgePort *BridgePort `yaml:"bridgePort,omitempty"`
	//   description: VLAN specific options.
	DeviceVlans VlanList `yaml:"vlans,omitempty"`
	//   description: |
	//     The interface's MTU.
	//     If used in combination with DHCP, this will override any MTU settings returned from DHCP server.
	DeviceMTU int `yaml:"mtu,omitempty"`
	//   description: |
	//     Indicates if DHCP should be used to configure the interface.
	//     The following DHCP options are supported:
	//
	//     - `OptionClasslessStaticRoute`
	//     - `OptionDomainNameServer`
	//     - `OptionDNSDomainSearchList`
	//     - `OptionHostName`
	//
	//   examples:
	//     - value: true
	DeviceDHCP *bool `yaml:"dhcp,omitempty"`
	//   description: Indicates if the interface should be ignored (skips configuration).
	DeviceIgnore *bool `yaml:"ignore,omitempty"`
	//   description: |
	//     Indicates if the interface is a dummy interface.
	//     `dummy` is used to specify that this interface should be a virtual-only, dummy interface.
	DeviceDummy *bool `yaml:"dummy,omitempty"`
	//   description: |
	//     DHCP specific options.
	//     `dhcp` *must* be set to true for these to take effect.
	//   examples:
	//     - value: networkConfigDHCPOptionsExample()
	DeviceDHCPOptions *DHCPOptions `yaml:"dhcpOptions,omitempty"`
	//   description: |
	//     Wireguard specific configuration.
	//     Includes things like private key, listen port, peers.
	//   examples:
	//     - name: wireguard server example
	//       value: networkConfigWireguardHostExample()
	//     - name: wireguard peer example
	//       value: networkConfigWireguardPeerExample()
	DeviceWireguardConfig *DeviceWireguardConfig `yaml:"wireguard,omitempty"`
	//   description: Virtual (shared) IP address configuration.
	//   examples:
	//     - name: layer2 vip example
	//       value: networkConfigVIPLayer2Example()
	DeviceVIPConfig *DeviceVIPConfig `yaml:"vip,omitempty"`
}

// DHCPOptions contains options for configuring the DHCP settings for a given interface.
//
// docgen:nodoc
type DHCPOptions struct {
	//   description: The priority of all routes received via DHCP.
	DHCPRouteMetric uint32 `yaml:"routeMetric"`
	//   description: Enables DHCPv4 protocol for the interface (default is enabled).
	DHCPIPv4 *bool `yaml:"ipv4,omitempty"`
	//   description: Enables DHCPv6 protocol for the interface (default is disabled).
	DHCPIPv6 *bool `yaml:"ipv6,omitempty"`
	//   description: Set client DUID (hex string).
	DHCPDUIDv6 string `yaml:"duidv6,omitempty"`
}

// DeviceWireguardConfig contains settings for configuring Wireguard network interface.
//
// docgen:nodoc
type DeviceWireguardConfig struct {
	//   description: |
	//     Specifies a private key configuration (base64 encoded).
	//     Can be generated by `wg genkey`.
	WireguardPrivateKey string `yaml:"privateKey,omitempty"`
	//   description: Specifies a device's listening port.
	WireguardListenPort int `yaml:"listenPort,omitempty"`
	//   description: Specifies a device's firewall mark.
	WireguardFirewallMark int `yaml:"firewallMark,omitempty"`
	//   description: Specifies a list of peer configurations to apply to a device.
	WireguardPeers []*DeviceWireguardPeer `yaml:"peers,omitempty"`
}

// DeviceWireguardPeer a WireGuard device peer configuration.
//
// docgen:nodoc
type DeviceWireguardPeer struct {
	//   description: |
	//     Specifies the public key of this peer.
	//     Can be extracted from private key by running `wg pubkey < private.key > public.key && cat public.key`.
	WireguardPublicKey string `yaml:"publicKey,omitempty"`
	//   description: Specifies the endpoint of this peer entry.
	WireguardEndpoint string `yaml:"endpoint,omitempty"`
	//   description: |
	//     Specifies the persistent keepalive interval for this peer.
	//     Field format accepts any Go time.Duration format ('1h' for one hour, '10m' for ten minutes).
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	WireguardPersistentKeepaliveInterval time.Duration `yaml:"persistentKeepaliveInterval,omitempty"`
	//   description: AllowedIPs specifies a list of allowed IP addresses in CIDR notation for this peer.
	WireguardAllowedIPs []string `yaml:"allowedIPs,omitempty"`
}

// DeviceVIPConfig contains settings for configuring a Virtual Shared IP on an interface.
//
// docgen:nodoc
type DeviceVIPConfig struct {
	// description: Specifies the IP address to be used.
	SharedIP string `yaml:"ip,omitempty"`
	// description: Specifies the Equinix Metal API settings to assign VIP to the node.
	EquinixMetalConfig *VIPEquinixMetalConfig `yaml:"equinixMetal,omitempty"`
	// description: Specifies the Hetzner Cloud API settings to assign VIP to the node.
	HCloudConfig *VIPHCloudConfig `yaml:"hcloud,omitempty"`
}

// VIPEquinixMetalConfig contains settings for Equinix Metal VIP management.
//
// docgen:nodoc
type VIPEquinixMetalConfig struct {
	// description: Specifies the Equinix Metal API Token.
	EquinixMetalAPIToken string `yaml:"apiToken"`
}

// VIPHCloudConfig contains settings for Hetzner Cloud VIP management.
//
// docgen:nodoc
type VIPHCloudConfig struct {
	// description: Specifies the Hetzner Cloud API Token.
	HCloudAPIToken string `yaml:"apiToken"`
}

// Bond contains the various options for configuring a bonded interface.
//
// docgen:nodoc
type Bond struct {
	//   description: The interfaces that make up the bond.
	BondInterfaces []string `yaml:"interfaces"`
	//   description: |
	//     Picks a network device using the selector.
	//     Mutually exclusive with `interfaces`.
	//     Supports partial match using wildcard syntax.
	//   examples:
	//     - name: select a device with bus prefix 00:*, a device with mac address matching `*:f0:ab` and `virtio` kernel driver.
	//       value: networkDeviceSelectorExamples()
	BondDeviceSelectors []NetworkDeviceSelector `yaml:"deviceSelectors,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	//     Not supported at the moment.
	BondARPIPTarget []string `yaml:"arpIPTarget,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondMode string `yaml:"mode"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondHashPolicy string `yaml:"xmitHashPolicy,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondLACPRate string `yaml:"lacpRate,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	//     Not supported at the moment.
	BondADActorSystem string `yaml:"adActorSystem,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondARPValidate string `yaml:"arpValidate,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondARPAllTargets string `yaml:"arpAllTargets,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondPrimary string `yaml:"primary,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondPrimaryReselect string `yaml:"primaryReselect,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondFailOverMac string `yaml:"failOverMac,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondADSelect string `yaml:"adSelect,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondMIIMon uint32 `yaml:"miimon,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondUpDelay uint32 `yaml:"updelay,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondDownDelay uint32 `yaml:"downdelay,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondARPInterval uint32 `yaml:"arpInterval,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondResendIGMP uint32 `yaml:"resendIgmp,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondMinLinks uint32 `yaml:"minLinks,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondLPInterval uint32 `yaml:"lpInterval,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondPacketsPerSlave uint32 `yaml:"packetsPerSlave,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondNumPeerNotif uint8 `yaml:"numPeerNotif,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondTLBDynamicLB uint8 `yaml:"tlbDynamicLb,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondAllSlavesActive uint8 `yaml:"allSlavesActive,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondUseCarrier *bool `yaml:"useCarrier,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondADActorSysPrio uint16 `yaml:"adActorSysPrio,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondADUserPortKey uint16 `yaml:"adUserPortKey,omitempty"`
	//   description: |
	//     A bond option.
	//     Please see the official kernel documentation.
	BondPeerNotifyDelay uint32 `yaml:"peerNotifyDelay,omitempty"`
}

// STP contains the various options for configuring the STP properties of a bridge interface.
//
// docgen:nodoc
type STP struct {
	//   description: Whether Spanning Tree Protocol (STP) is enabled.
	STPEnabled *bool `yaml:"enabled,omitempty"`
}

// BridgeVLAN contains the various options for configuring the VLAN properties of a bridge interface.
//
// docgen:nodoc
type BridgeVLAN struct {
	//   description: Whether VLAN filtering is enabled.
	BridgeVLANFiltering *bool `yaml:"vlanFiltering,omitempty"`
}

// Bridge contains the various options for configuring a bridge interface.
//
// docgen:nodoc
type Bridge struct {
	//   description: The interfaces that make up the bridge.
	BridgedInterfaces []string `yaml:"interfaces"`
	//   description: |
	//     Enable STP on this bridge.
	//     Please see the official kernel documentation.
	BridgeSTP *STP `yaml:"stp,omitempty"`
	//   description: |
	//     Enable VLAN-awareness on this bridge.
	//     Please see the official kernel documentation.
	BridgeVLAN *BridgeVLAN `yaml:"vlan,omitempty"`
}

// BridgePort contains settings for assigning a link to a bridge interface.
//
// docgen:nodoc
type BridgePort struct {
	//   description: The name of the bridge master interface
	BridgePortMaster string `yaml:"master,omitempty"`
}

// VlanList is a list of *Vlan structures with overridden merge process.
//
// docgen:nodoc
type VlanList []*Vlan

// Merge the network interface configuration intelligently.
func (vlans *VlanList) Merge(other any) error {
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

// Vlan represents vlan settings for a device.
//
// docgen:nodoc
type Vlan struct {
	//   description: The addresses in CIDR notation or as plain IPs to use.
	VlanAddresses []string `yaml:"addresses,omitempty"`
	// docgen:nodoc
	VlanCIDR string `yaml:"cidr,omitempty"`
	//   description: A list of routes associated with the VLAN.
	VlanRoutes []*Route `yaml:"routes"`
	//   description: Indicates if DHCP should be used.
	VlanDHCP *bool `yaml:"dhcp,omitempty"`
	//   description: The VLAN's ID.
	VlanID uint16 `yaml:"vlanId"`
	//   description: The VLAN's MTU.
	VlanMTU uint32 `yaml:"mtu,omitempty"`
	//   description: The VLAN's virtual IP address configuration.
	VlanVIP *DeviceVIPConfig `yaml:"vip,omitempty"`
	//   description: |
	//     DHCP specific options.
	//     `dhcp` *must* be set to true for these to take effect.
	VlanDHCPOptions *DHCPOptions `yaml:"dhcpOptions,omitempty"`
}

// Route represents a network route.
//
// docgen:nodoc
type Route struct {
	//   description: The route's network (destination).
	RouteNetwork string `yaml:"network"`
	//   description: The route's gateway (if empty, creates link scope route).
	RouteGateway string `yaml:"gateway"`
	//   description: The route's source address (optional).
	RouteSource string `yaml:"source,omitempty"`
	//   description: The optional metric for the route.
	RouteMetric uint32 `yaml:"metric,omitempty"`
	//   description: The optional MTU for the route.
	RouteMTU uint32 `yaml:"mtu,omitempty"`
}

// RegistryMirrorConfig represents mirror configuration for a registry.
type RegistryMirrorConfig struct {
	//   description: |
	//     List of endpoints (URLs) for registry mirrors to use.
	//     Endpoint configures HTTP/HTTPS access mode, host name,
	//     port and path (if path is not set, it defaults to `/v2`).
	MirrorEndpoints []string `yaml:"endpoints"`
	//   description: |
	//     Use the exact path specified for the endpoint (don't append /v2/).
	//     This setting is often required for setting up multiple mirrors
	//     on a single instance of a registry.
	MirrorOverridePath *bool `yaml:"overridePath,omitempty"`
	//   description: |
	//     Skip fallback to the upstream endpoint, for example the mirror configuration
	//     for `docker.io` will not fallback to `registry-1.docker.io`.
	MirrorSkipFallback *bool `yaml:"skipFallback,omitempty"`
}

// RegistryConfig specifies auth & TLS config per registry.
type RegistryConfig struct {
	//   description: |
	//     The TLS configuration for the registry.
	//   examples:
	//     - value: machineConfigRegistryTLSConfigExample1()
	//     - value: machineConfigRegistryTLSConfigExample2()
	RegistryTLS *RegistryTLSConfig `yaml:"tls,omitempty"`
	//   description: |
	//     The auth configuration for this registry.
	//     Note: changes to the registry auth will not be picked up by the CRI containerd plugin without a reboot.
	//   examples:
	//     - value: machineConfigRegistryAuthConfigExample()
	RegistryAuth *RegistryAuthConfig `yaml:"auth,omitempty"`
}

// RegistryAuthConfig specifies authentication configuration for a registry.
type RegistryAuthConfig struct {
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in [`.docker/config.json`](https://docs.docker.com/engine/api/v1.41/#section/Authentication).
	RegistryUsername string `yaml:"username,omitempty"`
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in [`.docker/config.json`](https://docs.docker.com/engine/api/v1.41/#section/Authentication).
	RegistryPassword string `yaml:"password,omitempty"`
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in [`.docker/config.json`](https://docs.docker.com/engine/api/v1.41/#section/Authentication).
	RegistryAuth string `yaml:"auth,omitempty"`
	//   description: |
	//     Optional registry authentication.
	//     The meaning of each field is the same with the corresponding field in [`.docker/config.json`](https://docs.docker.com/engine/api/v1.41/#section/Authentication).
	RegistryIdentityToken string `yaml:"identityToken,omitempty"`
}

// RegistryTLSConfig specifies TLS config for HTTPS registries.
type RegistryTLSConfig struct {
	//   description: |
	//     Enable mutual TLS authentication with the registry.
	//     Client certificate and key should be base64-encoded.
	//   examples:
	//     - value: pemEncodedCertificateExample()
	//   schema:
	//     type: object
	//     additionalProperties: false
	//     properties:
	//       crt:
	//         type: string
	//       key:
	//         type: string
	TLSClientIdentity *x509.PEMEncodedCertificateAndKey `yaml:"clientIdentity,omitempty"`
	//   description: |
	//     CA registry certificate to add the list of trusted certificates.
	//     Certificate should be base64-encoded.
	//   schema:
	//     type: string
	TLSCA Base64Bytes `yaml:"ca,omitempty"`
	//   description: |
	//     Skip TLS server certificate verification (not recommended).
	TLSInsecureSkipVerify *bool `yaml:"insecureSkipVerify,omitempty"`
}

// SystemDiskEncryptionConfig specifies system disk partitions encryption settings.
//
//docgen:nodoc
type SystemDiskEncryptionConfig struct {
	//   description: |
	//     State partition encryption.
	StatePartition *EncryptionConfig `yaml:"state,omitempty"`
	//   description: |
	//     Ephemeral partition encryption.
	EphemeralPartition *EncryptionConfig `yaml:"ephemeral,omitempty"`
}

var _ config.Features = (*FeaturesConfig)(nil)

// FeaturesConfig describes individual Talos features that can be switched on or off.
type FeaturesConfig struct {
	// docgen:nodoc
	RBAC *bool `yaml:"rbac,omitempty"`
	// docgen:nodoc
	//
	// Deprecated: use HostConfig instead.
	StableHostname *bool `yaml:"stableHostname,omitempty"`
	//   description: |
	//    Configure Talos API access from Kubernetes pods.
	//
	//    This feature is disabled if the feature config is not specified.
	//   examples:
	//     - value: kubernetesTalosAPIAccessConfigExample()
	KubernetesTalosAPIAccessConfig *KubernetesTalosAPIAccessConfig `yaml:"kubernetesTalosAPIAccess,omitempty"`
	// docgen:nodoc
	ApidCheckExtKeyUsage *bool `yaml:"apidCheckExtKeyUsage,omitempty"`
	//   description: |
	//     Enable XFS project quota support for EPHEMERAL partition and user disks.
	//     Also enables kubelet tracking of ephemeral disk usage in the kubelet via quota.
	DiskQuotaSupport *bool `yaml:"diskQuotaSupport,omitempty"`
	//   description: |
	//     KubePrism - local proxy/load balancer on defined port that will distribute
	//     requests to all API servers in the cluster.
	KubePrismSupport *KubePrism `yaml:"kubePrism,omitempty"`
	//   description: |
	//     Configures host DNS caching resolver.
	HostDNSSupport *HostDNSConfig `yaml:"hostDNS,omitempty"`
	//   description: |
	//     Enable Image Cache feature.
	ImageCacheSupport *ImageCacheConfig `yaml:"imageCache,omitempty"`
	//   description: |
	//     Select the node address sort algorithm.
	//     The 'v1' algorithm sorts addresses by the address itself.
	//     The 'v2' algorithm prefers more specific prefixes.
	//     If unset, defaults to 'v1'.
	FeatureNodeAddressSortAlgorithm string `yaml:"nodeAddressSortAlgorithm,omitempty"`
}

// KubePrism describes the configuration for the KubePrism load balancer.
type KubePrism struct {
	//   description: |
	//     Enable KubePrism support - will start local load balancing proxy.
	ServerEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     KubePrism port.
	ServerPort int `yaml:"port,omitempty"`
}

// ImageCacheConfig describes the configuration for the Image Cache feature.
type ImageCacheConfig struct {
	//   description: |
	//     Enable local image cache.
	CacheLocalEnabled *bool `yaml:"localEnabled,omitempty"`
}

// KubernetesTalosAPIAccessConfig describes the configuration for the Talos API access from Kubernetes pods.
type KubernetesTalosAPIAccessConfig struct {
	//   description: |
	//     Enable Talos API access from Kubernetes pods.
	AccessEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     The list of Talos API roles which can be granted for access from Kubernetes pods.
	//
	//     Empty list means that no roles can be granted, so access is blocked.
	AccessAllowedRoles []string `yaml:"allowedRoles,omitempty"`
	//   description: |
	//     The list of Kubernetes namespaces Talos API access is available from.
	AccessAllowedKubernetesNamespaces []string `yaml:"allowedKubernetesNamespaces,omitempty"`
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
type VolumeMountConfig struct {
	//   description: |
	//     Path on the host.
	//   examples:
	//     - value: '"/var/lib/auth"'
	VolumeHostPath string `yaml:"hostPath"`
	//   description: |
	//     Path in the container.
	//   examples:
	//     - value: '"/etc/kubernetes/auth"'
	VolumeMountPath string `yaml:"mountPath"`
	//   description: |
	//     Mount the volume read only.
	//   examples:
	//     - value: true
	VolumeReadOnly bool `yaml:"readonly,omitempty"`
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
type ClusterInlineManifest struct {
	//   description: |
	//     Name of the manifest.
	//     Name should be unique.
	//   examples:
	//     - value: '"csi"'
	InlineManifestName string `yaml:"name"`
	//   description: |
	//     Manifest contents as a string.
	//   examples:
	//     - value: '"/etc/kubernetes/auth"'
	InlineManifestContents string `yaml:"contents"`
}

// NetworkKubeSpan struct describes KubeSpan configuration.
type NetworkKubeSpan struct {
	// description: |
	//   Enable the KubeSpan feature.
	//   Cluster discovery should be enabled with .cluster.discovery.enabled for KubeSpan to be enabled.
	KubeSpanEnabled *bool `yaml:"enabled,omitempty"`
	// description: |
	//   Control whether Kubernetes pod CIDRs are announced over KubeSpan from the node.
	//   If disabled, CNI handles encapsulating pod-to-pod traffic into some node-to-node tunnel,
	//   and KubeSpan handles the node-to-node traffic.
	//   If enabled, KubeSpan will take over pod-to-pod traffic and send it over KubeSpan directly.
	//   When enabled, KubeSpan should have a way to detect complete pod CIDRs of the node which
	//   is not always the case with CNIs not relying on Kubernetes for IPAM.
	KubeSpanAdvertiseKubernetesNetworks *bool `yaml:"advertiseKubernetesNetworks,omitempty"`
	// description: |
	//   Skip sending traffic via KubeSpan if the peer connection state is not up.
	//   This provides configurable choice between connectivity and security: either traffic is always
	//   forced to go via KubeSpan (even if Wireguard peer connection is not up), or traffic can go directly
	//   to the peer if Wireguard connection can't be established.
	KubeSpanAllowDownPeerBypass *bool `yaml:"allowDownPeerBypass,omitempty"`
	// description: |
	//   KubeSpan can collect and publish extra endpoints for each member of the cluster
	//   based on Wireguard endpoint information for each peer.
	//   This feature is disabled by default, don't enable it
	//   with high number of peers (>50) in the KubeSpan network (performance issues).
	KubeSpanHarvestExtraEndpoints *bool `yaml:"harvestExtraEndpoints,omitempty"`
	// description: |
	//   KubeSpan link MTU size.
	//   Default value is 1420.
	KubeSpanMTU *uint32 `yaml:"mtu,omitempty"`
	// description: |
	//   KubeSpan advanced filtering of network addresses .
	//
	//   Settings in this section are optional, and settings apply only to the node.
	KubeSpanFilters *KubeSpanFilters `yaml:"filters,omitempty"`
}

// KubeSpanFilters struct describes KubeSpan advanced network addresses filtering.
type KubeSpanFilters struct {
	// description: |
	//   Filter node addresses which will be advertised as KubeSpan endpoints for peer-to-peer Wireguard connections.
	//
	//   By default, all addresses are advertised, and KubeSpan cycles through all endpoints until it finds one that works.
	//
	//   Default value: no filtering.
	// examples:
	//   - name: Exclude addresses in 192.168.0.0/16 subnet.
	//     value: '[]string{"0.0.0.0/0", "!192.168.0.0/16", "::/0"}'
	KubeSpanFiltersEndpoints []string `yaml:"endpoints,omitempty"`
}

// NetworkDeviceSelector struct describes network device selector.
//
// docgen:nodoc
type NetworkDeviceSelector struct {
	// description: PCI, USB bus prefix, supports matching by wildcard.
	NetworkDeviceBus string `yaml:"busPath,omitempty"`
	// description: Device hardware (MAC) address, supports matching by wildcard.
	NetworkDeviceHardwareAddress string `yaml:"hardwareAddr,omitempty"`
	// description: |
	//    Device permanent hardware address, supports matching by wildcard.
	//    The permanent address doesn't change when the link is enslaved to a bond,
	//    so it's recommended to use this field for bond members.
	NetworkDevicePermanentAddress string `yaml:"permanentAddr,omitempty"`
	// description: PCI ID (vendor ID, product ID), supports matching by wildcard.
	NetworkDevicePCIID string `yaml:"pciID,omitempty"`
	// description: Kernel driver, supports matching by wildcard.
	NetworkDeviceKernelDriver string `yaml:"driver,omitempty"`
	// description: Select only physical devices.
	NetworkDevicePhysical *bool `yaml:"physical,omitempty"`
}

// ClusterDiscoveryConfig struct configures cluster membership discovery.
type ClusterDiscoveryConfig struct {
	// description: |
	//   Enable the cluster membership discovery feature.
	//   Cluster discovery is based on individual registries which are configured under the registries field.
	DiscoveryEnabled *bool `yaml:"enabled,omitempty"`
	// description: |
	//   Configure registries used for cluster member discovery.
	DiscoveryRegistries DiscoveryRegistriesConfig `yaml:"registries"`
}

// DiscoveryRegistriesConfig struct configures cluster membership discovery.
type DiscoveryRegistriesConfig struct {
	// description: |
	//   Kubernetes registry uses Kubernetes API server to discover cluster members and stores additional information
	//   as annotations on the Node resources.
	//
	//   This feature is deprecated as it is not compatible with Kubernetes 1.32+.
	//   See https://github.com/siderolabs/talos/issues/9980 for more information.
	RegistryKubernetes RegistryKubernetesConfig `yaml:"kubernetes"`
	// description: |
	//   Service registry is using an external service to push and pull information about cluster members.
	RegistryService RegistryServiceConfig `yaml:"service"`
}

// RegistryKubernetesConfig struct configures Kubernetes discovery registry.
type RegistryKubernetesConfig struct {
	// description: |
	//   Disable Kubernetes discovery registry.
	RegistryDisabled *bool `yaml:"disabled,omitempty"`
}

// RegistryServiceConfig struct configures Kubernetes discovery registry.
type RegistryServiceConfig struct {
	// description: |
	//   Disable external service discovery registry.
	RegistryDisabled *bool `yaml:"disabled,omitempty"`
	// description: |
	//   External service endpoint.
	// examples:
	//   - value: constants.DefaultDiscoveryServiceEndpoint
	RegistryEndpoint string `yaml:"endpoint,omitempty"`
}

// UdevConfig describes how the udev system should be configured.
type UdevConfig struct {
	//   description: |
	//     List of udev rules to apply to the udev system
	UdevRules []string `yaml:"rules,omitempty"`
}

// LoggingConfig struct configures Talos logging.
type LoggingConfig struct {
	// description: |
	//   Logging destination.
	LoggingDestinations []LoggingDestination `yaml:"destinations"`
}

// LoggingDestination struct configures Talos logging destination.
type LoggingDestination struct {
	// description: |
	//   Where to send logs. Supported protocols are "tcp" and "udp".
	// examples:
	//   - value: loggingEndpointExample1()
	//   - value: loggingEndpointExample2()
	LoggingEndpoint *Endpoint `yaml:"endpoint"`
	// description: |
	//   Logs format.
	// values:
	//   - json_lines
	LoggingFormat string `yaml:"format"`
	// description: |
	//   Extra tags (key-value) pairs to attach to every log message sent.
	LoggingExtraTags map[string]string `yaml:"extraTags,omitempty"`
}

// KernelConfig struct configures Talos Linux kernel.
type KernelConfig struct {
	// description: |
	//   Kernel modules to load.
	KernelModules []*KernelModuleConfig `yaml:"modules,omitempty"`
}

// KernelModuleConfig struct configures Linux kernel modules to load.
type KernelModuleConfig struct {
	// description: |
	//   Module name.
	ModuleName string `yaml:"name"`
	// description: |
	//   Module parameters, changes applied after reboot.
	ModuleParameters []string `yaml:"parameters,omitempty"`
}
