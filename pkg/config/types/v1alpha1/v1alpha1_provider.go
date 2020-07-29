// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	"net/url"
	goruntime "runtime"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v3"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/bootkube-plugin/pkg/asset"

	criplugin "github.com/talos-systems/talos/internal/pkg/containers/cri/containerd"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

const (
	// Version is the version string for v1alpha1.
	Version = "v1alpha1"
)

// Version implements the config.Provider interface.
func (c *Config) Version() string {
	return Version
}

// Debug implements the config.Provider interface.
func (c *Config) Debug() bool {
	return c.ConfigDebug
}

// Persist implements the config.Provider interface.
func (c *Config) Persist() bool {
	return c.ConfigPersist
}

// Machine implements the config.Provider interface.
func (c *Config) Machine() config.MachineConfig {
	return c.MachineConfig
}

// Cluster implements the config.Provider interface.
func (c *Config) Cluster() config.ClusterConfig {
	return c.ClusterConfig
}

// String implements the config.Provider interface.
func (c *Config) String() (string, error) {
	b, err := c.Bytes()
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Bytes implements the config.Provider interface.
func (c *Config) Bytes() ([]byte, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// Install implements the config.Provider interface.
func (m *MachineConfig) Install() config.Install {
	if m.MachineInstall == nil {
		return &InstallConfig{}
	}

	return m.MachineInstall
}

// Security implements the config.Provider interface.
func (m *MachineConfig) Security() config.Security {
	return m
}

// Disks implements the config.Provider interface.
func (m *MachineConfig) Disks() []config.Disk {
	return m.MachineDisks
}

// Network implements the config.Provider interface.
func (m *MachineConfig) Network() config.MachineNetwork {
	if m.MachineNetwork == nil {
		return &NetworkConfig{}
	}

	return m.MachineNetwork
}

// Time implements the config.Provider interface.
func (m *MachineConfig) Time() config.Time {
	if m.MachineTime == nil {
		return &TimeConfig{}
	}

	return m.MachineTime
}

// Kubelet implements the config.Provider interface.
func (m *MachineConfig) Kubelet() config.Kubelet {
	return m.MachineKubelet
}

// Env implements the config.Provider interface.
func (m *MachineConfig) Env() config.Env {
	return m.MachineEnv
}

// Files implements the config.Provider interface.
func (m *MachineConfig) Files() ([]config.File, error) {
	files, err := m.Registries().ExtraFiles()

	return append(files, m.MachineFiles...), err
}

// Type implements the config.Provider interface.
func (m *MachineConfig) Type() machine.Type {
	switch m.MachineType {
	case "init":
		return machine.TypeInit
	case "controlplane":
		return machine.TypeControlPlane
	default:
		return machine.TypeJoin
	}
}

// Server implements the config.Provider interface.
func (m *MachineConfig) Server() string {
	return ""
}

// Sysctls implements the config.Provider interface.
func (m *MachineConfig) Sysctls() map[string]string {
	if m.MachineSysctls == nil {
		m.MachineSysctls = make(map[string]string)
	}

	return m.MachineSysctls
}

// CA implements the config.Provider interface.
func (m *MachineConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return m.MachineCA
}

// Token implements the config.Provider interface.
func (m *MachineConfig) Token() string {
	return m.MachineToken
}

// CertSANs implements the config.Provider interface.
func (m *MachineConfig) CertSANs() []string {
	return m.MachineCertSANs
}

// SetCertSANs implements the config.Provider interface.
func (m *MachineConfig) SetCertSANs(sans []string) {
	m.MachineCertSANs = append(m.MachineCertSANs, sans...)
}

// Registries implements the config.Provider interface.
func (m *MachineConfig) Registries() config.Registries {
	return &m.MachineRegistries
}

// Image implements the config.Provider interface.
func (k *KubeletConfig) Image() string {
	image := k.KubeletImage

	if image == "" {
		image = fmt.Sprintf("%s:v%s", constants.KubeletImage, constants.DefaultKubernetesVersion)
	}

	return image
}

// ExtraArgs implements the config.Provider interface.
func (k *KubeletConfig) ExtraArgs() map[string]string {
	if k == nil {
		k = &KubeletConfig{}
	}

	if k.KubeletExtraArgs == nil {
		k.KubeletExtraArgs = make(map[string]string)
	}

	return k.KubeletExtraArgs
}

// ExtraMounts implements the config.Provider interface.
func (k *KubeletConfig) ExtraMounts() []specs.Mount {
	return k.KubeletExtraMounts
}

// Name implements the config.Provider interface.
func (c *ClusterConfig) Name() string {
	return c.ClusterName
}

// Endpoint implements the config.Provider interface.
func (c *ClusterConfig) Endpoint() *url.URL {
	return c.ControlPlane.Endpoint.URL
}

// LocalAPIServerPort implements the config.Provider interface.
func (c *ClusterConfig) LocalAPIServerPort() int {
	if c.ControlPlane.LocalAPIServerPort == 0 {
		return 6443
	}

	return c.ControlPlane.LocalAPIServerPort
}

// CertSANs implements the config.Provider interface.
func (c *ClusterConfig) CertSANs() []string {
	return c.APIServerConfig.CertSANs
}

// SetCertSANs implements the config.Provider interface.
func (c *ClusterConfig) SetCertSANs(sans []string) {
	if c.APIServerConfig == nil {
		c.APIServerConfig = &APIServerConfig{}
	}

	c.APIServerConfig.CertSANs = append(c.APIServerConfig.CertSANs, sans...)
}

// CA implements the config.Provider interface.
func (c *ClusterConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return c.ClusterCA
}

// AESCBCEncryptionSecret implements the config.Provider interface.
func (c *ClusterConfig) AESCBCEncryptionSecret() string {
	return c.ClusterAESCBCEncryptionSecret
}

// Config implements the config.Provider interface.
func (c *ClusterConfig) Config(t machine.Type) (string, error) {
	return "", nil
}

// APIServer implements the config.Provider interface.
func (c *ClusterConfig) APIServer() config.APIServer {
	if c.APIServerConfig == nil {
		return &APIServerConfig{}
	}

	return c.APIServerConfig
}

// Image implements the config.Provider interface.
func (a *APIServerConfig) Image() string {
	image := a.ContainerImage

	if image == "" {
		image = fmt.Sprintf("%s:v%s", constants.KubernetesAPIServerImage, constants.DefaultKubernetesVersion)
	}

	return image
}

// ExtraArgs implements the config.Provider interface.
func (a *APIServerConfig) ExtraArgs() map[string]string {
	return a.ExtraArgsConfig
}

// ControllerManager implements the config.Provider interface.
func (c *ClusterConfig) ControllerManager() config.ControllerManager {
	if c.ControllerManagerConfig == nil {
		return &ControllerManagerConfig{}
	}

	return c.ControllerManagerConfig
}

// Image implements the config.Provider interface.
func (c *ControllerManagerConfig) Image() string {
	image := c.ContainerImage

	if image == "" {
		image = fmt.Sprintf("%s:v%s", constants.KubernetesControllerManagerImage, constants.DefaultKubernetesVersion)
	}

	return image
}

// ExtraArgs implements the config.Provider interface.
func (c *ControllerManagerConfig) ExtraArgs() map[string]string {
	return c.ExtraArgsConfig
}

// Proxy implements the config.Provider interface.
func (c *ClusterConfig) Proxy() config.Proxy {
	if c.ProxyConfig == nil {
		return &ProxyConfig{}
	}

	return c.ProxyConfig
}

// Image implements the config.Provider interface.
func (p *ProxyConfig) Image() string {
	image := p.ContainerImage

	if image == "" {
		image = fmt.Sprintf("%s:v%s", constants.KubeProxyImage, constants.DefaultKubernetesVersion)
	}

	return image
}

// Mode implements the Proxy interface.
func (p *ProxyConfig) Mode() string {
	if p.ModeConfig == "" {
		return "iptables"
	}

	return p.ModeConfig
}

// ExtraArgs implements the Proxy interface.
func (p *ProxyConfig) ExtraArgs() map[string]string {
	return p.ExtraArgsConfig
}

// Scheduler implements the config.Provider interface.
func (c *ClusterConfig) Scheduler() config.Scheduler {
	if c.SchedulerConfig == nil {
		return &SchedulerConfig{}
	}

	return c.SchedulerConfig
}

// AdminKubeconfig implements the config.Provider interface.
func (c *ClusterConfig) AdminKubeconfig() config.AdminKubeconfig {
	return c.AdminKubeconfigConfig
}

// Image implements the config.Provider interface.
func (s *SchedulerConfig) Image() string {
	image := s.ContainerImage

	if image == "" {
		image = fmt.Sprintf("%s:v%s", constants.KubernetesSchedulerImage, constants.DefaultKubernetesVersion)
	}

	return image
}

// ExtraArgs implements the config.Provider interface.
func (s *SchedulerConfig) ExtraArgs() map[string]string {
	return s.ExtraArgsConfig
}

// Etcd implements the config.Provider interface.
func (c *ClusterConfig) Etcd() config.Etcd {
	return c.EtcdConfig
}

// Image implements the config.Provider interface.
func (e *EtcdConfig) Image() string {
	image := e.ContainerImage
	suffix := ""

	if goruntime.GOARCH == "arm64" {
		suffix = "-arm64"
	}

	if image == "" {
		image = fmt.Sprintf("%s:%s%s", constants.EtcdImage, constants.DefaultEtcdVersion, suffix)
	}

	return image
}

// CA implements the config.Provider interface.
func (e *EtcdConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return e.RootCA
}

// ExtraArgs implements the config.Provider interface.
func (e *EtcdConfig) ExtraArgs() map[string]string {
	if e.EtcdExtraArgs == nil {
		e.EtcdExtraArgs = make(map[string]string)
	}

	return e.EtcdExtraArgs
}

// Mirrors implements the Registries interface.
func (r *RegistriesConfig) Mirrors() map[string]config.RegistryMirrorConfig {
	return r.RegistryMirrors
}

// Config implements the Registries interface.
func (r *RegistriesConfig) Config() map[string]config.RegistryConfig {
	return r.RegistryConfig
}

// ExtraFiles implements the Registries interface.
func (r *RegistriesConfig) ExtraFiles() ([]config.File, error) {
	return criplugin.GenerateRegistriesConfig(r)
}

// Token implements the config.Provider interface.
func (c *ClusterConfig) Token() config.Token {
	return c
}

// ID implements the config.Provider interface.
func (c *ClusterConfig) ID() string {
	parts := strings.Split(c.BootstrapToken, ".")
	if len(parts) != 2 {
		return ""
	}

	return parts[0]
}

// Secret implements the config.Provider interface.
func (c *ClusterConfig) Secret() string {
	parts := strings.Split(c.BootstrapToken, ".")
	if len(parts) != 2 {
		return ""
	}

	return parts[1]
}

// Network implements the config.Provider interface.
func (c *ClusterConfig) Network() config.ClusterNetwork {
	return c
}

// DNSDomain implements the config.Provider interface.
func (c *ClusterConfig) DNSDomain() string {
	if c.ClusterNetwork == nil {
		return constants.DefaultDNSDomain
	}

	return c.ClusterNetwork.DNSDomain
}

// CNI implements the config.Provider interface.
func (c *ClusterConfig) CNI() config.CNI {
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

// PodCIDR implements the config.Provider interface.
func (c *ClusterConfig) PodCIDR() string {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.PodSubnet) == 0:
		return constants.DefaultIPv4PodNet
	}

	return strings.Join(c.ClusterNetwork.PodSubnet, ",")
}

// ServiceCIDR implements the config.Provider interface.
func (c *ClusterConfig) ServiceCIDR() string {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.ServiceSubnet) == 0:
		return constants.DefaultIPv4ServiceNet
	}

	return strings.Join(c.ClusterNetwork.ServiceSubnet, ",")
}

