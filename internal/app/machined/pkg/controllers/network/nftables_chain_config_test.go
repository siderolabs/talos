// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	configtypes "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type NfTablesChainConfigTestSuite struct {
	ctest.DefaultSuite
}

func (suite *NfTablesChainConfigTestSuite) injectConfig(block bool) {
	kubeletIngressCfg := networkcfg.NewRuleConfigV1Alpha1()
	kubeletIngressCfg.MetaName = "kubelet-ingress"
	kubeletIngressCfg.PortSelector.Ports = []networkcfg.PortRange{
		{
			Lo: 10250,
			Hi: 10250,
		},
	}
	kubeletIngressCfg.PortSelector.Protocol = nethelpers.ProtocolTCP
	kubeletIngressCfg.Ingress = []networkcfg.IngressRule{
		{
			Subnet: netip.MustParsePrefix("10.0.0.0/8"),
			Except: networkcfg.Prefix{Prefix: netip.MustParsePrefix("10.3.0.0/16")},
		},
		{
			Subnet: netip.MustParsePrefix("192.168.0.0/16"),
		},
	}

	apidIngressCfg := networkcfg.NewRuleConfigV1Alpha1()
	apidIngressCfg.MetaName = "apid-ingress"
	apidIngressCfg.PortSelector.Ports = []networkcfg.PortRange{
		{
			Lo: 50000,
			Hi: 50000,
		},
	}
	apidIngressCfg.PortSelector.Protocol = nethelpers.ProtocolTCP
	apidIngressCfg.Ingress = []networkcfg.IngressRule{
		{
			Subnet: netip.MustParsePrefix("0.0.0.0/0"),
		},
	}

	configs := []configtypes.Document{kubeletIngressCfg, apidIngressCfg}

	if block {
		defaultActionCfg := networkcfg.NewDefaultActionConfigV1Alpha1()
		defaultActionCfg.Ingress = nethelpers.DefaultActionBlock

		configs = append(configs, defaultActionCfg)
	}

	cfg, err := container.New(configs...)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(cfg)))
}

func (suite *NfTablesChainConfigTestSuite) TestDefaultAccept() {
	ctest.AssertNoResource[*network.NfTablesChain](suite, netctrl.IngressChainName)

	suite.injectConfig(false)

	ctest.AssertResource(suite, netctrl.IngressChainName, func(chain *network.NfTablesChain, asrt *assert.Assertions) {
		spec := chain.TypedSpec()

		asrt.Equal(nethelpers.ChainTypeFilter, spec.Type)
		asrt.Equal(nethelpers.ChainPriorityMangle+10, spec.Priority)
		asrt.Equal(nethelpers.ChainHookInput, spec.Hook)
		asrt.Equal(nethelpers.VerdictAccept, spec.Policy)

		asrt.Equal(
			[]network.NfTablesRule{
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
				{
					MatchSourceAddress: &network.NfTablesAddressMatch{
						IncludeSubnets: []netip.Prefix{
							netip.MustParsePrefix("10.0.0.0/8"),
							netip.MustParsePrefix("192.168.0.0/16"),
						},
						ExcludeSubnets: []netip.Prefix{
							netip.MustParsePrefix("10.3.0.0/16"),
						},
						Invert: true,
					},
					MatchLayer4: &network.NfTablesLayer4Match{
						Protocol: nethelpers.ProtocolTCP,
						MatchDestinationPort: &network.NfTablesPortMatch{
							Ranges: []network.PortRange{
								{
									Lo: 10250,
									Hi: 10250,
								},
							},
						},
					},
					AnonCounter: true,
					Verdict:     pointer.To(nethelpers.VerdictDrop),
				},
				{
					MatchSourceAddress: &network.NfTablesAddressMatch{
						IncludeSubnets: []netip.Prefix{
							netip.MustParsePrefix("0.0.0.0/0"),
						},
						Invert: true,
					},
					MatchLayer4: &network.NfTablesLayer4Match{
						Protocol: nethelpers.ProtocolTCP,
						MatchDestinationPort: &network.NfTablesPortMatch{
							Ranges: []network.PortRange{
								{
									Lo: 50000,
									Hi: 50000,
								},
							},
						},
					},
					AnonCounter: true,
					Verdict:     pointer.To(nethelpers.VerdictDrop),
				},
			},
			spec.Rules)
	})
}

