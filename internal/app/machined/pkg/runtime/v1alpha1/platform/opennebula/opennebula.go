// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package opennebula

import (
	"bytes"
	"context"
	"encoding/base64"
	stderrors "errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-envparse"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/address"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const methodSkip = "skip"

// OpenNebula is the concrete type that implements the runtime.Platform interface.
type OpenNebula struct{}

// Name implements the runtime.Platform interface.
func (o *OpenNebula) Name() string {
	return "opennebula"
}

// isDigitsOnly returns true if s is non-empty and contains only ASCII digits.
func isDigitsOnly(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return s != ""
}

// collectAliasNames scans oneContext for keys of the form
// <aliasPrefix><digits>_MAC and returns the sorted list of alias base names
// (e.g. "ETH0_ALIAS0", "ETH0_ALIAS1").
func collectAliasNames(oneContext map[string]string, aliasPrefix string) []string {
	seen := map[string]bool{}

	var aliasNames []string

	for key := range oneContext {
		if !strings.HasPrefix(key, aliasPrefix) || !strings.HasSuffix(key, "_MAC") {
			continue
		}

		middle := strings.TrimPrefix(strings.TrimSuffix(key, "_MAC"), aliasPrefix)
		if !isDigitsOnly(middle) {
			continue
		}

		aliasName := aliasPrefix + middle
		if !seen[aliasName] {
			seen[aliasName] = true
			aliasNames = append(aliasNames, aliasName)
		}
	}

	slices.Sort(aliasNames)

	return aliasNames
}

// parseAlias parses the addresses for a single alias entry. Returns nil, nil
// when the alias should be skipped (DETACH non-empty or EXTERNAL=YES).
func parseAlias(oneContext map[string]string, aliasName, ifaceNameLower string) ([]network.AddressSpecSpec, error) {
	// Skip detached aliases — reference: [ -z "${detach}" ]
	if oneContext[aliasName+"_DETACH"] != "" {
		return nil, nil
	}

	// Skip externally managed aliases — reference: ! is_true "${external}"
	if strings.EqualFold(oneContext[aliasName+"_EXTERNAL"], "yes") {
		return nil, nil
	}

	var addrs []network.AddressSpecSpec

	if ipStr := oneContext[aliasName+"_IP"]; ipStr != "" {
		ipPrefix, err := address.IPPrefixFrom(ipStr, oneContext[aliasName+"_MASK"])
		if err != nil {
			return nil, fmt.Errorf("alias %s: failed to parse IPv4: %w", aliasName, err)
		}

		addrs = append(addrs, network.AddressSpecSpec{
			Address:     ipPrefix,
			LinkName:    ifaceNameLower,
			Family:      nethelpers.FamilyInet4,
			Scope:       nethelpers.ScopeGlobal,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			ConfigLayer: network.ConfigPlatform,
		})
	}

	ip6Str := oneContext[aliasName+"_IP6"]
	if ip6Str == "" {
		ip6Str = oneContext[aliasName+"_IPV6"]
	}

	if ip6Str != "" {
		ip6Prefix, err := ip6PrefixFrom(ip6Str, oneContext[aliasName+"_IP6_PREFIX_LENGTH"])
		if err != nil {
			return nil, fmt.Errorf("alias %s: failed to parse IPv6: %w", aliasName, err)
		}

		addrs = append(addrs, network.AddressSpecSpec{
			Address:     ip6Prefix,
			LinkName:    ifaceNameLower,
			Family:      nethelpers.FamilyInet6,
			Scope:       nethelpers.ScopeGlobal,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			ConfigLayer: network.ConfigPlatform,
		})
	}

	if ulaStr := oneContext[aliasName+"_IP6_ULA"]; ulaStr != "" {
		ulaPrefix, err := ip6PrefixFrom(ulaStr, "64")
		if err != nil {
			return nil, fmt.Errorf("alias %s: failed to parse IPv6 ULA: %w", aliasName, err)
		}

		addrs = append(addrs, network.AddressSpecSpec{
			Address:     ulaPrefix,
			LinkName:    ifaceNameLower,
			Family:      nethelpers.FamilyInet6,
			Scope:       nethelpers.ScopeGlobal,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			ConfigLayer: network.ConfigPlatform,
		})
	}

	return addrs, nil
}

// parseAliases collects ETHn_ALIASm_* address entries for a given interface.
// An alias is skipped when DETACH is non-empty OR EXTERNAL=YES, matching the
// reference netcfg-networkd behavior (lines 395-400).
func parseAliases(oneContext map[string]string, ifaceName, ifaceNameLower string) ([]network.AddressSpecSpec, error) {
	aliasNames := collectAliasNames(oneContext, ifaceName+"_ALIAS")

	var addrs []network.AddressSpecSpec

	for _, aliasName := range aliasNames {
		aliasAddrs, err := parseAlias(oneContext, aliasName, ifaceNameLower)
		if err != nil {
			return nil, err
		}

		addrs = append(addrs, aliasAddrs...)
	}

	return addrs, nil
}

// sanitizeHostname replaces characters invalid in DNS labels with hyphens,
// strips leading/trailing hyphens from the whole string and from each label.
// This mirrors the reference sanitization in one-apps/context-linux:
//
//	sed -e 's/[^-a-zA-Z0-9\.]/-/g' -e 's/^-*//g' -e 's/-*$//g'
//
// Talos is intentionally stricter: it also trims hyphens per-label so every
// label is RFC-1123-valid (no label may start or end with a hyphen).
func sanitizeHostname(raw string) string {
	var b strings.Builder

	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}

	s := strings.Trim(b.String(), "-")

	labels := strings.Split(s, ".")
	for i, l := range labels {
		labels[i] = strings.Trim(l, "-")
	}

	return strings.Join(labels, ".")
}

