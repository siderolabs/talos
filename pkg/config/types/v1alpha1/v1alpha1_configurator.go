// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/kubernetes-sigs/bootkube/pkg/asset"
	"github.com/opencontainers/runtime-spec/specs-go"

	criplugin "github.com/talos-systems/talos/internal/pkg/containers/cri/containerd"
	"github.com/talos-systems/talos/pkg/config/cluster"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

const (
	// Version is the version string for v1alpha1.
	Version = "v1alpha1"
)

// Version implements the Configurator interface.
func (c *Config) Version() string {
	return Version
}

// Debug implements the Configurator interface.
func (c *Config) Debug() bool {
	return false
}

// Machine implements the Configurator interface.
func (c *Config) Machine() machine.Machine {
	return c.MachineConfig
}

// Cluster implements the Configurator interface.
func (c *Config) Cluster() cluster.Cluster {
	return c.ClusterConfig
}

// String implements the Configurator interface.
func (c *Config) String() (string, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Install implements the Configurator interface.
func (m *MachineConfig) Install() machine.Install {
	if m.MachineInstall == nil {
		return &InstallConfig{}
	}

	return m.MachineInstall
}

// Security implements the Configurator interface.
func (m *MachineConfig) Security() machine.Security {
	return m
}

// Disks implements the Configurator interface.
func (m *MachineConfig) Disks() []machine.Disk {
	return m.MachineDisks
}

// Network implements the Configurator interface.
func (m *MachineConfig) Network() machine.Network {
	if m.MachineNetwork == nil {
		return &NetworkConfig{}
	}

	return m.MachineNetwork
}

// Time implements the Configurator interface.
func (m *MachineConfig) Time() machine.Time {
	if m.MachineTime == nil {
		return &TimeConfig{}
	}

	return m.MachineTime
}

// Kubelet implements the Configurator interface.
func (m *MachineConfig) Kubelet() machine.Kubelet {
	return m.MachineKubelet
}

// Env implements the Configurator interface.
func (m *MachineConfig) Env() machine.Env {
	return m.MachineEnv
}

// Files implements the Configurator interface.
func (m *MachineConfig) Files() ([]machine.File, error) {
	files, err := m.Registries().ExtraFiles()

	return append(files, m.MachineFiles...), err
}

// Type implements the Configurator interface.
func (m *MachineConfig) Type() machine.Type {
	switch m.MachineType {
	case "init":
		return machine.TypeInit
	case "controlplane":
		return machine.TypeControlPlane
	default:
		return machine.TypeWorker
	}
}

// Server implements the Configurator interface.
func (m *MachineConfig) Server() string {
	return ""
}

// Sysctls implements the Configurator interface.
func (m *MachineConfig) Sysctls() map[string]string {
	if m.MachineSysctls == nil {
		m.MachineSysctls = make(map[string]string)
	}

	return m.MachineSysctls
}

// CA implements the Configurator interface.
func (m *MachineConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return m.MachineCA
}

// Token implements the Configurator interface.
func (m *MachineConfig) Token() string {
	return m.MachineToken
}

// CertSANs implements the Configurator interface.
func (m *MachineConfig) CertSANs() []string {
	return m.MachineCertSANs
}

// SetCertSANs implements the Configurator interface.
func (m *MachineConfig) SetCertSANs(sans []string) {
	m.MachineCertSANs = append(m.MachineCertSANs, sans...)
}

// Registries implements the Configurator interface.
func (m *MachineConfig) Registries() machine.Registries {
	return &m.MachineRegistries
}

// Image implements the Configurator interface.
func (k *KubeletConfig) Image() string {
	image := k.KubeletImage

	if image == "" {
		image = fmt.Sprintf("%s:v%s", constants.KubernetesImage, constants.DefaultKubernetesVersion)
	}

	return image
}

// ExtraArgs implements the Configurator interface.
func (k *KubeletConfig) ExtraArgs() map[string]string {
	if k == nil {
		k = &KubeletConfig{}
	}

	if k.KubeletExtraArgs == nil {
		k.KubeletExtraArgs = make(map[string]string)
	}

	return k.KubeletExtraArgs
}

// ExtraMounts implements the Configurator interface.
func (k *KubeletConfig) ExtraMounts() []specs.Mount {
	return k.KubeletExtraMounts
}

// Name implements the Configurator interface.
func (c *ClusterConfig) Name() string {
	return c.ClusterName
}

// Endpoint implements the Configurator interface.
func (c *ClusterConfig) Endpoint() *url.URL {
	return c.ControlPlane.Endpoint.URL
}

// LocalAPIServerPort implements the Configurator interface.
func (c *ClusterConfig) LocalAPIServerPort() int {
	if c.ControlPlane.LocalAPIServerPort == 0 {
		return 6443
	}

	return c.ControlPlane.LocalAPIServerPort
}

// CertSANs implements the Configurator interface.
func (c *ClusterConfig) CertSANs() []string {
	return c.APIServerConfig.CertSANs
}

// SetCertSANs implements the Configurator interface.
func (c *ClusterConfig) SetCertSANs(sans []string) {
	if c.APIServerConfig == nil {
		c.APIServerConfig = &APIServerConfig{}
	}

	c.APIServerConfig.CertSANs = append(c.APIServerConfig.CertSANs, sans...)
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
	return "", nil
}

// APIServer implements the Configurator interface.
func (c *ClusterConfig) APIServer() cluster.APIServer {
	if c.APIServerConfig == nil {
		return &APIServerConfig{}
	}

	return c.APIServerConfig
}

// ExtraArgs implements the Configurator interface.
func (a *APIServerConfig) ExtraArgs() map[string]string {
	return a.ExtraArgsConfig
}

// ControllerManager implements the Configurator interface.
func (c *ClusterConfig) ControllerManager() cluster.ControllerManager {
	if c.ControllerManagerConfig == nil {
		return &ControllerManagerConfig{}
	}

	return c.ControllerManagerConfig
}

// ExtraArgs implements the Configurator interface.
func (c *ControllerManagerConfig) ExtraArgs() map[string]string {
	return c.ExtraArgsConfig
}

// Scheduler implements the Configurator interface.
func (c *ClusterConfig) Scheduler() cluster.Scheduler {
	if c.SchedulerConfig == nil {
		return &SchedulerConfig{}
	}

	return c.SchedulerConfig
}

// AdminKubeconfig implements the Configurator interface.
func (c *ClusterConfig) AdminKubeconfig() cluster.AdminKubeconfig {
	return c.AdminKubeconfigConfig
}

// ExtraArgs implements the Configurator interface.
func (s *SchedulerConfig) ExtraArgs() map[string]string {
	return s.ExtraArgsConfig
}

// Etcd implements the Configurator interface.
func (c *ClusterConfig) Etcd() cluster.Etcd {
	return c.EtcdConfig
}

// Image implements the Configurator interface.
func (e *EtcdConfig) Image() string {
	image := e.ContainerImage

	if image == "" {
		image = fmt.Sprintf("%s:%s", constants.EtcdImage, constants.DefaultEtcdVersion)
	}

	return image
}

// CA implements the Configurator interface.
func (e *EtcdConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return e.RootCA
}

// ExtraArgs implements the Configurator interface.
func (e *EtcdConfig) ExtraArgs() map[string]string {
	if e.EtcdExtraArgs == nil {
		e.EtcdExtraArgs = make(map[string]string)
	}

	return e.EtcdExtraArgs
}

// Mirrors implements the Registries interface.
func (r *RegistriesConfig) Mirrors() map[string]machine.RegistryMirrorConfig {
	return r.RegistryMirrors
}

// Config implements the Registries interface.
func (r *RegistriesConfig) Config() map[string]machine.RegistryConfig {
	return r.RegistryConfig
}

// ExtraFiles implements the Registries interface.
func (r *RegistriesConfig) ExtraFiles() ([]machine.File, error) {
	return criplugin.GenerateRegistriesConfig(r)
}

// Token implements the Configurator interface.
func (c *ClusterConfig) Token() cluster.Token {
	return c
}

// ID implements the Configurator interface.
func (c *ClusterConfig) ID() string {
	parts := strings.Split(c.BootstrapToken, ".")
	if len(parts) != 2 {
		return ""
	}

	return parts[0]
}

// Secret implements the Configurator interface.
func (c *ClusterConfig) Secret() string {
	parts := strings.Split(c.BootstrapToken, ".")
	if len(parts) != 2 {
		return ""
	}

	return parts[1]
}

// Network implements the Configurator interface.
func (c *ClusterConfig) Network() cluster.Network {
	return c
}

// CNI implements the Configurator interface.
func (c *ClusterConfig) CNI() cluster.CNI {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough

	case c.ClusterNetwork.CNI == nil:
		return &CNIConfig{
			CNIName: constants.DefaultCNI,
		}
	}

	return c.ClusterNetwork.CNI
}

