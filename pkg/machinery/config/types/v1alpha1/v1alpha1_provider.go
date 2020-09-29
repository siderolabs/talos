// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"crypto/tls"
	stdx509 "crypto/x509"
	"fmt"
	"net/url"
	"os"
	goruntime "runtime"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v3"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/bootkube-plugin/pkg/asset"

	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
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
	disks := make([]config.Disk, len(m.MachineDisks))

	for i := 0; i < len(m.MachineDisks); i++ {
		disks[i] = m.MachineDisks[i]
	}

	return disks
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
		return &TimeConfig{
			TimeEnabled: true,
		}
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
	files := make([]config.File, len(m.MachineFiles))

	for i := 0; i < len(m.MachineFiles); i++ {
		files[i] = m.MachineFiles[i]
	}

	return files, nil
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
		image = fmt.Sprintf("%s-%s:v%s", constants.KubernetesAPIServerImage, goruntime.GOARCH, constants.DefaultKubernetesVersion)
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
		image = fmt.Sprintf("%s-%s:v%s", constants.KubernetesControllerManagerImage, goruntime.GOARCH, constants.DefaultKubernetesVersion)
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
		image = fmt.Sprintf("%s-%s:v%s", constants.KubeProxyImage, goruntime.GOARCH, constants.DefaultKubernetesVersion)
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
		image = fmt.Sprintf("%s-%s:v%s", constants.KubernetesSchedulerImage, goruntime.GOARCH, constants.DefaultKubernetesVersion)
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
	mirrors := make(map[string]config.RegistryMirrorConfig, len(r.RegistryMirrors))

	for k, v := range r.RegistryMirrors {
		mirrors[k] = v
	}

	return mirrors
}

// Config implements the Registries interface.
func (r *RegistriesConfig) Config() map[string]config.RegistryConfig {
	registries := make(map[string]config.RegistryConfig, len(r.RegistryConfig))

	for k, v := range r.RegistryConfig {
		registries[k] = v
	}

	return registries
}

// TLS implements the Registries interface.
func (r *RegistryConfig) TLS() config.RegistryTLSConfig {
	if r == nil {
		return nil
	}

	return r.RegistryTLS
}

// Auth implements the Registries interface.
func (r *RegistryConfig) Auth() config.RegistryAuthConfig {
	if r == nil {
		return nil
	}

	return r.RegistryAuth
}

// Username implements the Registries interface.
func (r *RegistryAuthConfig) Username() string {
	return r.RegistryUsername
}

// Password implements the Registries interface.
func (r *RegistryAuthConfig) Password() string {
	return r.RegistryPassword
}

// Auth implements the Registries interface.
func (r *RegistryAuthConfig) Auth() string {
	return r.RegistryAuth
}

// IdentityToken implements the Registries interface.
func (r *RegistryAuthConfig) IdentityToken() string {
	return r.RegistryIdentityToken
}

// ClientIdentity implements the Registries interface.
func (r *RegistryTLSConfig) ClientIdentity() *x509.PEMEncodedCertificateAndKey {
	return r.TLSClientIdentity
}

// CA implements the Registries interface.
func (r *RegistryTLSConfig) CA() []byte {
	return r.TLSCA
}

// InsecureSkipVerify implements the Registries interface.
func (r *RegistryTLSConfig) InsecureSkipVerify() bool {
	return r.TLSInsecureSkipVerify
}

