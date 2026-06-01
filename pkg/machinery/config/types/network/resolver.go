// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// ResolverKind is a ResolverConfig document kind.
const ResolverKind = "ResolverConfig"

func init() {
	registry.Register(ResolverKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &ResolverConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkResolverConfig        = &ResolverConfigV1Alpha1{}
	_ config.NetworkHostDNSConfig         = &ResolverConfigV1Alpha1{}
	_ config.Validator                    = &ResolverConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &ResolverConfigV1Alpha1{}
)

// ResolverConfigV1Alpha1 is a config document to configure DNS resolving.
//
//	examples:
//	  - value: exampleResolverConfigV1Alpha1()
//	  - value: exampleResolverConfigV1Alpha2()
//	  - value: exampleResolverConfigV1Alpha3()
//	  - value: exampleResolverConfigV1Alpha4()
//	  - value: exampleResolverConfigV1Alpha5()
//	alias: ResolverConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/ResolverConfig
type ResolverConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     A list of nameservers (DNS servers) to use for resolving domain names.
	//
	//     Nameservers are used to resolve domain names on the host, and they are also
	//     propagated to Kubernetes DNS (CoreDNS) for use by pods running on the cluster.
	//
	//     This overrides any nameservers obtained via DHCP or platform configuration.
	//     Default configuration is to use 1.1.1.1 and 8.8.8.8 as nameservers.
	ResolverNameservers []NameserverConfig `yaml:"nameservers,omitempty"`
	//   description: |
	//     Configuration for search domains (in /etc/resolv.conf).
	//
	//     The default is to derive search domains from the hostname FQDN.
	ResolverSearchDomains SearchDomainsConfig `yaml:"searchDomains,omitempty"`
	//   description: |
	//     Configuration for host DNS resolver.
	//
	//     This configures a local DNS caching resolver on the host to improve DNS resolution performance and reliability.
	ResolverHostDNS HostDNSConfig `yaml:"hostDNS,omitempty"`
}

// NameserverConfig represents a single nameserver configuration.
type NameserverConfig struct {
	//   description: |
	//     The IP address of the nameserver.
	//   examples:
	//    - value: >
	//       Addr{netip.MustParseAddr("10.0.0.1")}
	//   schema:
	//     type: string
	//     pattern: ^[0-9a-f.:]+$
	Address Addr `yaml:"address"`
	//   description: |
	//     A DNS protocol to use.
	//
	//     The default protocol is plain DNS (`Do53`) (DNS over TCP/UDP). Set this to
	//     `DoT` to use DNS over TLS (RFC 7858) on TCP port 853, or `DoH` to use DNS
	//     over HTTPS (RFC 8484) on TCP port 443 with the `/dns-query` URL path. Both
	//     `DoT` and `DoH` deliver encrypted queries to this nameserver.
	//
	//     Note: encrypted DNS protocols require a correct system clock to validate
	//     certificates. If NTP is configured with hostnames that need to be resolved
	//     through DoT/DoH, the boot may stall: NTP needs DNS, and TLS needs valid
	//     time. Either rely on the hardware clock, configure NTP servers by IP, or
	//     keep at least one plain-DNS fallback nameserver.
	//   values:
	//     - "Do53"
	//     - "DoT"
	//     - "DoH"
	Protocol nethelpers.DNSProtocol `yaml:"protocol,omitempty"`
	//   description: |
	//     TLS server name to validate the nameserver certificate against.
	//
	//     This field should be set if the protocol is set to `DoT` or `DoH`.
	//     The value is used both as the SNI sent during the TLS handshake and as the
	//     name verified against the server certificate. For `DoH`, it is also used as
	//     the host portion of the request URL (`https://<tlsServerName>/dns-query`)
	//     while the connection itself is established to the configured `address`.
	//
	//   examples:
	//     - value: >
	//        "dns1.example.com"
	TLSServerName string `yaml:"tlsServerName,omitempty"`
}