// PodCIDR implements the Configurator interface.
func (c *ClusterConfig) PodCIDR() string {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.PodSubnet) == 0:
		return constants.DefaultPodCIDR
	}

	return strings.Join(c.ClusterNetwork.PodSubnet, ",")
}

// ServiceCIDR implements the Configurator interface.
func (c *ClusterConfig) ServiceCIDR() string {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.ServiceSubnet) == 0:
		return constants.DefaultServiceCIDR
	}

	return strings.Join(c.ClusterNetwork.ServiceSubnet, ",")
}

// ExtraManifestURLs implements the Configurator interface.
func (c *ClusterConfig) ExtraManifestURLs() []string {
	return c.ExtraManifests
}

// PodCheckpointer implements the Configurator interface.
func (c *ClusterConfig) PodCheckpointer() cluster.PodCheckpointer {
	if c.PodCheckpointerConfig == nil {
		return &PodCheckpointer{}
	}

	return c.PodCheckpointerConfig
}

// CoreDNS implements the Configurator interface.
func (c *ClusterConfig) CoreDNS() cluster.CoreDNS {
	if c.CoreDNSConfig == nil {
		return &CoreDNS{}
	}

	return c.CoreDNSConfig
}

// Name implements the Configurator interface.
func (c *CNIConfig) Name() string {
	return c.CNIName
}