// parseRouteFields extracts the destination prefix, gateway string, and optional
// metric string from the fields of a single route entry.
//
// The reference one-apps implementation (context-linux) always parses routes as:
//
//	rsplit=( ${route} ); dst="${rsplit[0]}"; gw="${rsplit[2]}"
//
// meaning token[1] is always skipped and the gateway is always at token[2].
// The canonical format is "DEST/PREFIX via GW" where "via" occupies token[1].
// The legacy dotted-mask format "DEST MASK GW" follows the same index layout.
//
// As a Talos extension, an optional bare metric may follow the gateway.
func parseRouteFields(parts []string) (dest netip.Prefix, gwStr, metricStr string, err error) {
	// Both CIDR ("DEST/PREFIX via GW") and legacy ("DEST MASK GW") formats
	// require at least 3 tokens, with the gateway always at index 2.
	if len(parts) < 3 {
		return dest, "", "", fmt.Errorf("expected at least 3 fields (DEST/PREFIX via GW or DEST MASK GW)")
	}

	if strings.Contains(parts[0], "/") {
		// CIDR format: "DEST/PREFIX via GW [METRIC]"
		// parts[1] is the separator token (conventionally "via") and is skipped,
		// matching the reference rsplit[1] which is never read.
		dest, err = netip.ParsePrefix(parts[0])
		if err != nil {
			return dest, "", "", fmt.Errorf("failed to parse destination: %w", err)
		}

		dest = dest.Masked()
	} else {
		// Legacy format: "DEST MASK GW [METRIC]"
		var prefix netip.Prefix

		prefix, err = address.IPPrefixFrom(parts[0], parts[1])
		if err != nil {
			return dest, "", "", fmt.Errorf("failed to parse destination: %w", err)
		}

		dest = prefix.Masked()
	}

	gwStr = parts[2]

	if len(parts) >= 4 {
		metricStr = parts[3]
	}

	return dest, gwStr, metricStr, nil
}

// parseRouteEntry parses a single trimmed route entry into a RouteSpecSpec.
func parseRouteEntry(entry, linkName string) (network.RouteSpecSpec, error) {
	dest, gwStr, metricStr, err := parseRouteFields(strings.Fields(entry))
	if err != nil {
		return network.RouteSpecSpec{}, fmt.Errorf("route entry %q: %w", entry, err)
	}

	gw, err := netip.ParseAddr(gwStr)
	if err != nil {
		return network.RouteSpecSpec{}, fmt.Errorf("route entry %q: failed to parse gateway: %w", entry, err)
	}

	metric := uint32(network.DefaultRouteMetric)

	if metricStr != "" {
		m, err := strconv.ParseUint(metricStr, 10, 32)
		if err != nil {
			return network.RouteSpecSpec{}, fmt.Errorf("route entry %q: failed to parse metric: %w", entry, err)
		}

		metric = uint32(m)
	}

	family := nethelpers.FamilyInet4
	if gw.Is6() {
		family = nethelpers.FamilyInet6
	}

	route := network.RouteSpecSpec{
		ConfigLayer: network.ConfigPlatform,
		Destination: dest,
		Gateway:     gw,
		OutLinkName: linkName,
		Table:       nethelpers.TableMain,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Family:      family,
		Priority:    metric,
	}

	route.Normalize()

	return route, nil
}