// ExtraManifestURLs implements the config.Provider interface.
func (c *ClusterConfig) ExtraManifestURLs() []string {
	return c.ExtraManifests
}

// ExtraManifestHeaderMap implements the config.Provider interface.
func (c *ClusterConfig) ExtraManifestHeaderMap() map[string]string {
	return c.ExtraManifestHeaders
}

// PodCheckpointer implements the config.Provider interface.
func (c *ClusterConfig) PodCheckpointer() config.PodCheckpointer {
	if c.PodCheckpointerConfig == nil {
		return &PodCheckpointer{}
	}

	return c.PodCheckpointerConfig
}

// CoreDNS implements the config.Provider interface.
func (c *ClusterConfig) CoreDNS() config.CoreDNS {
	if c.CoreDNSConfig == nil {
		return &CoreDNS{}
	}

	return c.CoreDNSConfig
}

// Name implements the config.Provider interface.
func (c *CNIConfig) Name() string {
	return c.CNIName
}

// URLs implements the config.Provider interface.
func (c *CNIConfig) URLs() []string {
	return c.CNIUrls
}

// Hostname implements the config.Provider interface.
func (n *NetworkConfig) Hostname() string {
	return n.NetworkHostname
}

// SetHostname implements the config.Provider interface.
func (n *NetworkConfig) SetHostname(hostname string) {
	n.NetworkHostname = hostname
}

