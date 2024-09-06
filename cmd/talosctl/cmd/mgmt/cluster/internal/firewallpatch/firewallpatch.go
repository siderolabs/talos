// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package firewallpatch provides a set of default config patches to enable firewall.
package firewallpatch

import (
	"net/netip"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

func ingressRuleWithinCluster(cidrs []netip.Prefix, gateways []netip.Addr) []network.IngressRule {
	rules := make([]network.IngressRule, 0, len(cidrs))

	for i := range cidrs {
		rules = append(rules,
			network.IngressRule{
				Subnet: cidrs[i],
				Except: network.Prefix{Prefix: netip.PrefixFrom(gateways[i], gateways[i].BitLen())},
			},
		)
	}

	return rules
}

func ingressRuleWideOpen() []network.IngressRule {
	return []network.IngressRule{
		{
			Subnet: netip.MustParsePrefix("0.0.0.0/0"),
		},
		{
			Subnet: netip.MustParsePrefix("::/0"),
		},
	}
}

func ingressOnly(ips []netip.Addr) []network.IngressRule {
	return xslices.Map(ips, func(ip netip.Addr) network.IngressRule {
		return network.IngressRule{
			Subnet: netip.PrefixFrom(ip, ip.BitLen()),
		}
	})
}

// ControlPlane generates a default firewall for a controlplane node.
//
// Kubelet and Trustd are only available within the cluster.
// Apid & Kubernetes API is wide open.
// Etcd is only available within the controlplanes.
func ControlPlane(defaultAction nethelpers.DefaultAction, cidrs []netip.Prefix, gateways []netip.Addr, controlplanes []netip.Addr) configpatcher.Patch {
	def := network.NewDefaultActionConfigV1Alpha1()
	def.Ingress = defaultAction

	kubeletRule := network.NewRuleConfigV1Alpha1()
	kubeletRule.MetaName = "kubelet-ingress"
	kubeletRule.PortSelector.Ports = []network.PortRange{
		{
			Lo: constants.KubeletPort,
			Hi: constants.KubeletPort,
		},
	}
	kubeletRule.PortSelector.Protocol = nethelpers.ProtocolTCP
	kubeletRule.Ingress = ingressRuleWithinCluster(cidrs, gateways)

	apidRule := network.NewRuleConfigV1Alpha1()
	apidRule.MetaName = "apid-ingress"
	apidRule.PortSelector.Ports = []network.PortRange{
		{
			Lo: constants.ApidPort,
			Hi: constants.ApidPort,
		},
	}
	apidRule.PortSelector.Protocol = nethelpers.ProtocolTCP
	apidRule.Ingress = ingressRuleWideOpen()

	trustdRule := network.NewRuleConfigV1Alpha1()
	trustdRule.MetaName = "trustd-ingress"
	trustdRule.PortSelector.Ports = []network.PortRange{
		{
			Lo: constants.TrustdPort,
			Hi: constants.TrustdPort,
		},
	}
	trustdRule.PortSelector.Protocol = nethelpers.ProtocolTCP
	trustdRule.Ingress = ingressRuleWithinCluster(cidrs, gateways)

	kubeAPIRule := network.NewRuleConfigV1Alpha1()
	kubeAPIRule.MetaName = "kubernetes-api-ingress"
	kubeAPIRule.PortSelector.Ports = []network.PortRange{
		{
			Lo: constants.DefaultControlPlanePort,
			Hi: constants.DefaultControlPlanePort,
		},
	}
	kubeAPIRule.PortSelector.Protocol = nethelpers.ProtocolTCP
	kubeAPIRule.Ingress = ingressRuleWideOpen()

	etcdRule := network.NewRuleConfigV1Alpha1()
	etcdRule.MetaName = "etcd-ingress"
	etcdRule.PortSelector.Ports = []network.PortRange{
		{
			Lo: constants.EtcdClientPort,
			Hi: constants.EtcdPeerPort,
		},
	}
	etcdRule.PortSelector.Protocol = nethelpers.ProtocolTCP
	etcdRule.Ingress = ingressOnly(controlplanes)

	vxlanRule := network.NewRuleConfigV1Alpha1()
	vxlanRule.MetaName = "cni-vxlan"
	vxlanRule.PortSelector.Ports = []network.PortRange{
		{
			Lo: 4789, // Flannel, Calico VXLAN
			Hi: 4789,
		},
		{
			Lo: 8472, // Cilium VXLAN
			Hi: 8472,
		},
	}
	vxlanRule.PortSelector.Protocol = nethelpers.ProtocolUDP
	vxlanRule.Ingress = ingressRuleWithinCluster(cidrs, gateways)

	provider, err := container.New(def, kubeletRule, apidRule, trustdRule, kubeAPIRule, etcdRule, vxlanRule)
	if err != nil { // should not fail
		panic(err)
	}

	return configpatcher.NewStrategicMergePatch(provider)
}

// Worker generates a default firewall for a worker node.
//
// Kubelet & apid are only available within the cluster.
func Worker(defaultAction nethelpers.DefaultAction, cidrs []netip.Prefix, gateways []netip.Addr) configpatcher.Patch {
	def := network.NewDefaultActionConfigV1Alpha1()
	def.Ingress = defaultAction

	kubeletRule := network.NewRuleConfigV1Alpha1()
	kubeletRule.MetaName = "kubelet-ingress"
	kubeletRule.PortSelector.Ports = []network.PortRange{
		{
			Lo: constants.KubeletPort,
			Hi: constants.KubeletPort,
		},
	}
	kubeletRule.PortSelector.Protocol = nethelpers.ProtocolTCP
	kubeletRule.Ingress = ingressRuleWithinCluster(cidrs, gateways)

	apidRule := network.NewRuleConfigV1Alpha1()
	apidRule.MetaName = "apid-ingress"
	apidRule.PortSelector.Ports = []network.PortRange{
		{
			Lo: constants.ApidPort,
			Hi: constants.ApidPort,
		},
	}
	apidRule.PortSelector.Protocol = nethelpers.ProtocolTCP
	apidRule.Ingress = ingressRuleWithinCluster(cidrs, gateways)

	vxlanRule := network.NewRuleConfigV1Alpha1()
	vxlanRule.MetaName = "cni-vxlan"
	vxlanRule.PortSelector.Ports = []network.PortRange{
		{
			Lo: 4789, // Flannel, Calico VXLAN
			Hi: 4789,
		},
		{
			Lo: 8472, // Cilium VXLAN
			Hi: 8472,
		},
	}
	vxlanRule.PortSelector.Protocol = nethelpers.ProtocolUDP
	vxlanRule.Ingress = ingressRuleWithinCluster(cidrs, gateways)

	provider, err := container.New(def, kubeletRule, apidRule, vxlanRule)
	if err != nil { // should not fail
		panic(err)
	}

	return configpatcher.NewStrategicMergePatch(provider)
}
