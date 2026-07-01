// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"net/netip"
	"net/url"
	"slices"
	"strings"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ClusterConfig implements config.ClusterConfig, config.Token, and config.ClusterNetwork interfaces.

// Name implements the config.ClusterConfig interface.
func (c *ClusterConfig) Name() string {
	return c.ClusterName
}

// APIServer implements the config.ClusterConfig interface.
func (c *ClusterConfig) APIServer() *APIServerConfig {
	if c.APIServerConfig == nil {
		return &APIServerConfig{}
	}

	return c.APIServerConfig
}

// ControllerManager implements the config.ClusterConfig interface.
func (c *ClusterConfig) ControllerManager() *ControllerManagerConfig {
	if c.ControllerManagerConfig == nil {
		return &ControllerManagerConfig{}
	}

	return c.ControllerManagerConfig
}

// Proxy implements the config.ClusterConfig interface.
func (c *ClusterConfig) Proxy() *ProxyConfig {
	if c.ProxyConfig == nil {
		return &ProxyConfig{}
	}

	return c.ProxyConfig
}

// Scheduler implements the config.ClusterConfig interface.
func (c *ClusterConfig) Scheduler() *SchedulerConfig {
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

	return c.APIServerConfig.ExtraCertSANs
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

// LocalAPIServerPort implements the config.ClusterConfig interface.
func (c *ClusterConfig) LocalAPIServerPort() int {
	if c.ControlPlane == nil || c.ControlPlane.LocalAPIServerPort == 0 {
		return constants.DefaultControlPlanePort
	}

	return c.ControlPlane.LocalAPIServerPort
}

// CoreDNS implements the config.ClusterConfig interface.
func (c *ClusterConfig) CoreDNS() *CoreDNS {
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
	result := slices.Clone(c.ExtraManifests)

	if c.ClusterNetwork != nil && c.ClusterNetwork.CNI != nil {
		result = slices.Concat(result, c.ClusterNetwork.CNI.CNIUrls)
	}

	return result
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
	if c.AllowSchedulingOnControlPlanes != nil {
		return pointer.SafeDeref(c.AllowSchedulingOnControlPlanes)
	}

	return pointer.SafeDeref(c.AllowSchedulingOnMasters)
}

// ID returns the unique identifier for the cluster.
func (c *ClusterConfig) ID() string {
	return c.ClusterID //nolint:staticcheck // legacy configuration
}

// Secret returns the cluster secret.
func (c *ClusterConfig) Secret() string {
	return c.ClusterSecret //nolint:staticcheck // legacy configuration
}

// CNI implements the config.ClusterNetwork interface.
func (c *ClusterConfig) CNI() *CNIConfig {
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
func (c *ClusterConfig) PodCIDRs() []netip.Prefix {
	var subnets []string

	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.PodSubnet) == 0:
		subnets = []string{constants.DefaultIPv4PodNet}
	default:
		subnets = c.ClusterNetwork.PodSubnet
	}

	return xslices.Map(subnets, func(s string) netip.Prefix {
		ip, _ := netip.ParsePrefix(s) //nolint:errcheck // the subnets are validated

		return ip
	})
}

// ServiceCIDRs implements the config.ClusterNetwork interface.
func (c *ClusterConfig) ServiceCIDRs() []netip.Prefix {
	var subnets []string

	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.ServiceSubnet) == 0:
		subnets = []string{constants.DefaultIPv4ServiceNet}
	default:
		subnets = c.ClusterNetwork.ServiceSubnet
	}

	return xslices.Map(subnets, func(s string) netip.Prefix {
		ip, _ := netip.ParsePrefix(s) //nolint:errcheck // the subnets are validated

		return ip
	})
}

// DNSDomain implements the config.ClusterNetwork interface.
func (c *ClusterConfig) DNSDomain() string {
	if c.ClusterNetwork == nil || c.ClusterNetwork.DNSDomain == "" {
		return constants.DefaultDNSDomain
	}

	return c.ClusterNetwork.DNSDomain
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
