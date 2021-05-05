// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package wglan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/wglan-manager/client"
	"github.com/talos-systems/wglan-manager/types"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/cri-api/pkg/errors"

	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var zeroKey = wgtypes.Key{}

// Discoverer defines an interface by which Nodes may be discovered.
type Discoverer interface {
	// Add registers information about the Node
	Add(ctx context.Context, clusterID string, n *types.Node) error

	List(ctx context.Context, clusterID string) ([]*PrePeer, error)
}

type externalDiscoverer struct {
	urlRoot string
}

func (d *externalDiscoverer) Add(ctx context.Context, clusterID string, n *types.Node) error {
	return client.Add(d.urlRoot, clusterID, n)
}

func (d *externalDiscoverer) List(ctx context.Context, clusterID string) ([]*PrePeer, error) {
	list, err := client.List(d.urlRoot, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of nodes from discovery service %q, cluster %q: %w", d.urlRoot, clusterID, err)
	}

	if len(list) < 1 {
		return nil, fmt.Errorf("no peers available")
	}

	var ret []*PrePeer //nolint: prealloc

	for _, n := range list {
		if n.ID == zeroKey {
			return nil, fmt.Errorf("empty key received from discovery service %q, cluster %q", d.urlRoot, clusterID)
		}

		if n.IP.IsZero() {
			continue
		}

		ret = append(ret, &PrePeer{
			IP:         n.IP,
			NodeIPSets: d.populateNodeIPSets(n),
			PublicKey:  n.ID,
		})
	}

	return ret, nil
}

func (d *externalDiscoverer) populateNodeIPSets(n *types.Node) (set *NodeIPSets) {
	set = new(NodeIPSets)

	if n == nil || n.ID == zeroKey {
		return set
	}

	if len(n.SelfIPs) > 0 {
		mergeIPSets(set.SelfIPs, n.SelfIPs)
	}

	for _, ep := range n.KnownEndpoints {
		set.KnownEndpoints = append(set.KnownEndpoints, ep.Endpoint)
	}

	return set
}

type kubeDiscoverer struct {
	includePodSubnets bool
}

func (d *kubeDiscoverer) secretName(nodeName string) string {
	return fmt.Sprintf("%s-wglan-node", nodeName)
}

//nolint: gocyclo,cyclop
func (d *kubeDiscoverer) Add(ctx context.Context, clusterID string, n *types.Node) (err error) {
	var (
		changed bool
		kc      *kubernetes.Client
		node    *v1.Node
	)

	kc, err = kubernetes.NewClientFromKubeletKubeconfig()
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	node, err = kc.CoreV1().Nodes().Get(ctx, n.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// node does not exist yet
			return nil
		}

		return fmt.Errorf("failed to get node %q: %w", n.Name, err)
	}

	oldData, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal existing node data: %w", err)
	}

	// Set public key
	if n.ID != zeroKey {
		var existingKey wgtypes.Key

		existingKey, err = keyFromNode(*node)
		if err != nil {
			return fmt.Errorf("failed to parse key from node %q: %w", node.Name, err)
		}

		if existingKey == zeroKey || existingKey != n.ID {
			changed = true
		}

		node.Annotations[constants.WireguardPublicKeyAnnotation] = n.ID.String()
	}

	// Set wireguard IP
	if !n.IP.IsZero() {
		var existing netaddr.IP

		existing, err = ipFromNode(*node)
		if err != nil {
			return fmt.Errorf("failed to parse IP from node %q: %w", node.Name, err)
		}

		if existing.IsZero() || existing != n.IP {
			changed = true
		}

		node.Annotations[constants.WireguardIPAnnotation] = n.IP.String()
	}

	if len(n.SelfIPs) > 0 {
		var existingIPs []netaddr.IP

		existingIPs, err = ipsFromSelfIPs(*node)
		if err != nil {
			return fmt.Errorf("failed to parse self IPs from node %q: %w", node.Name, err)
		}

		var selfIPChanged bool
		if existingIPs, selfIPChanged = mergeIPSets(existingIPs, n.SelfIPs); selfIPChanged {
			changed = true
		}

		node.Annotations[constants.NetworkSelfIPsAnnotation] = ipsToListString(existingIPs)
	}

	if !changed {
		return nil
	}

	newData, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal new data for node %q: %w", node.Name, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1.Node{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch: %w", err)
	}

	if _, err := kc.CoreV1().Nodes().Patch(ctx, n.Name, ktypes.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		if apierrors.IsConflict(err) {
			return fmt.Errorf("unable to update node %q due to conflict: %w", d.secretName(n.Name), err)
		}

		return fmt.Errorf("error patching node %q: %w", n.Name, err)
	}

	return nil
}

func (d *kubeDiscoverer) List(ctx context.Context, clusterID string) ([]*PrePeer, error) {
	// See if we can yet construct a kubernetes client
	kc, err := kubernetes.NewClientFromKubeletKubeconfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	resp, err := kc.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get list of nodes: %w", err)
	}

	var list []*PrePeer //nolint: prealloc

	for _, n := range resp.Items {
		p := new(PrePeer)

		p.PublicKey, err = keyFromNode(n)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key from node %s: %w", n.Name, err)
		}

		if p.PublicKey == zeroKey {
			continue
		}

		p.IP, err = ipFromNode(n)
		if err != nil {
			return nil, fmt.Errorf("failed to parse wireguard IP from node %s: %w", n.Name, err)
		}

		p.NodeIPSets, err = populateNodeIPSets(n, d.includePodSubnets)
		if err != nil {
			return nil, fmt.Errorf("failed to populate node IP sets from node %s: %w", n.Name, err)
		}

		list = append(list, p)
	}

	return list, nil
}

