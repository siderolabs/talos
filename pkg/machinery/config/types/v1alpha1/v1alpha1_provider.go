// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"crypto/tls"
	stdx509 "crypto/x509"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/blockdevice/encryption"
	"github.com/siderolabs/go-blockdevice/blockdevice/util/disk"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Verify interfaces.
var (
	_ config.Document       = (*Config)(nil)
	_ config.SecretDocument = (*Config)(nil)
	_ config.Validator      = (*Config)(nil)
)

const (
	// Version is the version string for v1alpha1.
	Version = "v1alpha1"
)

// Clone implements config.Document interface.
func (c *Config) Clone() config.Document {
	return c.DeepCopy()
}

// Kind returns the kind of the document.
func (c *Config) Kind() string {
	return Version // legacy document
}

// APIVersion returns the API version of the document.
func (c *Config) APIVersion() string {
	return "" // legacy document
}

// Debug implements the config.Provider interface.
func (c *Config) Debug() bool {
	if c == nil {
		return false
	}

	return pointer.SafeDeref(c.ConfigDebug)
}

// Machine implements the config.Provider interface.
func (c *Config) Machine() config.MachineConfig {
	if c == nil || c.MachineConfig == nil {
		return &MachineConfig{}
	}

	return c.MachineConfig
}

// SeccompProfiles implements the config.Provider interface.
func (m *MachineConfig) SeccompProfiles() []config.SeccompProfile {
	return xslices.Map(m.MachineSeccompProfiles, func(m *MachineSeccompProfile) config.SeccompProfile { return m })
}

// Name implements the config.Provider interface.
func (m *MachineSeccompProfile) Name() string {
	return m.MachineSeccompProfileName
}

// Value implements the config.Provider interface.
func (m *MachineSeccompProfile) Value() map[string]interface{} {
	return m.MachineSeccompProfileValue.Object
}

// NodeLabels implements the config.Provider interface.
func (m *MachineConfig) NodeLabels() config.NodeLabels {
	return m.MachineNodeLabels
}

// NodeTaints implements the config.Provider interface.
func (m *MachineConfig) NodeTaints() config.NodeTaints {
	return m.MachineNodeTaints
}

// Cluster implements the config.Provider interface.
func (c *Config) Cluster() config.ClusterConfig {
	if c == nil || c.ClusterConfig == nil {
		return &ClusterConfig{}
	}

	return c.ClusterConfig
}