// SearchDomainsConfig represents search domains configuration.
type SearchDomainsConfig struct {
	//   description: |
	//     A list of search domains to be used for DNS resolution.
	//
	//     Search domains are appended to unqualified domain names during DNS resolution.
	//     For example, if "example.com" is a search domain and a user tries to resolve
	//     "host", the system will attempt to resolve "host.example.com".
	//
	//     This overrides any search domains obtained via DHCP or platform configuration.
	//     The default configuration derives the search domain from the hostname FQDN.
	SearchDomains []string `yaml:"domains,omitempty"`
	//   description: |
	//     Disable default search domain configuration from hostname FQDN.
	//
	//     When set to true, the system will not derive search domains from the hostname FQDN.
	//     This allows for a custom configuration of search domains without any defaults.
	SearchDisableDefault *bool `yaml:"disableDefault,omitempty"`
}

// HostDNSConfig represents host DNS configuration.
type HostDNSConfig struct {
	//   description: |
	//     Enable host DNS caching resolver.
	//
	//     When enabled, a local DNS caching resolver is deployed on the host to improve DNS resolution performance and reliability.
	//     Upstream DNS servers for the host resolver are configured using the `nameservers` field in this config document.
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

// NewResolverConfigV1Alpha1 creates a new ResolverConfig config document.
func NewResolverConfigV1Alpha1() *ResolverConfigV1Alpha1 {
	return &ResolverConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       ResolverKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleResolverConfigV1Alpha1() *ResolverConfigV1Alpha1 {
	cfg := NewResolverConfigV1Alpha1()
	cfg.ResolverNameservers = []NameserverConfig{
		{
			Address: Addr{netip.MustParseAddr("1.1.1.1")},
		},
		{
			Address: Addr{netip.MustParseAddr("ff08::1")},
		},
	}
	cfg.ResolverSearchDomains = SearchDomainsConfig{
		SearchDomains: []string{"example.com"},
	}

	return cfg
}

func exampleResolverConfigV1Alpha2() *ResolverConfigV1Alpha1 {
	cfg := NewResolverConfigV1Alpha1()
	cfg.ResolverSearchDomains = SearchDomainsConfig{
		SearchDisableDefault: new(true),
	}

	return cfg
}

func exampleResolverConfigV1Alpha3() *ResolverConfigV1Alpha1 {
	cfg := NewResolverConfigV1Alpha1()
	cfg.ResolverHostDNS = HostDNSConfig{
		HostDNSEnabled:              new(true),
		HostDNSForwardKubeDNSToHost: new(true),
		HostDNSResolveMemberNames:   new(true),
	}

	return cfg
}

func exampleResolverConfigV1Alpha4() *ResolverConfigV1Alpha1 {
	cfg := NewResolverConfigV1Alpha1()
	cfg.ResolverNameservers = []NameserverConfig{
		{
			Address:       Addr{netip.MustParseAddr("9.9.9.9")},
			Protocol:      nethelpers.DNSProtocolDNSOverTLS,
			TLSServerName: "dns.quad9.net",
		},
		{
			Address:       Addr{netip.MustParseAddr("2620:fe::fe")},
			Protocol:      nethelpers.DNSProtocolDNSOverTLS,
			TLSServerName: "dns.quad9.net",
		},
	}

	return cfg
}

func exampleResolverConfigV1Alpha5() *ResolverConfigV1Alpha1 {
	cfg := NewResolverConfigV1Alpha1()
	cfg.ResolverNameservers = []NameserverConfig{
		{
			Address:       Addr{netip.MustParseAddr("1.1.1.1")},
			Protocol:      nethelpers.DNSProtocolDNSOverHTTP,
			TLSServerName: "cloudflare-dns.com",
		},
		{
			Address:       Addr{netip.MustParseAddr("2606:4700:4700::1111")},
			Protocol:      nethelpers.DNSProtocolDNSOverHTTP,
			TLSServerName: "cloudflare-dns.com",
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *ResolverConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *ResolverConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.SearchDomains() != nil {
		return errors.New(".machine.network.searchDomains is already set in v1alpha1 config")
	}

	if v1alpha1Cfg.Resolvers() != nil {
		return errors.New(".machine.network.nameservers is already set in v1alpha1 config")
	}

	if v1alpha1Cfg.DisableSearchDomain() {
		return errors.New(".machine.network.disableSearchDomain is already set in v1alpha1 config")
	}

	if !value.IsZero(s.ResolverHostDNS) {
		if v1alpha1Cfg.NetworkHostDNSConfig() != nil {
			return errors.New(".machine.features.hostDNS is already set in v1alpha1 config")
		}
	}

	return nil
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s *ResolverConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings []string
		errs     error
	)

	if !value.IsZero(s.ResolverHostDNS) {
		if !s.HostDNSEnabled() {
			if s.ForwardKubeDNSToHost() {
				errs = errors.Join(errs, errors.New("hostDNS.forwardKubeDNSToHost cannot be enabled when hostDNS.enabled is false"))
			}

			if s.ResolveMemberNames() {
				errs = errors.Join(errs, errors.New("hostDNS.resolveMemberNames cannot be enabled when hostDNS.enabled is false"))
			}
		}
	}

	nonRegularDNS := 0

	for idx, ns := range s.ResolverNameservers {
		switch ns.Protocol {
		case nethelpers.DNSProtocolDNSOverTLS:
			nonRegularDNS++

			if ns.TLSServerName == "" {
				errs = errors.Join(errs, fmt.Errorf("tlsServerName must be set when protocol is DoT: entry %d", idx))
			}

		case nethelpers.DNSProtocolDNSOverHTTP:
			nonRegularDNS++

			if ns.TLSServerName == "" {
				errs = errors.Join(errs, fmt.Errorf("tlsServerName must be set when protocol is DoH: entry %d", idx))
			}

		case nethelpers.DNSProtocolDefault:
			if ns.TLSServerName != "" {
				errs = errors.Join(errs, fmt.Errorf("tlsServerName must be empty when protocol is Do53: entry %d", idx))
			}
		default:
			errs = errors.Join(errs, fmt.Errorf("unsupported DNS protocol: entry %d", idx))
		}

		if !ns.Address.IsValid() {
			errs = errors.Join(errs, fmt.Errorf("nameserver address must be a valid IP: entry %d", idx))
		}
	}

	if nonRegularDNS > 0 && nonRegularDNS == len(s.ResolverNameservers) {
		warnings = append(
			warnings,
			"all configured nameservers use encrypted DNS (DoT or DoH): validating certificates requires a correct system clock, "+
				"so boot may stall when NTP servers are configured by hostname; consider keeping at least one plain-DNS fallback "+
				"or configuring NTP servers by IP address",
		)
	}

	return warnings, errs
}

// Resolvers implements NetworkResolverConfig interface.
func (s *ResolverConfigV1Alpha1) Resolvers() []config.NetworkResolver {
	return xslices.Map(s.ResolverNameservers, func(ns NameserverConfig) config.NetworkResolver {
		return config.NetworkResolver{
			Addr:          ns.Address.Addr,
			Protocol:      ns.Protocol,
			TLSServerName: ns.TLSServerName,
		}
	})
}

// SearchDomains implements NetworkResolverConfig interface.
func (s *ResolverConfigV1Alpha1) SearchDomains() []string {
	return slices.Clone(s.ResolverSearchDomains.SearchDomains)
}

// DisableSearchDomain implements NetworkResolverConfig interface.
func (s *ResolverConfigV1Alpha1) DisableSearchDomain() bool {
	return pointer.SafeDeref(s.ResolverSearchDomains.SearchDisableDefault)
}

// HostDNSEnabled implements NetworkHostDNSConfig interface.
func (s *ResolverConfigV1Alpha1) HostDNSEnabled() bool {
	return pointer.SafeDeref(s.ResolverHostDNS.HostDNSEnabled)
}

// ForwardKubeDNSToHost implements NetworkHostDNSConfig interface.
func (s *ResolverConfigV1Alpha1) ForwardKubeDNSToHost() bool {
	return pointer.SafeDeref(s.ResolverHostDNS.HostDNSForwardKubeDNSToHost)
}

// ResolveMemberNames implements NetworkHostDNSConfig interface.
func (s *ResolverConfigV1Alpha1) ResolveMemberNames() bool {
	return pointer.SafeDeref(s.ResolverHostDNS.HostDNSResolveMemberNames)
}