// GetTLSConfig prepares TLS configuration for connection.
func (r *RegistryTLSConfig) GetTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	if r.TLSClientIdentity != nil {
		cert, err := tls.X509KeyPair(r.TLSClientIdentity.Crt, r.TLSClientIdentity.Key)
		if err != nil {
			return nil, fmt.Errorf("error parsing client identity: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if r.CA() != nil {
		tlsConfig.RootCAs = stdx509.NewCertPool()
		tlsConfig.RootCAs.AppendCertsFromPEM(r.TLSCA)
	}

	if r.TLSInsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	return tlsConfig, nil
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
	interfaces := make([]config.Device, len(n.NetworkInterfaces))

	for i := 0; i < len(n.NetworkInterfaces); i++ {
		interfaces[i] = n.NetworkInterfaces[i]
	}

	return interfaces
}

// Resolvers implements the config.Provider interface.
func (n *NetworkConfig) Resolvers() []string {
	return n.NameServers
}

// ExtraHosts implements the config.Provider interface.
func (n *NetworkConfig) ExtraHosts() []config.ExtraHost {
	hosts := make([]config.ExtraHost, len(n.ExtraHostEntries))

	for i := 0; i < len(n.ExtraHostEntries); i++ {
		hosts[i] = n.ExtraHostEntries[i]
	}

	return hosts
}

// IP implements the MachineNetwork interface.
func (e *ExtraHost) IP() string {
	return e.HostIP
}

// Aliases implements the MachineNetwork interface.
func (e *ExtraHost) Aliases() []string {
	return e.HostAliases
}

// Interface implements the MachineNetwork interface.
func (d *Device) Interface() string {
	return d.DeviceInterface
}

// CIDR implements the MachineNetwork interface.
func (d *Device) CIDR() string {
	return d.DeviceCIDR
}

// Routes implements the MachineNetwork interface.
func (d *Device) Routes() []config.Route {
	routes := make([]config.Route, len(d.DeviceRoutes))

	for i := 0; i < len(d.DeviceRoutes); i++ {
		routes[i] = d.DeviceRoutes[i]
	}

	return routes
}

// Bond implements the MachineNetwork interface.
func (d *Device) Bond() config.Bond {
	if d.DeviceBond == nil {
		return nil
	}

	return d.DeviceBond
}

// Vlans implements the MachineNetwork interface.
func (d *Device) Vlans() []config.Vlan {
	vlans := make([]config.Vlan, len(d.DeviceVlans))

	for i := 0; i < len(d.DeviceVlans); i++ {
		vlans[i] = d.DeviceVlans[i]
	}

	return vlans
}

// MTU implements the MachineNetwork interface.
func (d *Device) MTU() int {
	return d.DeviceMTU
}

// DHCP implements the MachineNetwork interface.
func (d *Device) DHCP() bool {
	return d.DeviceDHCP
}

// Ignore implements the MachineNetwork interface.
func (d *Device) Ignore() bool {
	return d.DeviceIgnore
}

// Dummy implements the MachineNetwork interface.
func (d *Device) Dummy() bool {
	return d.DeviceDummy
}

// Network implements the MachineNetwork interface.
func (r *Route) Network() string {
	return r.RouteNetwork
}

// Gateway implements the MachineNetwork interface.
func (r *Route) Gateway() string {
	return r.RouteGateway
}

// Interfaces implements the MachineNetwork interface.
func (b *Bond) Interfaces() []string {
	if b == nil {
		return nil
	}

	return b.BondInterfaces
}

// ARPIPTarget implements the MachineNetwork interface.
func (b *Bond) ARPIPTarget() []string {
	if b == nil {
		return nil
	}

	return b.BondARPIPTarget
}

// Mode implements the MachineNetwork interface.
func (b *Bond) Mode() string {
	return b.BondMode
}

// HashPolicy implements the MachineNetwork interface.
func (b *Bond) HashPolicy() string {
	return b.BondHashPolicy
}

// LACPRate implements the MachineNetwork interface.
func (b *Bond) LACPRate() string {
	return b.BondLACPRate
}

// ADActorSystem implements the MachineNetwork interface.
func (b *Bond) ADActorSystem() string {
	return b.BondADActorSystem
}

// ARPValidate implements the MachineNetwork interface.
func (b *Bond) ARPValidate() string {
	return b.BondARPValidate
}

// ARPAllTargets implements the MachineNetwork interface.
func (b *Bond) ARPAllTargets() string {
	return b.BondARPAllTargets
}

// Primary implements the MachineNetwork interface.
func (b *Bond) Primary() string {
	return b.BondPrimary
}

// PrimaryReselect implements the MachineNetwork interface.
func (b *Bond) PrimaryReselect() string {
	return b.BondPrimaryReselect
}

// FailOverMac implements the MachineNetwork interface.
func (b *Bond) FailOverMac() string {
	return b.BondFailOverMac
}

// ADSelect implements the MachineNetwork interface.
func (b *Bond) ADSelect() string {
	return b.BondADSelect
}

// MIIMon implements the MachineNetwork interface.
func (b *Bond) MIIMon() uint32 {
	return b.BondMIIMon
}

// UpDelay implements the MachineNetwork interface.
func (b *Bond) UpDelay() uint32 {
	return b.BondUpDelay
}

// DownDelay implements the MachineNetwork interface.
func (b *Bond) DownDelay() uint32 {
	return b.BondDownDelay
}

// ARPInterval implements the MachineNetwork interface.
func (b *Bond) ARPInterval() uint32 {
	return b.BondARPInterval
}

// ResendIGMP implements the MachineNetwork interface.
func (b *Bond) ResendIGMP() uint32 {
	return b.BondResendIGMP
}

// MinLinks implements the MachineNetwork interface.
func (b *Bond) MinLinks() uint32 {
	return b.BondMinLinks
}

// LPInterval implements the MachineNetwork interface.
func (b *Bond) LPInterval() uint32 {
	return b.BondLPInterval
}

// PacketsPerSlave implements the MachineNetwork interface.
func (b *Bond) PacketsPerSlave() uint32 {
	return b.BondPacketsPerSlave
}

// NumPeerNotif implements the MachineNetwork interface.
func (b *Bond) NumPeerNotif() uint8 {
	return b.BondNumPeerNotif
}

// TLBDynamicLB implements the MachineNetwork interface.
func (b *Bond) TLBDynamicLB() uint8 {
	return b.BondTLBDynamicLB
}

// AllSlavesActive implements the MachineNetwork interface.
func (b *Bond) AllSlavesActive() uint8 {
	return b.BondAllSlavesActive
}

// UseCarrier implements the MachineNetwork interface.
func (b *Bond) UseCarrier() bool {
	return b.BondUseCarrier
}

// ADActorSysPrio implements the MachineNetwork interface.
func (b *Bond) ADActorSysPrio() uint16 {
	return b.BondADActorSysPrio
}

// ADUserPortKey implements the MachineNetwork interface.
func (b *Bond) ADUserPortKey() uint16 {
	return b.BondADUserPortKey
}

// PeerNotifyDelay implements the MachineNetwork interface.
func (b *Bond) PeerNotifyDelay() uint32 {
	return b.BondPeerNotifyDelay
}

// CIDR implements the MachineNetwork interface.
func (v *Vlan) CIDR() string {
	return v.VlanCIDR
}

// Routes implements the MachineNetwork interface.
func (v *Vlan) Routes() []config.Route {
	routes := make([]config.Route, len(v.VlanRoutes))

	for i := 0; i < len(v.VlanRoutes); i++ {
		routes[i] = v.VlanRoutes[i]
	}

	return routes
}

// DHCP implements the MachineNetwork interface.
func (v *Vlan) DHCP() bool {
	return v.VlanDHCP
}

// ID implements the MachineNetwork interface.
func (v *Vlan) ID() uint16 {
	return v.VlanID
}

// Enabled implements the config.Provider interface.
func (t *TimeConfig) Enabled() bool {
	return t.TimeEnabled
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

// Endpoints implements the config.Provider interface.
func (r *RegistryMirrorConfig) Endpoints() []string {
	return r.MirrorEndpoints
}

// Content implements the config.Provider interface.
func (f *MachineFile) Content() string {
	return f.FileContent
}

// Permissions implements the config.Provider interface.
func (f *MachineFile) Permissions() os.FileMode {
	return f.FilePermissions
}

// Path implements the config.Provider interface.
func (f *MachineFile) Path() string {
	return f.FilePath
}

// Op implements the config.Provider interface.
func (f *MachineFile) Op() string {
	return f.FileOp
}

// Device implements the config.Provider interface.
func (d *MachineDisk) Device() string {
	return d.DeviceName
}

// Partitions implements the config.Provider interface.
func (d *MachineDisk) Partitions() []config.Partition {
	partitions := make([]config.Partition, len(d.DiskPartitions))

	for i := 0; i < len(d.DiskPartitions); i++ {
		partitions[i] = d.DiskPartitions[i]
	}

	return partitions
}

// Size implements the config.Provider interface.
func (p *DiskPartition) Size() uint {
	return p.DiskSize
}

// MountPoint implements the config.Provider interface.
func (p *DiskPartition) MountPoint() string {
	return p.DiskMountPoint
}