// Redact implements the config.SecretDocument interface.
//
//nolint:gocyclo
func (c *Config) Redact(replacement string) {
	if c == nil {
		return
	}

	redactBytes := func(b []byte) []byte {
		if len(b) == 0 {
			return b
		}

		return []byte(replacement)
	}

	redactStr := func(s string) string {
		return string(redactBytes([]byte(s)))
	}

	if c.MachineConfig != nil {
		c.MachineConfig.MachineToken = redactStr(c.MachineConfig.MachineToken)
		if c.MachineConfig.MachineCA != nil {
			c.MachineConfig.MachineCA.Key = redactBytes(c.MachineConfig.MachineCA.Key)
		}
	}

	if c.ClusterConfig != nil {
		c.ClusterConfig.ClusterSecret = redactStr(c.ClusterConfig.ClusterSecret)
		c.ClusterConfig.BootstrapToken = redactStr(c.ClusterConfig.BootstrapToken)
		c.ClusterConfig.ClusterAESCBCEncryptionSecret = redactStr(c.ClusterConfig.ClusterAESCBCEncryptionSecret)
		c.ClusterConfig.ClusterSecretboxEncryptionSecret = redactStr(c.ClusterConfig.ClusterSecretboxEncryptionSecret)

		if c.ClusterConfig.ClusterServiceAccount != nil {
			c.ClusterConfig.ClusterServiceAccount.Key = redactBytes(c.ClusterConfig.ClusterServiceAccount.Key)
		}

		if c.ClusterConfig.ClusterCA != nil {
			c.ClusterConfig.ClusterCA.Key = redactBytes(c.ClusterConfig.ClusterCA.Key)
		}

		if c.ClusterConfig.ClusterAggregatorCA != nil {
			c.ClusterConfig.ClusterAggregatorCA.Key = redactBytes(c.ClusterConfig.ClusterAggregatorCA.Key)
		}

		if c.ClusterConfig.EtcdConfig != nil && c.ClusterConfig.EtcdConfig.RootCA != nil {
			c.ClusterConfig.EtcdConfig.RootCA.Key = redactBytes(c.ClusterConfig.EtcdConfig.RootCA.Key)
		}
	}
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
	return xslices.Map(m.MachineDisks, func(d *MachineDisk) config.Disk { return d })
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

// Controlplane implements the config.Provider interface.
func (m *MachineConfig) Controlplane() config.MachineControlPlane {
	if m.MachineControlPlane == nil {
		return &MachineControlPlaneConfig{}
	}

	return m.MachineControlPlane
}

// Pods implements the config.Provider interface.
func (m *MachineConfig) Pods() []map[string]interface{} {
	return xslices.Map(m.MachinePods, func(u Unstructured) map[string]interface{} { return u.Object })
}

// ControllerManager implements the config.Provider interface.
func (m *MachineControlPlaneConfig) ControllerManager() config.MachineControllerManager {
	if m.MachineControllerManager == nil {
		return &MachineControllerManagerConfig{}
	}

	return m.MachineControllerManager
}

// Scheduler implements the config.Provider interface.
func (m *MachineControlPlaneConfig) Scheduler() config.MachineScheduler {
	if m.MachineScheduler == nil {
		return &MachineSchedulerConfig{}
	}

	return m.MachineScheduler
}

// Disabled implements the config.Provider interface.
func (m *MachineControllerManagerConfig) Disabled() bool {
	return pointer.SafeDeref(m.MachineControllerManagerDisabled)
}

// Disabled implements the config.Provider interface.
func (m *MachineSchedulerConfig) Disabled() bool {
	return pointer.SafeDeref(m.MachineSchedulerDisabled)
}

// Kubelet implements the config.Provider interface.
func (m *MachineConfig) Kubelet() config.Kubelet {
	if m.MachineKubelet == nil {
		return &KubeletConfig{}
	}

	return m.MachineKubelet
}

// Env implements the config.Provider interface.
func (m *MachineConfig) Env() config.Env {
	return m.MachineEnv
}

// Files implements the config.Provider interface.
func (m *MachineConfig) Files() ([]config.File, error) {
	return xslices.Map(m.MachineFiles, func(f *MachineFile) config.File { return f }), nil
}

// Type implements the config.Provider interface.
func (m *MachineConfig) Type() machine.Type {
	t, _ := machine.ParseType(m.MachineType) //nolint:errcheck

	return t
}

// Server implements the config.Provider interface.
func (m *MachineConfig) Server() string {
	return ""
}

// Sysctls implements the config.Provider interface.
func (m *MachineConfig) Sysctls() map[string]string {
	if m.MachineSysctls == nil {
		return make(map[string]string)
	}

	return m.MachineSysctls
}

// Sysfs implements the config.Provider interface.
func (m *MachineConfig) Sysfs() map[string]string {
	if m.MachineSysfs == nil {
		return make(map[string]string)
	}

	return m.MachineSysfs
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

// Registries implements the config.Provider interface.
func (m *MachineConfig) Registries() config.Registries {
	return &m.MachineRegistries
}

// SystemDiskEncryption implements the config.Provider interface.
func (m *MachineConfig) SystemDiskEncryption() config.SystemDiskEncryption {
	if m.MachineSystemDiskEncryption == nil {
		return &SystemDiskEncryptionConfig{}
	}

	return m.MachineSystemDiskEncryption
}

// Features implements the config.MachineConfig interface.
func (m *MachineConfig) Features() config.Features {
	if m.MachineFeatures == nil {
		return &FeaturesConfig{}
	}

	return m.MachineFeatures
}

// Udev implements the config.MachineConfig interface.
func (m *MachineConfig) Udev() config.UdevConfig {
	if m.MachineUdev == nil {
		return &UdevConfig{}
	}

	return m.MachineUdev
}

// Logging implements the config.MachineConfig interface.
func (m *MachineConfig) Logging() config.Logging {
	if m.MachineLogging == nil {
		return &LoggingConfig{}
	}

	return m.MachineLogging
}

// Kernel implements the config.MachineConfig interface.
func (m *MachineConfig) Kernel() config.Kernel {
	if m.MachineKernel == nil {
		return &KernelConfig{}
	}

	return m.MachineKernel
}

// Image implements the config.Provider interface.
func (k *KubeletConfig) Image() string {
	image := k.KubeletImage

	if image == "" {
		image = fmt.Sprintf("%s:v%s", constants.KubeletImage, constants.DefaultKubernetesVersion)
	}

	return image
}

// ClusterDNS implements the config.Provider interface.
func (k *KubeletConfig) ClusterDNS() []string {
	if k == nil || k.KubeletClusterDNS == nil {
		return nil
	}

	return k.KubeletClusterDNS
}

// ExtraArgs implements the config.Provider interface.
func (k *KubeletConfig) ExtraArgs() map[string]string {
	if k == nil || k.KubeletExtraArgs == nil {
		return make(map[string]string)
	}

	return k.KubeletExtraArgs
}

// ExtraMounts implements the config.Provider interface.
func (k *KubeletConfig) ExtraMounts() []specs.Mount {
	// use the intermediate type which is assignable to specs.Mount so that
	// we can be sure that `specs.Mount` and `Mount` have exactly same fields.
	//
	// as in Go []T1 is not assignable to []T2, even if T1 and T2 are assignable, we cannot
	// use direct conversion of Mount and specs.Mount
	type mountConverter struct {
		Destination string
		Type        string
		Source      string
		Options     []string
		UIDMappings []specs.LinuxIDMapping
		GIDMappings []specs.LinuxIDMapping
	}

	return xslices.Map(k.KubeletExtraMounts,
		func(m ExtraMount) specs.Mount {
			return specs.Mount(func() mountConverter {
				return mountConverter{
					Destination: m.Destination,
					Type:        m.Type,
					Source:      m.Source,
					Options:     m.Options,
					UIDMappings: xslices.Map(m.UIDMappings, func(m LinuxIDMapping) specs.LinuxIDMapping { return specs.LinuxIDMapping(m) }),
					GIDMappings: xslices.Map(m.GIDMappings, func(m LinuxIDMapping) specs.LinuxIDMapping { return specs.LinuxIDMapping(m) }),
				}
			}())
		})
}

// ExtraConfig implements the config.Provider interface.
func (k *KubeletConfig) ExtraConfig() map[string]interface{} {
	return k.KubeletExtraConfig.Object
}

// CredentialProviderConfig implements the config.Provider interface.
func (k *KubeletConfig) CredentialProviderConfig() map[string]interface{} {
	return k.KubeletCredentialProviderConfig.Object
}

// DefaultRuntimeSeccompProfileEnabled implements the config.Provider interface.
func (k *KubeletConfig) DefaultRuntimeSeccompProfileEnabled() bool {
	return pointer.SafeDeref(k.KubeletDefaultRuntimeSeccompProfileEnabled)
}

// RegisterWithFQDN implements the config.Provider interface.
func (k *KubeletConfig) RegisterWithFQDN() bool {
	return pointer.SafeDeref(k.KubeletRegisterWithFQDN)
}

// NodeIP implements the config.Provider interface.
func (k *KubeletConfig) NodeIP() config.KubeletNodeIP {
	if k.KubeletNodeIP == nil {
		return &KubeletNodeIPConfig{}
	}

	return k.KubeletNodeIP
}

// SkipNodeRegistration implements the config.Provider interface.
func (k *KubeletConfig) SkipNodeRegistration() bool {
	return pointer.SafeDeref(k.KubeletSkipNodeRegistration)
}

// DisableManifestsDirectory implements the KubeletConfig interface.
func (k *KubeletConfig) DisableManifestsDirectory() bool {
	return pointer.SafeDeref(k.KubeletDisableManifestsDirectory)
}

// ValidSubnets implements the config.Provider interface.
func (k *KubeletNodeIPConfig) ValidSubnets() []string {
	return k.KubeletNodeIPValidSubnets
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
	if r.RegistryTLS == nil {
		return nil
	}

	return r.RegistryTLS
}

// Auth implements the Registries interface.
func (r *RegistryConfig) Auth() config.RegistryAuthConfig {
	if r.RegistryAuth == nil {
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
	return pointer.SafeDeref(r.TLSInsecureSkipVerify)
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

	if r.InsecureSkipVerify() {
		tlsConfig.InsecureSkipVerify = true
	}

	return tlsConfig, nil
}

// Hostname implements the config.Provider interface.
func (n *NetworkConfig) Hostname() string {
	return n.NetworkHostname
}

// DisableSearchDomain implements the config.Provider interface.
func (n *NetworkConfig) DisableSearchDomain() bool {
	return pointer.SafeDeref(n.NetworkDisableSearchDomain)
}

// Devices implements the config.Provider interface.
func (n *NetworkConfig) Devices() []config.Device {
	return xslices.Map(n.NetworkInterfaces, func(d *Device) config.Device { return d })
}

// getDevice adds or returns existing Device by name.
//
// This method mutates configuration, but it's only used in config generation.
func (n *NetworkConfig) getDevice(iface IfaceSelector) *Device {
	for _, dev := range n.NetworkInterfaces {
		if iface.matches(dev) {
			return dev
		}
	}

	dev := iface.new()

	n.NetworkInterfaces = append(n.NetworkInterfaces, dev)

	return dev
}

// Resolvers implements the config.Provider interface.
func (n *NetworkConfig) Resolvers() []string {
	return n.NameServers
}

// ExtraHosts implements the config.Provider interface.
func (n *NetworkConfig) ExtraHosts() []config.ExtraHost {
	return xslices.Map(n.ExtraHostEntries, func(e *ExtraHost) config.ExtraHost { return e })
}

// KubeSpan implements the config.Provider interface.
func (n *NetworkConfig) KubeSpan() config.KubeSpan {
	if n.NetworkKubeSpan == nil {
		return &NetworkKubeSpan{}
	}

	return n.NetworkKubeSpan
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

// Addresses implements the MachineNetwork interface.
func (d *Device) Addresses() []string {
	switch {
	case len(d.DeviceAddresses) > 0:
		return slices.Clone(d.DeviceAddresses)
	case d.DeviceCIDR != "":
		return []string{d.DeviceCIDR}
	default:
		return nil
	}
}

// Routes implements the MachineNetwork interface.
func (d *Device) Routes() []config.Route {
	return xslices.Map(d.DeviceRoutes, func(r *Route) config.Route { return r })
}

// Bond implements the MachineNetwork interface.
func (d *Device) Bond() config.Bond {
	if d.DeviceBond == nil {
		return nil
	}

	return d.DeviceBond
}

// Bridge implements the MachineNetwork interface.
func (d *Device) Bridge() config.Bridge {
	if d.DeviceBridge == nil {
		return nil
	}

	return d.DeviceBridge
}

// Vlans implements the MachineNetwork interface.
func (d *Device) Vlans() []config.Vlan {
	return xslices.Map(d.DeviceVlans, func(v *Vlan) config.Vlan { return v })
}

// MTU implements the MachineNetwork interface.
func (d *Device) MTU() int {
	return d.DeviceMTU
}

// DHCP implements the MachineNetwork interface.
func (d *Device) DHCP() bool {
	return pointer.SafeDeref(d.DeviceDHCP)
}

// Ignore implements the MachineNetwork interface.
func (d *Device) Ignore() bool {
	return pointer.SafeDeref(d.DeviceIgnore)
}

// Dummy implements the MachineNetwork interface.
func (d *Device) Dummy() bool {
	return pointer.SafeDeref(d.DeviceDummy)
}

// DHCPOptions implements the MachineNetwork interface.
func (d *Device) DHCPOptions() config.DHCPOptions {
	// Default route metric on systemd is 1024. This sets the same.
	if d.DeviceDHCPOptions == nil {
		return &DHCPOptions{
			DHCPRouteMetric: uint32(0),
		}
	}

	return d.DeviceDHCPOptions
}

// VIPConfig implements the MachineNetwork interface.
func (d *Device) VIPConfig() config.VIPConfig {
	if d.DeviceVIPConfig == nil {
		return nil
	}

	return d.DeviceVIPConfig
}

// Selector implements the config.Device interface.
func (d *Device) Selector() config.NetworkDeviceSelector {
	if d.DeviceSelector == nil {
		return nil
	}

	return d.DeviceSelector
}

// IP implements the config.VIPConfig interface.
func (d *DeviceVIPConfig) IP() string {
	return d.SharedIP
}

// EquinixMetal implements the config.VIPConfig interface.
func (d *DeviceVIPConfig) EquinixMetal() config.VIPEquinixMetal {
	if d.EquinixMetalConfig == nil {
		return nil
	}

	return d.EquinixMetalConfig
}

// APIToken implements the config.VIPEquinixMetal interface.
func (v *VIPEquinixMetalConfig) APIToken() string {
	return v.EquinixMetalAPIToken
}

// HCloud implements the config.VIPConfig interface.
func (d *DeviceVIPConfig) HCloud() config.VIPHCloud {
	if d.HCloudConfig == nil {
		return nil
	}

	return d.HCloudConfig
}

// APIToken implements the config.VIPHCloud interface.
func (v *VIPHCloudConfig) APIToken() string {
	return v.HCloudAPIToken
}

// WireguardConfig implements the MachineNetwork interface.
func (d *Device) WireguardConfig() config.WireguardConfig {
	if d.DeviceWireguardConfig == nil {
		return nil
	}

	return d.DeviceWireguardConfig
}

// RouteMetric implements the DHCPOptions interface.
func (d *DHCPOptions) RouteMetric() uint32 {
	return d.DHCPRouteMetric
}

// IPv4 implements the DHCPOptions interface.
func (d *DHCPOptions) IPv4() bool {
	if d.DHCPIPv4 == nil {
		return true
	}

	return *d.DHCPIPv4
}

// IPv6 implements the DHCPOptions interface.
func (d *DHCPOptions) IPv6() bool {
	if d.DHCPIPv6 == nil {
		return false
	}

	return *d.DHCPIPv6
}

// DUIDv6 implements the DHCPOptions interface.
func (d *DHCPOptions) DUIDv6() string {
	return d.DHCPDUIDv6
}

// PrivateKey implements the MachineNetwork interface.
func (wc *DeviceWireguardConfig) PrivateKey() string {
	return wc.WireguardPrivateKey
}

// ListenPort implements the MachineNetwork interface.
func (wc *DeviceWireguardConfig) ListenPort() int {
	return wc.WireguardListenPort
}

// FirewallMark implements the MachineNetwork interface.
func (wc *DeviceWireguardConfig) FirewallMark() int {
	return wc.WireguardFirewallMark
}

// Peers implements the MachineNetwork interface.
func (wc *DeviceWireguardConfig) Peers() []config.WireguardPeer {
	return xslices.Map(wc.WireguardPeers, func(p *DeviceWireguardPeer) config.WireguardPeer { return p })
}

// PublicKey implements the MachineNetwork interface.
func (wd *DeviceWireguardPeer) PublicKey() string {
	return wd.WireguardPublicKey
}

// Endpoint implements the MachineNetwork interface.
func (wd *DeviceWireguardPeer) Endpoint() string {
	return wd.WireguardEndpoint
}

// PersistentKeepaliveInterval implements the MachineNetwork interface.
func (wd *DeviceWireguardPeer) PersistentKeepaliveInterval() time.Duration {
	return wd.WireguardPersistentKeepaliveInterval
}

// AllowedIPs implements the MachineNetwork interface.
func (wd *DeviceWireguardPeer) AllowedIPs() []string {
	return wd.WireguardAllowedIPs
}

// Bus implements config.NetworkDeviceSelector interface.
func (s *NetworkDeviceSelector) Bus() string {
	return s.NetworkDeviceBus
}

// HardwareAddress implements config.NetworkDeviceSelector interface.
func (s *NetworkDeviceSelector) HardwareAddress() string {
	return s.NetworkDeviceHardwareAddress
}

// PCIID implements config.NetworkDeviceSelector interface.
func (s *NetworkDeviceSelector) PCIID() string {
	return s.NetworkDevicePCIID
}

// KernelDriver implements config.NetworkDeviceSelector interface.
func (s *NetworkDeviceSelector) KernelDriver() string {
	return s.NetworkDeviceKernelDriver
}

// Physical implements config.NetworkDeviceSelector interface.
func (s *NetworkDeviceSelector) Physical() *bool {
	return s.NetworkDevicePhysical
}

// Network implements the MachineNetwork interface.
func (r *Route) Network() string {
	return r.RouteNetwork
}

// Gateway implements the MachineNetwork interface.
func (r *Route) Gateway() string {
	return r.RouteGateway
}

// Source implements the MachineNetwork interface.
func (r *Route) Source() string {
	return r.RouteSource
}

// Metric implements the MachineNetwork interface.
func (r *Route) Metric() uint32 {
	return r.RouteMetric
}

// MTU implements the MachineNetwork interface.
func (r *Route) MTU() uint32 {
	return r.RouteMTU
}

// Interfaces implements the MachineNetwork interface.
func (b *Bond) Interfaces() []string {
	if b == nil {
		return nil
	}

	return b.BondInterfaces
}

// Selectors implements the Bond interface.
func (b *Bond) Selectors() []config.NetworkDeviceSelector {
	if b == nil || b.BondDeviceSelectors == nil {
		return nil
	}

	return xslices.Map(b.BondDeviceSelectors, func(d NetworkDeviceSelector) config.NetworkDeviceSelector { return &d })
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
	if b.BondUseCarrier == nil {
		return true
	}

	return *b.BondUseCarrier
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

// Enabled implements the config.STP interface.
func (s *STP) Enabled() bool {
	if s == nil || s.STPEnabled == nil {
		return true
	}

	return *s.STPEnabled
}

// Interfaces implements the config.Bridge interface.
func (b *Bridge) Interfaces() []string {
	return b.BridgedInterfaces
}

// STP implements the config.Bridge interface.
func (b *Bridge) STP() config.STP {
	if b.BridgeSTP == nil {
		return nil
	}

	return b.BridgeSTP
}

// Addresses implements the MachineNetwork interface.
func (v *Vlan) Addresses() []string {
	switch {
	case len(v.VlanAddresses) > 0:
		return slices.Clone(v.VlanAddresses)
	case v.VlanCIDR != "":
		return []string{v.VlanCIDR}
	default:
		return nil
	}
}

// MTU implements the MachineNetwork interface.
func (v *Vlan) MTU() uint32 {
	return v.VlanMTU
}

// VIPConfig implements the MachineNetwork interface.
func (v *Vlan) VIPConfig() config.VIPConfig {
	if v.VlanVIP == nil {
		return nil
	}

	return v.VlanVIP
}

// Routes implements the MachineNetwork interface.
func (v *Vlan) Routes() []config.Route {
	return xslices.Map(v.VlanRoutes, func(r *Route) config.Route { return r })
}

// DHCP implements the MachineNetwork interface.
func (v *Vlan) DHCP() bool {
	return pointer.SafeDeref(v.VlanDHCP)
}

// DHCPOptions implements the MachineNetwork interface.
func (v *Vlan) DHCPOptions() config.DHCPOptions {
	// Default route metric on systemd is 1024. This sets the same.
	if v.VlanDHCPOptions == nil {
		return &DHCPOptions{
			DHCPRouteMetric: uint32(0),
		}
	}

	return v.VlanDHCPOptions
}

// ID implements the MachineNetwork interface.
func (v *Vlan) ID() uint16 {
	return v.VlanID
}

// Enabled implements KubeSpan interface.
func (k *NetworkKubeSpan) Enabled() bool {
	return pointer.SafeDeref(k.KubeSpanEnabled)
}

// ForceRouting implements KubeSpan interface.
func (k *NetworkKubeSpan) ForceRouting() bool {
	return !pointer.SafeDeref(k.KubeSpanAllowDownPeerBypass)
}

// AdvertiseKubernetesNetworks implements KubeSpan interface.
func (k *NetworkKubeSpan) AdvertiseKubernetesNetworks() bool {
	return pointer.SafeDeref(k.KubeSpanAdvertiseKubernetesNetworks)
}

// HarvestExtraEndpoints implements KubeSpan interface.
func (k *NetworkKubeSpan) HarvestExtraEndpoints() bool {
	if k.KubeSpanHarvestExtraEndpoints == nil {
		return true
	}

	return pointer.SafeDeref(k.KubeSpanHarvestExtraEndpoints)
}

// MTU implements the KubeSpan interface.
func (k *NetworkKubeSpan) MTU() uint32 {
	mtu := pointer.SafeDeref(k.KubeSpanMTU)
	if mtu == 0 {
		mtu = constants.KubeSpanLinkMTU
	}

	return mtu
}

// Filters implements the KubeSpan interface.
func (k *NetworkKubeSpan) Filters() config.KubeSpanFilters {
	if k.KubeSpanFilters == nil {
		return &KubeSpanFilters{}
	}

	return k.KubeSpanFilters
}

// Endpoints implements the config.KubeSpanFilters interface.
func (k *KubeSpanFilters) Endpoints() []string {
	return k.KubeSpanFiltersEndpoints
}

// Disabled implements the config.Provider interface.
func (t *TimeConfig) Disabled() bool {
	return pointer.SafeDeref(t.TimeDisabled)
}

// Servers implements the config.Provider interface.
func (t *TimeConfig) Servers() []string {
	return t.TimeServers
}

// BootTimeout implements the config.Provider interface.
func (t *TimeConfig) BootTimeout() time.Duration {
	return t.TimeBootTimeout
}

// Image implements the config.Provider interface.
func (i *InstallConfig) Image() string {
	return i.InstallImage
}

// Extensions implements the config.Provider interface.
func (i *InstallConfig) Extensions() []config.Extension {
	return xslices.Map(i.InstallExtensions, func(e InstallExtensionConfig) config.Extension { return e })
}

// Disk implements the config.Provider interface.
func (i *InstallConfig) Disk() (string, error) {
	matchers := i.DiskMatchers()
	if len(matchers) > 0 {
		d, err := disk.Find(matchers...)
		if err != nil {
			return "", err
		}

		if d != nil {
			return d.DeviceName, nil
		}

		return "", fmt.Errorf("no disk found matching provided parameters")
	}

	return i.InstallDisk, nil
}

// DiskMatchers implements the config.Provider interface.
//
//nolint:gocyclo
func (i *InstallConfig) DiskMatchers() []disk.Matcher {
	if i.InstallDiskSelector != nil {
		selector := i.InstallDiskSelector

		matchers := []disk.Matcher{}
		if selector.Size != nil {
			matchers = append(matchers, selector.Size.Matcher)
		}

		if selector.UUID != "" {
			matchers = append(matchers, disk.WithUUID(selector.UUID))
		}

		if selector.WWID != "" {
			matchers = append(matchers, disk.WithWWID(selector.WWID))
		}

		if selector.Model != "" {
			matchers = append(matchers, disk.WithModel(selector.Model))
		}

		if selector.Name != "" {
			matchers = append(matchers, disk.WithName(selector.Name))
		}

		if selector.Serial != "" {
			matchers = append(matchers, disk.WithSerial(selector.Serial))
		}

		if selector.Modalias != "" {
			matchers = append(matchers, disk.WithModalias(selector.Modalias))
		}

		if disk.Type(selector.Type) != disk.TypeUnknown {
			matchers = append(matchers, disk.WithType(disk.Type(selector.Type)))
		}

		if selector.BusPath != "" {
			matchers = append(matchers, disk.WithBusPath(selector.BusPath))
		}

		return matchers
	}

	return nil
}

// ExtraKernelArgs implements the config.Provider interface.
func (i *InstallConfig) ExtraKernelArgs() []string {
	return i.InstallExtraKernelArgs
}

// Zero implements the config.Provider interface.
func (i *InstallConfig) Zero() bool {
	return pointer.SafeDeref(i.InstallWipe)
}

// LegacyBIOSSupport implements the config.Provider interface.
func (i *InstallConfig) LegacyBIOSSupport() bool {
	return pointer.SafeDeref(i.InstallLegacyBIOSSupport)
}

// WithBootloader implements the config.Provider interface.
func (i *InstallConfig) WithBootloader() bool {
	if i.InstallBootloader == nil {
		return true
	}

	return *i.InstallBootloader
}

// Image implements the config.Provider interface.
func (i InstallExtensionConfig) Image() string {
	return i.ExtensionImage
}

// Enabled implements the config.Provider interface.
func (c *CoreDNS) Enabled() bool {
	return c.CoreDNSDisabled == nil || !*c.CoreDNSDisabled
}

// Image implements the config.Provider interface.
func (c *CoreDNS) Image() string {
	coreDNSImage := fmt.Sprintf("%s:%s", constants.CoreDNSImage, constants.DefaultCoreDNSVersion)

	if c.CoreDNSImage != "" {
		coreDNSImage = c.CoreDNSImage
	}

	return coreDNSImage
}

// CertLifetime implements the config.Provider interface.
func (a *AdminKubeconfigConfig) CertLifetime() time.Duration {
	if a.AdminKubeconfigCertLifetime == 0 {
		return constants.KubernetesAdminCertDefaultLifetime
	}

	return a.AdminKubeconfigCertLifetime
}

// CommonName implements the config.Provider interface.
func (a *AdminKubeconfigConfig) CommonName() string {
	return constants.KubernetesAdminCertCommonName
}

// CertOrganization implements the config.Provider interface.
func (a *AdminKubeconfigConfig) CertOrganization() string {
	return constants.KubernetesAdminCertOrganization
}

// Endpoints implements the config.Provider interface.
func (r *RegistryMirrorConfig) Endpoints() []string {
	return r.MirrorEndpoints
}

// OverridePath implements the Registries interface.
func (r *RegistryMirrorConfig) OverridePath() bool {
	return pointer.SafeDeref(r.MirrorOverridePath)
}

// Content implements the config.Provider interface.
func (f *MachineFile) Content() string {
	return f.FileContent
}

// Permissions implements the config.Provider interface.
func (f *MachineFile) Permissions() os.FileMode {
	return os.FileMode(f.FilePermissions)
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
	return xslices.Map(d.DiskPartitions, func(p *DiskPartition) config.Partition { return p })
}

// Size implements the config.Provider interface.
func (p *DiskPartition) Size() uint64 {
	return uint64(p.DiskSize)
}

// MountPoint implements the config.Provider interface.
func (p *DiskPartition) MountPoint() string {
	return p.DiskMountPoint
}

// Provider implements the config.Provider interface.
func (e *EncryptionConfig) Provider() string {
	if e.EncryptionProvider == "" {
		return encryption.LUKS2
	}

	return e.EncryptionProvider
}

// Cipher implements the config.Provider interface.
func (e *EncryptionConfig) Cipher() string {
	return e.EncryptionCipher
}

// KeySize implements the config.Provider interface.
func (e *EncryptionConfig) KeySize() uint {
	return e.EncryptionKeySize
}

// BlockSize implements the config.Provider interface.
func (e *EncryptionConfig) BlockSize() uint64 {
	return e.EncryptionBlockSize
}

// Options implements the config.Provider interface.
func (e *EncryptionConfig) Options() []string {
	return e.EncryptionPerfOptions
}

// Keys implements the config.Provider interface.
func (e *EncryptionConfig) Keys() []config.EncryptionKey {
	return xslices.Map(e.EncryptionKeys, func(k *EncryptionKey) config.EncryptionKey { return k })
}

// Static implements the config.Provider interface.
func (e *EncryptionKey) Static() config.EncryptionKeyStatic {
	if e.KeyStatic == nil {
		return nil
	}

	return e.KeyStatic
}

// NodeID implements the config.Provider interface.
func (e *EncryptionKey) NodeID() config.EncryptionKeyNodeID {
	if e.KeyNodeID == nil {
		return nil
	}

	return e.KeyNodeID
}

// KMS implements the config.Provider interface.
func (e *EncryptionKey) KMS() config.EncryptionKeyKMS {
	if e.KeyKMS == nil {
		return nil
	}

	return e.KeyKMS
}

// TPM implements the config.Provider interface.
func (e *EncryptionKey) TPM() config.EncryptionKeyTPM {
	if e.KeyTPM == nil {
		return nil
	}

	return e.KeyTPM
}

// String implements the config.Provider interface.
func (e *EncryptionKeyNodeID) String() string {
	return "nodeid"
}

// String implements the config.Provider interface.
func (e *EncryptionKeyTPM) String() string {
	return "tpm"
}

// Slot implements the config.Provider interface.
func (e *EncryptionKey) Slot() int {
	return e.KeySlot
}

// Key implements the config.Provider interface.
func (e *EncryptionKeyStatic) Key() []byte {
	return []byte(e.KeyData)
}

// String implements the config.Provider interface.
func (e *EncryptionKeyStatic) String() string {
	return "static"
}

// Endpoint implements the config.Provider interface.
func (e *EncryptionKeyKMS) Endpoint() string {
	return e.KMSEndpoint
}

// String implements the config.Provider interface.
func (e *EncryptionKeyKMS) String() string {
	return "kms"
}

// Get implements the config.Provider interface.
func (e *SystemDiskEncryptionConfig) Get(label string) config.Encryption {
	switch label {
	case constants.StatePartitionLabel:
		if e.StatePartition == nil {
			return nil
		}

		return e.StatePartition
	case constants.EphemeralPartitionLabel:
		if e.EphemeralPartition == nil {
			return nil
		}

		return e.EphemeralPartition
	}

	return nil
}

// HostPath implements the config.VolumeMount interface.
func (v VolumeMountConfig) HostPath() string {
	return v.VolumeHostPath
}

// MountPath implements the config.VolumeMount interface.
func (v VolumeMountConfig) MountPath() string {
	return v.VolumeMountPath
}

var volumeNameSanitizer = strings.NewReplacer("/", "-", "_", "-", ".", "-")

// Name implements the config.VolumeMount interface.
func (v VolumeMountConfig) Name() string {
	return strings.Trim(volumeNameSanitizer.Replace(v.VolumeMountPath), "-")
}

// ReadOnly implements the config.VolumeMount interface.
func (v VolumeMountConfig) ReadOnly() bool {
	return v.VolumeReadOnly
}

// Rules implements config.Udev interface.
func (u *UdevConfig) Rules() []string {
	return u.UdevRules
}
