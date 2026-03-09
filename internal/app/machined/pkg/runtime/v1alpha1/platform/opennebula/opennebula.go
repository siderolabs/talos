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

// ParseMetadata converts opennebula metadata to platform network config.
//
//nolint:gocyclo
func (o *OpenNebula) ParseMetadata(st state.State, oneContextPlain []byte) (*runtime.PlatformNetworkConfig, error) {
	// Initialize the PlatformNetworkConfig
	networkConfig := &runtime.PlatformNetworkConfig{}

	oneContext, err := envparse.Parse(bytes.NewReader(oneContextPlain))
	if err != nil {
		return nil, fmt.Errorf("failed to parse context file %q: %w", oneContextPlain, err)
	}

	// Create HostnameSpecSpec entry
	hostnameValue := oneContext["HOSTNAME"]
	if hostnameValue == "" {
		hostnameValue = oneContext["SET_HOSTNAME"]
		if hostnameValue == "" {
			hostnameValue = oneContext["NAME"]
		}
	}

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
			ifaceNameLower := strings.ToLower(ifaceName)

			if oneContext[ifaceName+"_METHOD"] == "dhcp" {
				// Create DHCP4 OperatorSpec entry
				networkConfig.Operators = append(networkConfig.Operators,
					network.OperatorSpecSpec{
						Operator:  network.OperatorDHCP4,
						LinkName:  ifaceNameLower,
						RequireUp: true,
						DHCP4: network.DHCP4OperatorSpec{
							RouteMetric:         1024,
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
						Priority:    network.DefaultRouteMetric,
					}

					route.Normalize()

					networkConfig.Routes = append(networkConfig.Routes, route)
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

	// Create HostnameSpecSpec entry
	networkConfig.Hostnames = append(networkConfig.Hostnames,
		network.HostnameSpecSpec{
			Hostname:    hostnameValue,
			Domainname:  oneContext["DNS_HOSTNAME"],
			ConfigLayer: network.ConfigPlatform,
		},
	)

	// Create Metadata entry
	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:   o.Name(),
		Hostname:   hostnameValue,
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