// URLs implements the Configurator interface.
func (c *CNIConfig) URLs() []string {
	return c.CNIUrls
}

// Hostname implements the Configurator interface.
func (n *NetworkConfig) Hostname() string {
	return n.NetworkHostname
}

// SetHostname implements the Configurator interface.
func (n *NetworkConfig) SetHostname(hostname string) {
	n.NetworkHostname = hostname
}

// Devices implements the Configurator interface.
func (n *NetworkConfig) Devices() []machine.Device {
	return n.NetworkInterfaces
}

// Resolvers implements the Configurator interface.
func (n *NetworkConfig) Resolvers() []string {
	return n.NameServers
}

// Servers implements the Configurator interface.
func (t *TimeConfig) Servers() []string {
	return t.TimeServers
}

// Image implements the Configurator interface.
func (i *InstallConfig) Image() string {
	return i.InstallImage
}

// Disk implements the Configurator interface.
func (i *InstallConfig) Disk() string {
	return i.InstallDisk
}

// ExtraKernelArgs implements the Configurator interface.
func (i *InstallConfig) ExtraKernelArgs() []string {
	return i.InstallExtraKernelArgs
}

// Zero implements the Configurator interface.
func (i *InstallConfig) Zero() bool {
	return i.InstallWipe
}

// Force implements the Configurator interface.
func (i *InstallConfig) Force() bool {
	return i.InstallForce
}

// WithBootloader implements the Configurator interface.
func (i *InstallConfig) WithBootloader() bool {
	return i.InstallBootloader
}

// Image implements the Configurator interface.
func (c *CoreDNS) Image() string {
	coreDNSImage := asset.DefaultImages.CoreDNS

	if c.CoreDNSImage != "" {
		coreDNSImage = c.CoreDNSImage
	}

	return coreDNSImage
}

// Image implements the Configurator interface.
func (p *PodCheckpointer) Image() string {
	checkpointerImage := constants.PodCheckpointerImage

	if p.PodCheckpointerImage != "" {
		checkpointerImage = p.PodCheckpointerImage
	}

	return checkpointerImage
}

// CertLifetime implements the Configurator interface.
func (a AdminKubeconfigConfig) CertLifetime() time.Duration {
	if a.AdminKubeconfigCertLifetime == 0 {
		return constants.KubernetesAdminCertDefaultLifetime
	}

	return a.AdminKubeconfigCertLifetime
}
