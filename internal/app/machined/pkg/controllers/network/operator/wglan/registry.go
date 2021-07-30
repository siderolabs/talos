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

// Registry defines an interface by which Nodes may be discovered and registered.
type Registry interface {
	// Add registers information about the Node to the registry.
	Add(ctx context.Context, clusterID string, n *types.Node) error

	// List returns the list of Nodes stored within the registry.
	List(ctx context.Context, clusterID string) ([]*Peer, error)

	// Name indicates the name of the Registry.
	Name() string
}

// RegistryExternal defines an external API-based node regstry.
type RegistryExternal struct {
	URLRoot string
}

// Add implements registry.Add.
func (r *RegistryExternal) Add(ctx context.Context, clusterID string, n *types.Node) error {
	return client.Add(r.URLRoot, clusterID, n)
}

// List implements registry.List.
func (r *RegistryExternal) List(ctx context.Context, clusterID string) ([]*Peer, error) {
	list, err := client.List(r.URLRoot, clusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get list of nodes from discovery service %q, cluster %q: %w", r.URLRoot, clusterID, err)
	}

	if len(list) < 1 {
		return nil, fmt.Errorf("no peers available")
	}

	var ret []*Peer //nolint: prealloc

	for _, n := range list {
		if n.ID == "" {
			return nil, fmt.Errorf("empty key received from discovery service %q, cluster %q", r.URLRoot, clusterID)
		}

		if n.IP.IsZero() {
			continue
		}

		ret = append(ret, &Peer{
			node: n,
		})
	}

	return ret, nil
}

// Name implements registry.Name.
func (r *RegistryExternal) Name() string {
return "external"
}

// RegistryKubernetes defines a Kubernetes-based node discoverer.
type RegistryKubernetes struct {
	IncludePodSubnets bool
}

func (r *RegistryKubernetes) secretName(nodeName string) string {
	return fmt.Sprintf("%s-wglan-node", nodeName)
}

// Add implements registry.Add.
//nolint: gocyclo
func (r *RegistryKubernetes) Add(ctx context.Context, clusterID string, n *types.Node) (err error) {
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
	if n.ID != "" {
		existingKey := keyFromNode(*node)

		if existingKey == "" || existingKey != n.ID {
			changed = true
		}

		node.Annotations[constants.WireguardPublicKeyAnnotation] = n.ID
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

	if len(n.Addresses) > 0 {
		var existingAddresses []*types.Address

		before := len(n.Addresses)

		existingAddresses, err = addressesFromKubernetesNode(*node)
		if err != nil {
			return fmt.Errorf("failed to parse self IPs from node %q: %w", node.Name, err)
		}

		n.AddAddresses(existingAddresses...)

		if len(n.Addresses) != before {
			changed = true
		}

		node.Annotations[constants.NetworkSelfIPsAnnotation] = addressesToIPListString(n.Addresses)
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
			return fmt.Errorf("unable to update node %q due to conflict: %w", r.secretName(n.Name), err)
		}

		return fmt.Errorf("error patching node %q: %w", n.Name, err)
	}

	return nil
}

// List implements registry.List.
func (r *RegistryKubernetes) List(ctx context.Context, clusterID string) ([]*Peer, error) {
	// See if we can yet construct a kubernetes client
	kc, err := kubernetes.NewClientFromKubeletKubeconfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	resp, err := kc.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get list of nodes: %w", err)
	}

	var list []*Peer //nolint: prealloc

	for _, n := range resp.Items {
		node := new(types.Node)

		if node.ID = keyFromNode(n); node.ID == "" {
			continue
		}

		node.Name = n.Name

		node.IP, err = ipFromNode(n)
		if err != nil {
			return nil, fmt.Errorf("failed to parse wireguard IP from node %s: %w", n.Name, err)
		}

		node.Addresses, err = addressesFromKubernetesNode(n)
		if err != nil {
			return nil, fmt.Errorf("failed to populate node IP sets from node %s: %w", n.Name, err)
		}

		assignedPrefixes, err := assignedPrefixesFromKubernetesNode(n)
		if err != nil {
			return nil, fmt.Errorf("failed to construct assigned prefix list from node %s: %w", n.Name, err)
		}

		p := &Peer{
			node:             node,
			assignedPrefixes: assignedPrefixes,
		}

		list = append(list, p)
	}

	return list, nil
}

// Name implements registry.Name.
func (r *RegistryKubernetes) Name() string {
	return "kubernetes"
}

func addressesToIPListString(addresses []*types.Address) string {
	out := make([]string, 0, len(addresses))

	for _, a := range addresses {
		if !a.IP.IsZero() {
			out = append(out, a.IP.String())
		}
	}

	return strings.Join(out, ",")
}

func addressesFromKubernetesNode(n v1.Node) (out []*types.Address, err error) {
	var merr *multierror.Error

	if data, ok := n.Annotations[constants.NetworkSelfIPsAnnotation]; ok {
		for _, ipString := range strings.Split(data, ",") {
			ip, err := netaddr.ParseIP(strings.TrimSpace(ipString))
			if err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to parse existing IP (%s) from node %q: %w", ipString, n.Name, err))

				continue
			}

			if !ip.IsZero() {
				out = append(out, &types.Address{IP: ip})
			}
		}
	}

	// Also add IPs from status.addresses
	for _, a := range n.Status.Addresses {
		ip, err := netaddr.ParseIP(a.Address)
		if err != nil {
			out = append(out, &types.Address{Name: a.Address})

			continue
		}

		out = append(out, &types.Address{IP: ip})
	}

	return out, merr.ErrorOrNil()
}

func keyFromNode(n v1.Node) string {
	keyString, ok := n.Annotations[constants.WireguardPublicKeyAnnotation]
	if !ok {
		return ""
	}

	return keyString
}

func ipFromNode(n v1.Node) (ip netaddr.IP, err error) {
	data, ok := n.Annotations[constants.WireguardIPAnnotation]
	if !ok {
		return ip, nil
	}

	return netaddr.ParseIP(data)
}

func assignedPrefixesFromKubernetesNode(n v1.Node) (*netaddr.IPSet, error) {
	set := new(netaddr.IPSetBuilder)

	if prefixes, ok := n.Annotations[constants.WireguardAssignedPrefixesAnnotation]; ok {
		for _, prefixString := range strings.Split(prefixes, ",") {
			ip, err := netaddr.ParseIPPrefix(strings.TrimSpace(prefixString))
			if err != nil {
				continue
			}

			set.AddPrefix(ip)
		}
	}

	return set.IPSet()
}

/*
type staticDiscoverer struct {
	peers []wgtypes.PeerConfig
}

func (d *staticDiscoverer) Add(ctx context.Context, clusterID string, n *types.Node) error {
	// we are static; no adds allowed
	return nil
}

func (d *staticDiscoverer) List(ctx context.Context, clusterID string) ([]*Peer, error) {
	var list []*Peer

	for _, p := range d.peers {
		list = append(list, &Peer{
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