func ipsToListString(ips []netaddr.IP) string {
	out := make([]string, 0, len(ips))

	for _, ip := range ips {
		out = append(out, ip.String())
	}

	return strings.Join(out, ",")
}

func ipsFromSelfIPs(n v1.Node) (out []netaddr.IP, err error) {
	var merr *multierror.Error

	if data, ok := n.Annotations[constants.NetworkSelfIPsAnnotation]; ok {
		for _, ipString := range strings.Split(data, ",") {
			ip, err := netaddr.ParseIP(strings.TrimSpace(ipString))
			if err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to parse existing IP (%s) from node %q: %w", ipString, n.Name, err))

				continue
			}

			if !ip.IsZero() {
				out = append(out, ip)
			}
		}
	}

	var found bool

	// Also add IPs from status.addresses
	for _, a := range n.Status.Addresses {
		found = false

		ip, err := netaddr.ParseIP(a.Address)
		if err != nil {
			continue // not all addresses will be IPs
		}

		for _, existing := range out {
			if ip == existing {
				found = true

				break
			}
		}

		if !found {
			out = append(out, ip)
		}
	}

	return out, merr.ErrorOrNil()
}

func knownEndpointsFromNode(n v1.Node) (out []netaddr.IPPort, err error) {
	var merr *multierror.Error

	if data, ok := n.Annotations[constants.WireguardKnownEndpointsAnnotation]; ok {
		var found bool

		for _, ipString := range strings.Split(data, ",") {
			ip, err := netaddr.ParseIPPort(strings.TrimSpace(ipString))
			if err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to parse known endpoint (%s) from node %q: %w", ipString, n.Name, err))

				continue
			}

			found = false

			for _, existing := range out {
				if ip == existing {
					found = true

					break
				}
			}

			if !found {
				out = append(out, ip)
			}
		}
	}

	return out, merr.ErrorOrNil()
}

func keyFromNode(n v1.Node) (key wgtypes.Key, err error) {
	keyString, ok := n.Annotations[constants.WireguardPublicKeyAnnotation]
	if !ok {
		return key, nil
	}

	return wgtypes.ParseKey(keyString)
}

func ipFromNode(n v1.Node) (ip netaddr.IP, err error) {
	data, ok := n.Annotations[constants.WireguardIPAnnotation]
	if !ok {
		return ip, nil
	}

	return netaddr.ParseIP(data)
}

//nolint: gocyclo
func populateNodeIPSets(n v1.Node, includePodSubnets bool) (set *NodeIPSets, err error) {
	set = new(NodeIPSets)

	assignedSetBuilder := &netaddr.IPSetBuilder{}

	selfIPs, err := ipsFromSelfIPs(n)
	if err != nil {
		return set, fmt.Errorf("failed to parse node %q self-IPs: %w", n.Name, err)
	}

	if prefixes, ok := n.Annotations[constants.WireguardAssignedPrefixesAnnotation]; ok {
		for _, prefixString := range strings.Split(prefixes, ",") {
			var ip netaddr.IPPrefix

			ip, err = netaddr.ParseIPPrefix(strings.TrimSpace(prefixString))
			if err != nil {
				continue
			}

			assignedSetBuilder.AddPrefix(ip)
		}
	}

	if includePodSubnets {
		var ip netaddr.IPPrefix

		ip, err = netaddr.ParseIPPrefix(n.Spec.PodCIDR)
		if err == nil {
			assignedSetBuilder.AddPrefix(ip)
		}

		for _, podSubnet := range n.Spec.PodCIDRs {
			var podPrefix netaddr.IPPrefix

			podPrefix, err = netaddr.ParseIPPrefix(podSubnet)
			if err == nil {
				assignedSetBuilder.AddPrefix(podPrefix)
			}
		}
	}

	knownEndpoints, err := knownEndpointsFromNode(n)
	if err != nil {
		return set, fmt.Errorf("failed to parse known endpoints from node %q: %w", n.Name, err)
	}

	set.SelfIPs = selfIPs

	set.AssignedPrefixes, err = assignedSetBuilder.IPSet()
	if err != nil {
		return set, fmt.Errorf("failed to compile assigned IP set: %w", err)
	}

	set.KnownEndpoints = knownEndpoints

	return set, nil
}

/*
type staticDiscoverer struct {
	peers []wgtypes.PeerConfig
}

func (d *staticDiscoverer) Add(ctx context.Context, clusterID string, n *types.Node) error {
	// we are static; no adds allowed
	return nil
}

func (d *staticDiscoverer) List(ctx context.Context, clusterID string) ([]*PrePeer, error) {
	var list []*PrePeer

	for _, p := range d.peers {
		list = append(list, &PrePeer{
			PublicKey: p.PublicKey,
		})
	}

	return nil, fmt.Errorf("TODO")
}

func (d *staticDiscoverer) populateNodeIPSets(p *wgtypes.PeerConfig) (set *NodeIPSets) {
	set = new(NodeIPSets)

	if p == nil {
		return
	}

	epList := []netaddr.IPPort{}
	autoSetBuilder := &netaddr.IPSetBuilder{}
	assignedSetBuilder := &netaddr.IPSetBuilder{}

	ep, err := netaddr.ParseIPPort(p.Endpoint.String())
	if err == nil {
		epList = append(epList, ep)
	}

	for _, refIP := range p.AllowedIPs {
		ip, err := netaddr.ParseIPPrefix(refIP.String())
		if err == nil {
			assignedSetBuilder.AddPrefix(ip)
		}
	}

	set.EndpointCandidates = epList
	set.AcceptPrefixes = autoSetBuilder.IPSet() // NB: not used for static sets
	set.AssignedPrefixes = assignedSetBuilder.IPSet()

	return set
}
*/
