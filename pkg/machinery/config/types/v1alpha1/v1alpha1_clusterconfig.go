// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ClusterConfig implements config.ClusterConfig, config.Token, and config.ClusterNetwork interfaces.

// Name implements the config.ClusterConfig interface.
func (c *ClusterConfig) Name() string {
	return c.ClusterName
}

// APIServer implements the config.ClusterConfig interface.
func (c *ClusterConfig) APIServer() config.APIServer {
	if c.APIServerConfig == nil {
		return &APIServerConfig{}
	}

	return c.APIServerConfig
}

// ControllerManager implements the config.ClusterConfig interface.
func (c *ClusterConfig) ControllerManager() config.ControllerManager {
	if c.ControllerManagerConfig == nil {
		return &ControllerManagerConfig{}
	}

	return c.ControllerManagerConfig
}

// Proxy implements the config.ClusterConfig interface.
func (c *ClusterConfig) Proxy() config.Proxy {
	if c.ProxyConfig == nil {
		return &ProxyConfig{}
	}

	return c.ProxyConfig
}

// Scheduler implements the config.ClusterConfig interface.
func (c *ClusterConfig) Scheduler() config.Scheduler {
	if c.SchedulerConfig == nil {
		return &SchedulerConfig{}
	}

	return c.SchedulerConfig
}

// Endpoint implements the config.ClusterConfig interface.
func (c *ClusterConfig) Endpoint() *url.URL {
	return c.ControlPlane.Endpoint.URL
}

// Token implements the config.ClusterConfig interface.
func (c *ClusterConfig) Token() config.Token {
	return clusterToken(c.BootstrapToken)
}

// CertSANs implements the config.ClusterConfig interface.
func (c *ClusterConfig) CertSANs() []string {
	if c.APIServerConfig == nil {
		return nil
	}

	return c.APIServerConfig.CertSANs
}

// IssuingCA implements the config.ClusterConfig interface.
func (c *ClusterConfig) IssuingCA() *x509.PEMEncodedCertificateAndKey {
	return c.ClusterCA
}

// AcceptedCAs implements the config.ClusterConfig interface.
func (c *ClusterConfig) AcceptedCAs() []*x509.PEMEncodedCertificate {
	return slices.Clone(c.ClusterAcceptedCAs)
}

// AggregatorCA implements the config.ClusterConfig interface.
func (c *ClusterConfig) AggregatorCA() *x509.PEMEncodedCertificateAndKey {
	return c.ClusterAggregatorCA
}

// ServiceAccount implements the config.ClusterConfig interface.
func (c *ClusterConfig) ServiceAccount() *x509.PEMEncodedKey {
	return c.ClusterServiceAccount
}

// AESCBCEncryptionSecret implements the config.ClusterConfig interface.
func (c *ClusterConfig) AESCBCEncryptionSecret() string {
	return c.ClusterAESCBCEncryptionSecret
}

// SecretboxEncryptionSecret implements the config.ClusterConfig interface.
func (c *ClusterConfig) SecretboxEncryptionSecret() string {
	return c.ClusterSecretboxEncryptionSecret
}

// Etcd implements the config.ClusterConfig interface.
func (c *ClusterConfig) Etcd() config.Etcd {
	if c.EtcdConfig == nil {
		return &EtcdConfig{}
	}

	return c.EtcdConfig
}

// Network implements the config.ClusterConfig interface.
func (c *ClusterConfig) Network() config.ClusterNetwork {
	return c
}

// LocalAPIServerPort implements the config.ClusterConfig interface.
func (c *ClusterConfig) LocalAPIServerPort() int {
	if c.ControlPlane == nil || c.ControlPlane.LocalAPIServerPort == 0 {
		return constants.DefaultControlPlanePort
	}

	return c.ControlPlane.LocalAPIServerPort
}

// CoreDNS implements the config.ClusterConfig interface.
func (c *ClusterConfig) CoreDNS() config.CoreDNS {
	if c.CoreDNSConfig == nil {
		return &CoreDNS{}
	}

	return c.CoreDNSConfig
}

// ExternalCloudProvider implements the config.ClusterConfig interface.
func (c *ClusterConfig) ExternalCloudProvider() config.ExternalCloudProvider {
	if c.ExternalCloudProviderConfig == nil {
		return &ExternalCloudProviderConfig{}
	}

	return c.ExternalCloudProviderConfig
}