// Devices implements the config.Provider interface.
func (n *NetworkConfig) Devices() []config.Device {
	return n.NetworkInterfaces
}

// Resolvers implements the config.Provider interface.
func (n *NetworkConfig) Resolvers() []string {
	return n.NameServers
}

// ExtraHosts implements the config.Provider interface.
func (n *NetworkConfig) ExtraHosts() []config.ExtraHost {
	return n.ExtraHostEntries
}

// Servers implements the config.Provider interface.
func (t *TimeConfig) Servers() []string {
	return t.TimeServers
}

// Image implements the config.Provider interface.
func (i *InstallConfig) Image() string {
	return i.InstallImage
}

// Disk implements the config.Provider interface.
func (i *InstallConfig) Disk() string {
	return i.InstallDisk
}

// ExtraKernelArgs implements the config.Provider interface.
func (i *InstallConfig) ExtraKernelArgs() []string {
	return i.InstallExtraKernelArgs
}

// Zero implements the config.Provider interface.
func (i *InstallConfig) Zero() bool {
	return i.InstallWipe
}

// Force implements the config.Provider interface.
func (i *InstallConfig) Force() bool {
	return i.InstallForce
}

// WithBootloader implements the config.Provider interface.
func (i *InstallConfig) WithBootloader() bool {
	return i.InstallBootloader
}

// Image implements the config.Provider interface.
func (c *CoreDNS) Image() string {
	coreDNSImage := asset.DefaultImages.CoreDNS

	if c.CoreDNSImage != "" {
		coreDNSImage = c.CoreDNSImage
	}

	return coreDNSImage
}

// Image implements the config.Provider interface.
func (p *PodCheckpointer) Image() string {
	checkpointerImage := constants.PodCheckpointerImage

	if p.PodCheckpointerImage != "" {
		checkpointerImage = p.PodCheckpointerImage
	}

	return checkpointerImage
}

// CertLifetime implements the config.Provider interface.
func (a AdminKubeconfigConfig) CertLifetime() time.Duration {
	if a.AdminKubeconfigCertLifetime == 0 {
		return constants.KubernetesAdminCertDefaultLifetime
	}

	return a.AdminKubeconfigCertLifetime
}
