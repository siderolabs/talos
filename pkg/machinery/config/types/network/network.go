// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package network provides network machine configuration documents.
package network

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output network_doc.go network.go bond.go bridge.go default_action_config.go dhcp4.go dhcp6.go dummy.go ethernet.go hcloud_vip.go hostname.go kubespan_endpoints.go layer2_vip.go link.go link_alias.go port_range.go resolver.go rule_config.go static_host.go time_sync.go vlan.go wireguard.go

//go:generate go tool github.com/siderolabs/deep-copy -type BondConfigV1Alpha1 -type BridgeConfigV1Alpha1 -type DefaultActionConfigV1Alpha1 -type DHCPv4ConfigV1Alpha1 -type DHCPv6ConfigV1Alpha1 -type DummyLinkConfigV1Alpha1 -type EthernetConfigV1Alpha1 -type HCloudVIPConfigV1Alpha1 -type HostnameConfigV1Alpha1 -type KubespanEndpointsConfigV1Alpha1 -type Layer2VIPConfigV1Alpha1 -type LinkConfigV1Alpha1 -type LinkAliasConfigV1Alpha1 -type ResolverConfigV1Alpha1 -type RuleConfigV1Alpha1 -type StaticHostConfigV1Alpha1 -type TimeSyncConfigV1Alpha1 -type VLANConfigV1Alpha1 -type WireguardConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