// ParseRoutes parses the ETH*_ROUTES variable into RouteSpecSpec entries.
// Multiple routes are separated by commas.
func ParseRoutes(routesStr, linkName string) ([]network.RouteSpecSpec, error) {
	var routes []network.RouteSpecSpec

	for entry := range strings.SplitSeq(routesStr, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		route, err := parseRouteEntry(entry, linkName)
		if err != nil {
			return nil, err
		}

		routes = append(routes, route)
	}

	return routes, nil
}

// parseIPv4StaticConfig handles the static addressing path for an interface:
// address, link, gateway route, extra static routes, and per-interface DNS.
func parseIPv4StaticConfig(
	oneContext map[string]string, ifaceName, ifaceNameLower string, routeMetric uint32,
	networkConfig *runtime.PlatformNetworkConfig, allDNSIPs *[]netip.Addr, allSearchDomains *[]string,
) error {
	ipPrefix, err := address.IPPrefixFrom(oneContext[ifaceName+"_IP"], oneContext[ifaceName+"_MASK"])
	if err != nil {
		return fmt.Errorf("failed to parse IP address: %w", err)
	}

	networkConfig.Addresses = append(
		networkConfig.Addresses,
		network.AddressSpecSpec{
			Address:         ipPrefix,
			LinkName:        ifaceNameLower,
			Family:          nethelpers.FamilyInet4,
			Scope:           nethelpers.ScopeGlobal,
			Flags:           nethelpers.AddressFlags(nethelpers.AddressPermanent),
			AnnounceWithARP: false,
			ConfigLayer:     network.ConfigPlatform,
		},
	)

	var mtu uint32

	if mtuStr := oneContext[ifaceName+"_MTU"]; mtuStr != "" {
		mtu64, err := strconv.ParseUint(mtuStr, 10, 32)
		if err != nil {
			return fmt.Errorf("failed to parse MTU: %w", err)
		}

		mtu = uint32(mtu64)
	}

	networkConfig.Links = append(
		networkConfig.Links,
		network.LinkSpecSpec{
			Name:        ifaceNameLower,
			Logical:     false,
			Up:          true,
			MTU:         mtu,
			Kind:        "",
			Type:        nethelpers.LinkEther,
			ParentName:  "",
			ConfigLayer: network.ConfigPlatform,
		},
	)

	if gwStr := oneContext[ifaceName+"_GATEWAY"]; gwStr != "" {
		gateway, err := netip.ParseAddr(gwStr)
		if err != nil {
			return fmt.Errorf("failed to parse gateway ip: %w", err)
		}

		route := network.RouteSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			Gateway:     gateway,
			OutLinkName: ifaceNameLower,
			Table:       nethelpers.TableMain,
			Protocol:    nethelpers.ProtocolStatic,
			Type:        nethelpers.TypeUnicast,
			Family:      nethelpers.FamilyInet4,
			Priority:    routeMetric,
		}

		route.Normalize()

		networkConfig.Routes = append(networkConfig.Routes, route)
	}

	if routesStr := oneContext[ifaceName+"_ROUTES"]; routesStr != "" {
		staticRoutes, err := ParseRoutes(routesStr, ifaceNameLower)
		if err != nil {
			return fmt.Errorf("interface %s: %w", ifaceName, err)
		}

		networkConfig.Routes = append(networkConfig.Routes, staticRoutes...)
	}

	for s := range strings.FieldsSeq(oneContext[ifaceName+"_DNS"]) {
		ip, err := netip.ParseAddr(s)
		if err != nil {
			return fmt.Errorf("interface %s: failed to parse DNS server %q: %w", ifaceName, s, err)
		}

		*allDNSIPs = append(*allDNSIPs, ip)
	}

	*allSearchDomains = append(*allSearchDomains, strings.Fields(oneContext[ifaceName+"_SEARCH_DOMAIN"])...)

	return nil
}

