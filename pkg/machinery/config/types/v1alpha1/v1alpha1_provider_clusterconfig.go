// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/talos-systems/crypto/x509"
	talosnet "github.com/talos-systems/net"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// ClusterConfig implements config.ClusterConfig, config.Token, and config.ClusterNetwork interfaces.

// config.ClusterConfig methods.

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
	return c
}

// CertSANs implements the config.ClusterConfig interface.
func (c *ClusterConfig) CertSANs() []string {
	return c.APIServerConfig.CertSANs
}

// CA implements the config.ClusterConfig interface.
func (c *ClusterConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return c.ClusterCA
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

// Config implements the config.ClusterConfig interface.
func (c *ClusterConfig) Config(t machine.Type) (string, error) {
	return "", nil
}

// Etcd implements the config.ClusterConfig interface.
func (c *ClusterConfig) Etcd() config.Etcd {
	if c.EtcdConfig == nil {
		c.EtcdConfig = &EtcdConfig{}
	}

	return c.EtcdConfig
}

// Network implements the config.ClusterConfig interface.
func (c *ClusterConfig) Network() config.ClusterNetwork {
	return c
}

// LocalAPIServerPort implements the config.ClusterConfig interface.
func (c *ClusterConfig) LocalAPIServerPort() int {
	if c.ControlPlane.LocalAPIServerPort == 0 {
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

// ExtraManifestURLs implements the config.ClusterConfig interface.
func (c *ClusterConfig) ExtraManifestURLs() []string {
	return c.ExtraManifests
}

// ExtraManifestHeaderMap implements the config.ClusterConfig interface.
func (c *ClusterConfig) ExtraManifestHeaderMap() map[string]string {
	return c.ExtraManifestHeaders
}

// AdminKubeconfig implements the config.ClusterConfig interface.
func (c *ClusterConfig) AdminKubeconfig() config.AdminKubeconfig {
	return c.AdminKubeconfigConfig
}

// ScheduleOnMasters implements the config.ClusterConfig interface.
func (c *ClusterConfig) ScheduleOnMasters() bool {
	return c.AllowSchedulingOnMasters
}

// config.Token methods.

// ID implements the config.Token interface.
func (c *ClusterConfig) ID() string {
	parts := strings.Split(c.BootstrapToken, ".")
	if len(parts) != 2 {
		return ""
	}

	return parts[0]
}

// Secret implements the config.Token interface.
func (c *ClusterConfig) Secret() string {
	parts := strings.Split(c.BootstrapToken, ".")
	if len(parts) != 2 {
		return ""
	}

	return parts[1]
}

// config.ClusterNetwork methods.

// CNI implements the config.ClusterNetwork interface.
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

// PodCIDR implements the config.ClusterNetwork interface.
func (c *ClusterConfig) PodCIDR() string {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.PodSubnet) == 0:
		return constants.DefaultIPv4PodNet
	}

	return strings.Join(c.ClusterNetwork.PodSubnet, ",")
}

// ServiceCIDR implements the config.ClusterNetwork interface.
func (c *ClusterConfig) ServiceCIDR() string {
	switch {
	case c.ClusterNetwork == nil:
		fallthrough
	case len(c.ClusterNetwork.ServiceSubnet) == 0:
		return constants.DefaultIPv4ServiceNet
	}

	return strings.Join(c.ClusterNetwork.ServiceSubnet, ",")
}

// DNSDomain implements the config.ClusterNetwork interface.
func (c *ClusterConfig) DNSDomain() string {
	if c.ClusterNetwork == nil {
		return constants.DefaultDNSDomain
	}

	return c.ClusterNetwork.DNSDomain
}

// APIServerIPs implements the config.ClusterNetwork interface.
func (c *ClusterConfig) APIServerIPs() ([]net.IP, error) {
	serviceCIDRs, err := talosnet.SplitCIDRs(c.ServiceCIDR())
	if err != nil {
		return nil, fmt.Errorf("failed to process Service CIDRs: %w", err)
	}

	return talosnet.NthIPInCIDRSet(serviceCIDRs, 1)
}

// DNSServiceIPs implements the config.ClusterNetwork interface.
func (c *ClusterConfig) DNSServiceIPs() ([]net.IP, error) {
	serviceCIDRs, err := talosnet.SplitCIDRs(c.ServiceCIDR())
	if err != nil {
		return nil, fmt.Errorf("failed to process Service CIDRs: %w", err)
	}

	return talosnet.NthIPInCIDRSet(serviceCIDRs, 10)
}

// Check interfaces.
var (
	_ config.ClusterConfig  = (*ClusterConfig)(nil)
	_ config.Token          = (*ClusterConfig)(nil)
	_ config.ClusterNetwork = (*ClusterConfig)(nil)
)
