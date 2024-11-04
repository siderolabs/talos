// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"cmp"
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// IngressChainName is the name of the ingress chain.
const IngressChainName = "ingress"

// NfTablesChainConfigController generates nftables rules based on machine configuration.
type NfTablesChainConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *NfTablesChainConfigController) Name() string {
	return "network.NfTablesChainConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NfTablesChainConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NfTablesChainConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.NfTablesChainType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *NfTablesChainConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) (err error) {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting machine config: %w", err)
		}

		r.StartTrackingOutputs()

		if cfg != nil && !(cfg.Config().NetworkRules().DefaultAction() == nethelpers.DefaultActionAccept && cfg.Config().NetworkRules().Rules() == nil) {
			if err = safe.WriterModify(ctx, r, network.NewNfTablesChain(network.NamespaceName, IngressChainName),
				func(chain *network.NfTablesChain) error {
					spec := chain.TypedSpec()

					spec.Type = nethelpers.ChainTypeFilter
					spec.Hook = nethelpers.ChainHookInput
					spec.Priority = nethelpers.ChainPriorityFilter
					spec.Policy = nethelpers.VerdictAccept

					// preamble
					spec.Rules = []network.NfTablesRule{
						// trusted interfaces: loopback, siderolink and kubespan
						{
							MatchIIfName: &network.NfTablesIfNameMatch{
								InterfaceNames: []string{
									"lo",
									constants.SideroLinkName,
									constants.KubeSpanLinkName,
								},
								Operator: nethelpers.OperatorEqual,
							},
							AnonCounter: true,
							Verdict:     pointer.To(nethelpers.VerdictAccept),
						},
					}

					defaultAction := cfg.Config().NetworkRules().DefaultAction()

					if defaultAction == nethelpers.DefaultActionBlock {
						spec.Policy = nethelpers.VerdictDrop

						spec.Rules = append(spec.Rules,
							// conntrack
							network.NfTablesRule{
								MatchConntrackState: &network.NfTablesConntrackStateMatch{
									States: []nethelpers.ConntrackState{
										nethelpers.ConntrackStateEstablished,
										nethelpers.ConntrackStateRelated,
									},
								},
								AnonCounter: true,
								Verdict:     pointer.To(nethelpers.VerdictAccept),
							},
							network.NfTablesRule{
								MatchConntrackState: &network.NfTablesConntrackStateMatch{
									States: []nethelpers.ConntrackState{
										nethelpers.ConntrackStateInvalid,
									},
								},
								AnonCounter: true,
								Verdict:     pointer.To(nethelpers.VerdictDrop),
							},
							// allow ICMP and ICMPv6 explicitly
							network.NfTablesRule{
								MatchLayer4: &network.NfTablesLayer4Match{
									Protocol: nethelpers.ProtocolICMP,
								},
								MatchLimit: &network.NfTablesLimitMatch{
									PacketRatePerSecond: 5,
								},
								AnonCounter: true,
								Verdict:     pointer.To(nethelpers.VerdictAccept),
							},
							network.NfTablesRule{
								MatchLayer4: &network.NfTablesLayer4Match{
									Protocol: nethelpers.ProtocolICMPv6,
								},
								MatchLimit: &network.NfTablesLimitMatch{
									PacketRatePerSecond: 5,
								},
								AnonCounter: true,
								Verdict:     pointer.To(nethelpers.VerdictAccept),
							},
						)

						if cfg.Config().Machine() != nil && cfg.Config().Cluster() != nil {
							if cfg.Config().Machine().Features().HostDNS().ForwardKubeDNSToHost() {
								hostDNSIP := netip.MustParseAddr(constants.HostDNSAddress)

								// allow traffic to host DNS
								for _, protocol := range []nethelpers.Protocol{nethelpers.ProtocolUDP, nethelpers.ProtocolTCP} {
									spec.Rules = append(spec.Rules,
										network.NfTablesRule{
											MatchSourceAddress: &network.NfTablesAddressMatch{
												IncludeSubnets: xslices.Map(
													slices.Concat(
														cfg.Config().Cluster().Network().PodCIDRs(),
														cfg.Config().Cluster().Network().ServiceCIDRs(),
													),
													netip.MustParsePrefix,
												),
											},
											MatchDestinationAddress: &network.NfTablesAddressMatch{
												IncludeSubnets: []netip.Prefix{netip.PrefixFrom(hostDNSIP, hostDNSIP.BitLen())},
											},
											MatchLayer4: &network.NfTablesLayer4Match{
												Protocol: protocol,
												MatchDestinationPort: &network.NfTablesPortMatch{
													Ranges: []network.PortRange{{Lo: 53, Hi: 53}},
												},
											},
											AnonCounter: true,
											Verdict:     pointer.To(nethelpers.VerdictAccept),
										},
									)
								}
							}
						}

						if cfg.Config().Cluster() != nil {
							spec.Rules = append(spec.Rules,
								// allow Kubernetes pod/service traffic
								network.NfTablesRule{
									MatchSourceAddress: &network.NfTablesAddressMatch{
										IncludeSubnets: xslices.Map(
											append(slices.Clone(cfg.Config().Cluster().Network().PodCIDRs()), cfg.Config().Cluster().Network().ServiceCIDRs()...),
											netip.MustParsePrefix,
										),
									},
									MatchDestinationAddress: &network.NfTablesAddressMatch{
										IncludeSubnets: xslices.Map(
											append(slices.Clone(cfg.Config().Cluster().Network().PodCIDRs()), cfg.Config().Cluster().Network().ServiceCIDRs()...),
											netip.MustParsePrefix,
										),
									},
									AnonCounter: true,
									Verdict:     pointer.To(nethelpers.VerdictAccept),
								},
							)
						}
					}

					for _, rule := range cfg.Config().NetworkRules().Rules() {
						portRanges := rule.PortRanges()

						// sort port ranges, machine config validation ensures that there are no overlaps
						slices.SortFunc(portRanges, func(a, b [2]uint16) int {
							return cmp.Compare(a[0], b[0])
						})

						// if default accept, drop anything that doesn't match the rule
						verdict := nethelpers.VerdictDrop

						if defaultAction == nethelpers.DefaultActionBlock {
							verdict = nethelpers.VerdictAccept
						}

						spec.Rules = append(spec.Rules,
							network.NfTablesRule{
								MatchSourceAddress: &network.NfTablesAddressMatch{
									IncludeSubnets: rule.Subnets(),
									ExcludeSubnets: rule.ExceptSubnets(),
									Invert:         defaultAction == nethelpers.DefaultActionAccept,
								},
								MatchLayer4: &network.NfTablesLayer4Match{
									Protocol: rule.Protocol(),
									MatchDestinationPort: &network.NfTablesPortMatch{
										Ranges: xslices.Map(portRanges, func(pr [2]uint16) network.PortRange {
											return network.PortRange{Lo: pr[0], Hi: pr[1]}
										}),
									},
								},
								AnonCounter: true,
								Verdict:     pointer.To(verdict),
							},
						)
					}

					return nil
				}); err != nil {
				return err
			}
		}

		if err = safe.CleanupOutputs[*network.NfTablesChain](ctx, r); err != nil {
			return err
		}
	}
}
