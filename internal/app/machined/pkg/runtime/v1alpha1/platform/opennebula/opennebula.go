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

// parseAliases collects ETHn_ALIASm_* address entries for a given interface.
// An alias is skipped when DETACH is non-empty OR EXTERNAL=YES, matching the
// reference netcfg-networkd behavior (lines 395-400).
func parseAliases(oneContext map[string]string, ifaceName, ifaceNameLower string) ([]network.AddressSpecSpec, error) {
	aliasNames := collectAliasNames(oneContext, ifaceName+"_ALIAS")

	var addrs []network.AddressSpecSpec

	for _, aliasName := range aliasNames {
		// Skip detached aliases — reference: [ -z "${detach}" ]
		if oneContext[aliasName+"_DETACH"] != "" {
			continue
		}

		// Skip externally managed aliases — reference: ! is_true "${external}"
		if strings.EqualFold(oneContext[aliasName+"_EXTERNAL"], "yes") {
			continue
		}

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

// ParseMetadata converts opennebula metadata to platform network config.
//
//nolint:gocyclo,cyclop
func (o *OpenNebula) ParseMetadata(st state.State, oneContextPlain []byte) (*runtime.PlatformNetworkConfig, error) {
	// Initialize the PlatformNetworkConfig
	networkConfig := &runtime.PlatformNetworkConfig{}

	oneContext, err := envparse.Parse(bytes.NewReader(oneContextPlain))
	if err != nil {
		return nil, fmt.Errorf("failed to parse context file %q: %w", oneContextPlain, err)
	}

	// Create HostnameSpecSpec entry
	// HOSTNAME is checked first (deviation from the reference which tries
	// SET_HOSTNAME before HOSTNAME) to preserve backward compatibility with
	// existing Talos deployments that rely on the OpenNebula-injected FQDN.
	hostnameValue := oneContext["HOSTNAME"]
	if hostnameValue == "" {
		hostnameValue = oneContext["SET_HOSTNAME"]
		if hostnameValue == "" {
			hostnameValue = oneContext["NAME"]
		}
	}

	hostnameValue = sanitizeHostname(hostnameValue)

	// Seed the merged DNS/search-domain slices with global variables (DNS,
	// SEARCH_DOMAIN). These are applied regardless of interface, matching the
	// reference get_nameservers()/get_searchdomains() which processes global
	// variables before per-interface ones.
	var allDNSIPs []netip.Addr

	var allSearchDomains []string

	for s := range strings.FieldsSeq(oneContext["DNS"]) {
		ip, err := netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("failed to parse global DNS server %q: %w", s, err)
		}

		allDNSIPs = append(allDNSIPs, ip)
	}

	allSearchDomains = append(allSearchDomains, strings.Fields(oneContext["SEARCH_DOMAIN"])...)

	// Iterate through parsed environment variables looking for ETHn_MAC keys.
	// The presence of ETHn_MAC is the sole trigger for interface configuration,
	// matching the behavior of the official OpenNebula guest contextualization
	// scripts (one-apps/context-linux: get_context_interfaces() uses ETH*_MAC
	// presence exclusively). The NETWORK context variable is a server-side
	// directive that tells OpenNebula to auto-inject ETH*_ variables from NIC
	// definitions; it is not a guest-side signal and is never read by the
	// official scripts.
	for key := range oneContext {
		if strings.HasPrefix(key, "ETH") && strings.HasSuffix(key, "_MAC") {
			ifaceName := strings.TrimSuffix(key, "_MAC")
			// Skip alias MAC keys (e.g. ETH0_ALIAS0_MAC); only process
			// top-level interface keys of the form ETH<digits>_MAC,
			// matching the reference get_context_interfaces() regex ETH[0-9]+.
			if !isDigitsOnly(strings.TrimPrefix(ifaceName, "ETH")) {
				continue
			}

			ifaceNameLower := strings.ToLower(ifaceName)

			routeMetric := uint32(network.DefaultRouteMetric)

			if metricStr := oneContext[ifaceName+"_METRIC"]; metricStr != "" {
				m, err := strconv.ParseUint(metricStr, 10, 32)
				if err != nil {
					return nil, fmt.Errorf("interface %s: failed to parse metric: %w", ifaceName, err)
				}

				routeMetric = uint32(m)
			}

			if oneContext[ifaceName+"_METHOD"] == "dhcp" {
				// Create DHCP4 OperatorSpec entry
				networkConfig.Operators = append(networkConfig.Operators,
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
			} else {
				// Parse IP address and create AddressSpecSpec entry
				ipPrefix, err := address.IPPrefixFrom(oneContext[ifaceName+"_IP"], oneContext[ifaceName+"_MASK"])
				if err != nil {
					return nil, fmt.Errorf("failed to parse IP address: %w", err)
				}

				networkConfig.Addresses = append(networkConfig.Addresses,
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

				if oneContext[ifaceName+"_MTU"] == "" {
					mtu = 0
				} else {
					var mtu64 uint64

					mtu64, err = strconv.ParseUint(oneContext[ifaceName+"_MTU"], 10, 32)
					// check if any error happened
					if err != nil {
						return nil, fmt.Errorf("failed to parse MTU: %w", err)
					}

					mtu = uint32(mtu64)
				}

				// Create LinkSpecSpec entry
				networkConfig.Links = append(networkConfig.Links,
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

				if oneContext[ifaceName+"_GATEWAY"] != "" {
					// Parse gateway address and create RouteSpecSpec entry
					gateway, err := netip.ParseAddr(oneContext[ifaceName+"_GATEWAY"])
					if err != nil {
						return nil, fmt.Errorf("failed to parse gateway ip: %w", err)
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
						return nil, fmt.Errorf("interface %s: %w", ifaceName, err)
					}

					networkConfig.Routes = append(networkConfig.Routes, staticRoutes...)
				}

				// Accumulate per-interface DNS servers and search domains into
				// the shared slices (global values were seeded before the loop).
				for s := range strings.FieldsSeq(oneContext[ifaceName+"_DNS"]) {
					ip, err := netip.ParseAddr(s)
					if err != nil {
						return nil, fmt.Errorf("interface %s: failed to parse DNS server %q: %w", ifaceName, s, err)
					}

					allDNSIPs = append(allDNSIPs, ip)
				}

				allSearchDomains = append(allSearchDomains, strings.Fields(oneContext[ifaceName+"_SEARCH_DOMAIN"])...)
			}

			// Process alias addresses for this interface (applies to both
			// static and DHCP interfaces, matching the reference behavior).
			aliasAddrs, err := parseAliases(oneContext, ifaceName, ifaceNameLower)
			if err != nil {
				return nil, err
			}

			networkConfig.Addresses = append(networkConfig.Addresses, aliasAddrs...)
		}
	}
	// Emit a single merged ResolverSpecSpec combining global and per-interface
	// values, matching the reference single /etc/resolv.conf output.
	if len(allDNSIPs) > 0 || len(allSearchDomains) > 0 {
		networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
			DNSServers:    allDNSIPs,
			SearchDomains: allSearchDomains,
			ConfigLayer:   network.ConfigPlatform,
		})
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