// parseIPv4Metric reads ETH*_METRIC and returns the parsed value, or 0 when
// the variable is absent. Callers apply their own default (e.g.
// network.DefaultRouteMetric for IPv4, 1 for IPv6 via parseIPv6Metric).
func parseIPv4Metric(oneContext map[string]string, ifaceName string) (uint32, error) {
	if metricStr := oneContext[ifaceName+"_METRIC"]; metricStr != "" {
		m, err := strconv.ParseUint(metricStr, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("interface %s: failed to parse metric: %w", ifaceName, err)
		}

		return uint32(m), nil
	}

	return 0, nil
}

// parseIPv6Metric reads ETH*_IP6_METRIC; falls back to ipv4Metric (when > 0),
// then to 1, matching the reference [ -z "$ip6_metric" ] && ip6_metric="${metric}".
func parseIPv6Metric(oneContext map[string]string, ifaceName string, ipv4Metric uint32) (uint32, error) {
	if metricStr := oneContext[ifaceName+"_IP6_METRIC"]; metricStr != "" {
		m, err := strconv.ParseUint(metricStr, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("interface %s: failed to parse IPv6 metric: %w", ifaceName, err)
		}

		return uint32(m), nil
	}

	if ipv4Metric > 0 {
		return ipv4Metric, nil
	}

	return 1, nil
}

// parseInterfaceIPv4 configures the IPv4 stack for one interface.
// Dispatches to DHCP4 operator or static config based on ETH*_METHOD.
func parseInterfaceIPv4(
	oneContext map[string]string, ifaceName, ifaceNameLower string, routeMetric uint32,
	networkConfig *runtime.PlatformNetworkConfig, allDNSIPs *[]netip.Addr, allSearchDomains *[]string,
) error {
	if oneContext[ifaceName+"_METHOD"] == methodSkip {
		return nil
	}

	if routeMetric == 0 {
		routeMetric = uint32(network.DefaultRouteMetric)
	}

	if oneContext[ifaceName+"_METHOD"] == "dhcp" {
		networkConfig.Operators = append(
			networkConfig.Operators,
			network.OperatorSpecSpec{
				Operator:  network.OperatorDHCP4,
				LinkName:  ifaceNameLower,
				RequireUp: true,
				DHCP4: network.DHCP4OperatorSpec{
					RouteMetric:         routeMetric,
					SkipHostnameRequest: true,
				},
				ConfigLayer: network.ConfigPlatform,
			},
		)

		return nil
	}

	return parseIPv4StaticConfig(oneContext, ifaceName, ifaceNameLower, routeMetric, networkConfig, allDNSIPs, allSearchDomains)
}

// ip6PrefixFrom builds a netip.Prefix from an IPv6 address string and an
// optional prefix-length string (default 64). The prefix is not masked so the
// full host address is preserved on the interface.
func ip6PrefixFrom(ipStr, prefixLenStr string) (netip.Prefix, error) {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("failed to parse IPv6 address %q: %w", ipStr, err)
	}

	bits := 64

	if prefixLenStr != "" {
		n, err := strconv.Atoi(prefixLenStr)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("failed to parse IPv6 prefix length %q: %w", prefixLenStr, err)
		}

		bits = n
	}

	return netip.PrefixFrom(ip, bits), nil
}

// parseIPv6Gateway reads ETH*_IP6_GATEWAY (or legacy GATEWAY6) and emits the
// default IPv6 route (::/0) with metric from parseIPv6Metric.
func parseIPv6Gateway(oneContext map[string]string, ifaceName, ifaceNameLower string, ipv4Metric uint32, networkConfig *runtime.PlatformNetworkConfig) error {
	gwStr := oneContext[ifaceName+"_IP6_GATEWAY"]
	if gwStr == "" {
		gwStr = oneContext[ifaceName+"_GATEWAY6"]
	}

	if gwStr == "" {
		return nil
	}

	gw, err := netip.ParseAddr(gwStr)
	if err != nil {
		return fmt.Errorf("interface %s: failed to parse IPv6 gateway %q: %w", ifaceName, gwStr, err)
	}

	metric, err := parseIPv6Metric(oneContext, ifaceName, ipv4Metric)
	if err != nil {
		return err
	}

	route := network.RouteSpecSpec{
		ConfigLayer: network.ConfigPlatform,
		Gateway:     gw,
		OutLinkName: ifaceNameLower,
		Table:       nethelpers.TableMain,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Family:      nethelpers.FamilyInet6,
		Priority:    metric,
	}

	route.Normalize()

	networkConfig.Routes = append(networkConfig.Routes, route)

	return nil
}