func (suite *NfTablesChainConfigTestSuite) TestDefaultBlock() {
	ctest.AssertNoResource[*network.NfTablesChain](suite, netctrl.IngressChainName)

	suite.injectConfig(true)

	ctest.AssertResource(suite, netctrl.IngressChainName, func(chain *network.NfTablesChain, asrt *assert.Assertions) {
		spec := chain.TypedSpec()

		asrt.Equal(nethelpers.ChainTypeFilter, spec.Type)
		asrt.Equal(nethelpers.ChainPriorityMangle+10, spec.Priority)
		asrt.Equal(nethelpers.ChainHookInput, spec.Hook)
		asrt.Equal(nethelpers.VerdictDrop, spec.Policy)

		asrt.Equal(
			[]network.NfTablesRule{
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
				{
					MatchConntrackState: &network.NfTablesConntrackStateMatch{
						States: []nethelpers.ConntrackState{
							nethelpers.ConntrackStateEstablished,
							nethelpers.ConntrackStateRelated,
						},
					},
					AnonCounter: true,
					Verdict:     pointer.To(nethelpers.VerdictAccept),
				},
				{
					MatchConntrackState: &network.NfTablesConntrackStateMatch{
						States: []nethelpers.ConntrackState{
							nethelpers.ConntrackStateInvalid,
						},
					},
					AnonCounter: true,
					Verdict:     pointer.To(nethelpers.VerdictDrop),
				},
				{
					MatchLayer4: &network.NfTablesLayer4Match{
						Protocol: nethelpers.ProtocolICMP,
					},
					MatchLimit: &network.NfTablesLimitMatch{
						PacketRatePerSecond: 5,
					},
					AnonCounter: true,
					Verdict:     pointer.To(nethelpers.VerdictAccept),
				},
				{
					MatchLayer4: &network.NfTablesLayer4Match{
						Protocol: nethelpers.ProtocolICMPv6,
					},
					MatchLimit: &network.NfTablesLimitMatch{
						PacketRatePerSecond: 5,
					},
					AnonCounter: true,
					Verdict:     pointer.To(nethelpers.VerdictAccept),
				},
				{
					MatchSourceAddress: &network.NfTablesAddressMatch{
						IncludeSubnets: []netip.Prefix{
							netip.MustParsePrefix("10.0.0.0/8"),
							netip.MustParsePrefix("192.168.0.0/16"),
						},
						ExcludeSubnets: []netip.Prefix{
							netip.MustParsePrefix("10.3.0.0/16"),
						},
					},
					MatchLayer4: &network.NfTablesLayer4Match{
						Protocol: nethelpers.ProtocolTCP,
						MatchDestinationPort: &network.NfTablesPortMatch{
							Ranges: []network.PortRange{
								{
									Lo: 10250,
									Hi: 10250,
								},
							},
						},
					},
					AnonCounter: true,
					Verdict:     pointer.To(nethelpers.VerdictAccept),
				},
				{
					MatchSourceAddress: &network.NfTablesAddressMatch{
						IncludeSubnets: []netip.Prefix{
							netip.MustParsePrefix("0.0.0.0/0"),
						},
					},
					MatchLayer4: &network.NfTablesLayer4Match{
						Protocol: nethelpers.ProtocolTCP,
						MatchDestinationPort: &network.NfTablesPortMatch{
							Ranges: []network.PortRange{
								{
									Lo: 50000,
									Hi: 50000,
								},
							},
						},
					},
					AnonCounter: true,
					Verdict:     pointer.To(nethelpers.VerdictAccept),
				},
			},
			spec.Rules)
	})
}

func TestNfTablesChainConfig(t *testing.T) {
	suite.Run(t, &NfTablesChainConfigTestSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&netctrl.NfTablesChainConfigController{}))
			},
		},
	})
}