// ExtraManifestURLs implements the config.ClusterConfig interface.
func (c *ClusterConfig) ExtraManifestURLs() []string {
	return c.ExtraManifests
}

// ExtraManifestHeaderMap implements the config.ClusterConfig interface.
func (c *ClusterConfig) ExtraManifestHeaderMap() map[string]string {
	return c.ExtraManifestHeaders
}

// InlineManifests implements the config.ClusterConfig interface.
func (c *ClusterConfig) InlineManifests() []config.InlineManifest {
	return xslices.Map(c.ClusterInlineManifests, func(m ClusterInlineManifest) config.InlineManifest { return m })
}

// AdminKubeconfig implements the config.ClusterConfig interface.
func (c *ClusterConfig) AdminKubeconfig() config.AdminKubeconfig {
	if c.AdminKubeconfigConfig == nil {
		return &AdminKubeconfigConfig{}
	}

	return c.AdminKubeconfigConfig
}

// ScheduleOnControlPlanes implements the config.ClusterConfig interface.
func (c *ClusterConfig) ScheduleOnControlPlanes() bool {
	return pointer.SafeDeref(c.AllowSchedulingOnControlPlanes)
}

// ID returns the unique identifier for the cluster.
func (c *ClusterConfig) ID() string {
	return c.ClusterID
}

// Secret returns the cluster secret.
func (c *ClusterConfig) Secret() string {
	return c.ClusterSecret
}

// CNI implements the config.ClusterNetwork interface.
func (c *ClusterConfig) CNI() config.CNI {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough

	case c.ClusterNetwork.CNI == nil:
		return &CNIConfig{
			CNIName: constants.FlannelCNI,
		}
	}

	return c.ClusterNetwork.CNI
}

// PodCIDRs implements the config.ClusterNetwork interface.
func (c *ClusterConfig) PodCIDRs() []string {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.PodSubnet) == 0:
		return []string{constants.DefaultIPv4PodNet}
	}

	return c.ClusterNetwork.PodSubnet
}

// ServiceCIDRs implements the config.ClusterNetwork interface.
func (c *ClusterConfig) ServiceCIDRs() []string {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.ServiceSubnet) == 0:
		return []string{constants.DefaultIPv4ServiceNet}
	}

	return c.ClusterNetwork.ServiceSubnet
}

// DNSDomain implements the config.ClusterNetwork interface.
func (c *ClusterConfig) DNSDomain() string {
	if c.ClusterNetwork == nil || c.ClusterNetwork.DNSDomain == "" {
		return constants.DefaultDNSDomain
	}

	return c.ClusterNetwork.DNSDomain
}

// APIServerIPs implements the config.ClusterNetwork interface.
func (c *ClusterConfig) APIServerIPs() ([]netip.Addr, error) {
	serviceCIDRs, err := sideronet.SplitCIDRs(strings.Join(c.ServiceCIDRs(), ","))
	if err != nil {
		return nil, fmt.Errorf("failed to process Service CIDRs: %w", err)
	}

	return sideronet.NthIPInCIDRSet(serviceCIDRs, 1)
}

// DNSServiceIPs implements the config.ClusterNetwork interface.
func (c *ClusterConfig) DNSServiceIPs() ([]netip.Addr, error) {
	serviceCIDRs, err := sideronet.SplitCIDRs(strings.Join(c.ServiceCIDRs(), ","))
	if err != nil {
		return nil, fmt.Errorf("failed to process Service CIDRs: %w", err)
	}

	return sideronet.NthIPInCIDRSet(serviceCIDRs, 10)
}

// Discovery implements the config.Cluster interface.
func (c *ClusterConfig) Discovery() config.Discovery {
	if c.ClusterDiscoveryConfig == nil {
		return &ClusterDiscoveryConfig{}
	}

	return c.ClusterDiscoveryConfig
}

type clusterToken string

// ID implements the config.Token interface.
func (t clusterToken) ID() string {
	parts := strings.Split(string(t), ".")
	if len(parts) != 2 {
		return ""
	}

	return parts[0]
}

// Secret implements the config.Token interface.
func (t clusterToken) Secret() string {
	parts := strings.Split(string(t), ".")
	if len(parts) != 2 {
		return ""
	}

	return parts[1]
}