// parseIPv6DHCP emits a DHCPv6 operator for an interface, with metric from
// parseIPv6Metric.
func parseIPv6DHCP(oneContext map[string]string, ifaceName, ifaceNameLower string, ipv4Metric uint32, networkConfig *runtime.PlatformNetworkConfig) error {
	metric, err := parseIPv6Metric(oneContext, ifaceName, ipv4Metric)
	if err != nil {
		return err
	}

	networkConfig.Operators = append(networkConfig.Operators, network.OperatorSpecSpec{
		Operator:  network.OperatorDHCP6,
		LinkName:  ifaceNameLower,
		RequireUp: true,
		DHCP6: network.DHCP6OperatorSpec{
			RouteMetric:         metric,
			SkipHostnameRequest: true,
		},
		ConfigLayer: network.ConfigPlatform,
	})

	return nil
}

// parseInterfaceIPv6 configures the IPv6 stack for one interface.
// Dispatches on the effective IP6_METHOD: disable/skip (no-op), auto (SLAAC),
// dhcp (DHCPv6 operator), or static/empty (static address path).
// When IP6_METHOD is unset, ipv4Method is used as fallback, matching the
// reference: [ -z "$ip6_method" ] && ip6_method="${method}".
func parseInterfaceIPv6(oneContext map[string]string, ifaceName, ifaceNameLower string, ipv4Method string, ipv4Metric uint32, networkConfig *runtime.PlatformNetworkConfig) error {
	ip6Method := strings.ToLower(oneContext[ifaceName+"_IP6_METHOD"])
	if ip6Method == "" {
		ip6Method = ipv4Method
	}

	switch ip6Method {
	case "disable", methodSkip:
		return nil
	case "auto":
		// SLAAC: the kernel accepts Router Advertisements by default in Talos;
		// no operator or sysctl is required to enable address auto-configuration.
		return nil
	case "dhcp":
		return parseIPv6DHCP(oneContext, ifaceName, ifaceNameLower, ipv4Metric, networkConfig)
	}

	ip6Str := oneContext[ifaceName+"_IP6"]
	if ip6Str == "" {
		ip6Str = oneContext[ifaceName+"_IPV6"]
	}

	prefixLenStr := oneContext[ifaceName+"_IP6_PREFIX_LENGTH"]

	if ip6Str != "" {
		ip6Prefix, err := ip6PrefixFrom(ip6Str, prefixLenStr)
		if err != nil {
			return fmt.Errorf("interface %s: %w", ifaceName, err)
		}

		networkConfig.Addresses = append(networkConfig.Addresses, network.AddressSpecSpec{
			Address:     ip6Prefix,
			LinkName:    ifaceNameLower,
			Family:      nethelpers.FamilyInet6,
			Scope:       nethelpers.ScopeGlobal,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			ConfigLayer: network.ConfigPlatform,
		})
	}

	if ulaStr := oneContext[ifaceName+"_IP6_ULA"]; ulaStr != "" {
		ulaPrefix, err := ip6PrefixFrom(ulaStr, "64")
		if err != nil {
			return fmt.Errorf("interface %s ULA: %w", ifaceName, err)
		}

		networkConfig.Addresses = append(networkConfig.Addresses, network.AddressSpecSpec{
			Address:     ulaPrefix,
			LinkName:    ifaceNameLower,
			Family:      nethelpers.FamilyInet6,
			Scope:       nethelpers.ScopeGlobal,
			Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
			ConfigLayer: network.ConfigPlatform,
		})
	}

	return parseIPv6Gateway(oneContext, ifaceName, ifaceNameLower, ipv4Metric, networkConfig)
}

// parseInterface runs all per-interface configuration (IPv4, IPv6, aliases).
func parseInterface(oneContext map[string]string, ifaceName string, networkConfig *runtime.PlatformNetworkConfig, allDNSIPs *[]netip.Addr, allSearchDomains *[]string) error {
	ifaceNameLower := strings.ToLower(ifaceName)
	ipv4Method := strings.ToLower(oneContext[ifaceName+"_METHOD"])

	ip6Method := strings.ToLower(oneContext[ifaceName+"_IP6_METHOD"])
	if ip6Method == "" {
		ip6Method = ipv4Method
	}

	if ipv4Method == methodSkip && (ip6Method == "" || ip6Method == methodSkip || ip6Method == "disable") {
		return nil
	}

	ipv4Metric, err := parseIPv4Metric(oneContext, ifaceName)
	if err != nil {
		return err
	}

	if err := parseInterfaceIPv4(oneContext, ifaceName, ifaceNameLower, ipv4Metric, networkConfig, allDNSIPs, allSearchDomains); err != nil {
		return err
	}

	if err := parseInterfaceIPv6(oneContext, ifaceName, ifaceNameLower, ipv4Method, ipv4Metric, networkConfig); err != nil {
		return err
	}

	aliasAddrs, err := parseAliases(oneContext, ifaceName, ifaceNameLower)
	if err != nil {
		return err
	}

	networkConfig.Addresses = append(networkConfig.Addresses, aliasAddrs...)

	return nil
}

// ethInterfaceName returns the interface name (e.g. "ETH0") from a context map
// key of the form ETH<digits>_MAC, or ("", false) for any other key.
func ethInterfaceName(key string) (string, bool) {
	if !strings.HasPrefix(key, "ETH") || !strings.HasSuffix(key, "_MAC") {
		return "", false
	}

	name := strings.TrimSuffix(key, "_MAC")

	if !isDigitsOnly(strings.TrimPrefix(name, "ETH")) {
		return "", false
	}

	return name, true
}

// resolveHostname reads SET_HOSTNAME from the context map and sanitizes it,
// matching the reference net-15-hostname script precedence. HOSTNAME and NAME
// are not used — the reference never reads them for hostname configuration.
// DNS_HOSTNAME is a server-side flag that triggers a reverse DNS lookup
// (a live network operation) and cannot be honored inside ParseMetadata.
func resolveHostname(oneContext map[string]string) string {
	return sanitizeHostname(oneContext["SET_HOSTNAME"])
}

// extractIPv4FromEndpoint extracts the host IPv4 address from a URL-like
// string (e.g. "http://169.254.16.9:5030"). Returns an invalid Addr if no
// IPv4 address can be parsed from the host portion.
func extractIPv4FromEndpoint(endpoint string) netip.Addr {
	s := endpoint

	// Strip scheme (e.g. "http://").
	if idx := strings.Index(s, "://"); idx >= 0 {
		s = s[idx+3:]
	}

	// Strip path, query, and port in order to isolate the bare host.
	for _, sep := range []string{"/", "?", ":"} {
		if idx := strings.Index(s, sep); idx >= 0 {
			s = s[:idx]
		}
	}

	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}
	}

	return addr
}

// parseOnegateProxyRoute emits a /32 scope-link host route to the ONEGATE
// endpoint when its host is a link-local IPv4 address (169.254.x.x). The
// route is attached to outLink (the first static interface), matching the
// reference add_onegate_proxy_route behavior.
func parseOnegateProxyRoute(oneContext map[string]string, outLink string, networkConfig *runtime.PlatformNetworkConfig) {
	endpoint := oneContext["ONEGATE_ENDPOINT"]
	if endpoint == "" {
		return
	}

	ip := extractIPv4FromEndpoint(endpoint)
	if !ip.IsValid() || !ip.Is4() || !ip.IsLinkLocalUnicast() {
		return
	}

	route := network.RouteSpecSpec{
		ConfigLayer: network.ConfigPlatform,
		Destination: netip.PrefixFrom(ip, 32),
		OutLinkName: outLink,
		Table:       nethelpers.TableMain,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Family:      nethelpers.FamilyInet4,
		Scope:       nethelpers.ScopeLink,
	}

	route.Normalize()

	networkConfig.Routes = append(networkConfig.Routes, route)
}

// processInterfaces iterates ETHn interfaces in sorted order, configures each
// one, and returns the name of the first static interface link (used to attach
// the ONEGATE proxy route). Sorted order matches the reference behavior of
// env | grep ... | sort (ETH0, ETH1, ETH2, ...).
func processInterfaces(
	oneContext map[string]string,
	networkConfig *runtime.PlatformNetworkConfig,
	allDNSIPs *[]netip.Addr,
	allSearchDomains *[]string,
) (firstStaticLink string, err error) {
	var ifaceNames []string

	for key := range oneContext {
		if ifaceName, ok := ethInterfaceName(key); ok {
			ifaceNames = append(ifaceNames, ifaceName)
		}
	}

	slices.Sort(ifaceNames)

	for _, ifaceName := range ifaceNames {
		if err := parseInterface(oneContext, ifaceName, networkConfig, allDNSIPs, allSearchDomains); err != nil {
			return "", err
		}

		if firstStaticLink == "" {
			method := strings.ToLower(oneContext[ifaceName+"_METHOD"])
			if method == "" || method == "static" {
				firstStaticLink = strings.ToLower(ifaceName)
			}
		}
	}

	return firstStaticLink, nil
}

// ParseMetadata converts opennebula metadata to platform network config.
func (o *OpenNebula) ParseMetadata(st state.State, oneContextPlain []byte) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	oneContext, err := envparse.Parse(bytes.NewReader(oneContextPlain))
	if err != nil {
		return nil, fmt.Errorf("failed to parse context file %q: %w", oneContextPlain, err)
	}

	hostnameValue := resolveHostname(oneContext)

	// Seed the merged DNS/search-domain slices with global variables (DNS,
	// SEARCH_DOMAIN). These are applied regardless of interface, matching the
	// reference get_nameservers()/get_searchdomains() which processes global
	// variables before per-interface ones.
	var allDNSIPs []netip.Addr

	for s := range strings.FieldsSeq(oneContext["DNS"]) {
		ip, err := netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("failed to parse global DNS server %q: %w", s, err)
		}

		allDNSIPs = append(allDNSIPs, ip)
	}

	allSearchDomains := append([]string(nil), strings.Fields(oneContext["SEARCH_DOMAIN"])...)

	firstStaticLink, err := processInterfaces(oneContext, networkConfig, &allDNSIPs, &allSearchDomains)
	if err != nil {
		return nil, err
	}

	if firstStaticLink != "" {
		parseOnegateProxyRoute(oneContext, firstStaticLink, networkConfig)
	}

	if len(allDNSIPs)+len(allSearchDomains) > 0 {
		resolverSpec := network.ResolverSpecSpec{
			NameServers: xslices.Map(allDNSIPs, func(addr netip.Addr) network.NameServerSpec {
				return network.NameServerSpec{
					Addr:     addr,
					Protocol: nethelpers.DNSProtocolDefault,
				}
			}),
			SearchDomains: allSearchDomains,
			ConfigLayer:   network.ConfigPlatform,
		}
		resolverSpec.Convert()

		networkConfig.Resolvers = append(networkConfig.Resolvers, resolverSpec)
	}

	hostnameSpec := network.HostnameSpecSpec{
		ConfigLayer: network.ConfigPlatform,
	}

	if hostnameValue != "" {
		if err := hostnameSpec.ParseFQDN(hostnameValue); err != nil {
			return nil, fmt.Errorf("failed to parse hostname: %w", err)
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:   o.Name(),
		Hostname:   hostnameSpec.Hostname,
		InstanceID: oneContext["VMID"],
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (o *OpenNebula) Configuration(ctx context.Context, r state.State) (machineConfig []byte, err error) {
	oneContextPlain, err := o.contextFromCD(ctx, r)
	if err != nil {
		return nil, err
	}

	oneContext, err := envparse.Parse(bytes.NewReader(oneContextPlain))
	if err != nil {
		return nil, fmt.Errorf("failed to parse environment file %q: %w", oneContextPlain, err)
	}

	userData, ok := oneContext["USER_DATA"]
	if !ok {
		// Legacy fallback: reference does USER_DATA="${USER_DATA:-${USERDATA}}".
		userData, ok = oneContext["USERDATA"]
	}

	if !ok {
		return nil, errors.ErrNoConfigSource
	}

	machineConfig, err = base64.StdEncoding.DecodeString(userData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode USER_DATA: %v", err)
	}

	return machineConfig, nil
}

// Mode implements the runtime.Platform interface.
func (o *OpenNebula) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (o *OpenNebula) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (o *OpenNebula) NetworkConfiguration(ctx context.Context, st state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	oneContext, err := o.contextFromCD(ctx, st)
	if stderrors.Is(err, errors.ErrNoConfigSource) {
		err = nil
	}

	if err != nil {
		return err
	}

	networkConfig, err := o.ParseMetadata(st, oneContext)
	if err != nil {
		return err
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
