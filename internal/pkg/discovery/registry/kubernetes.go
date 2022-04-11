// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"inet.af/netaddr"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/informers"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
)

// Kubernetes defines a Kubernetes-based node discoverer.
type Kubernetes struct {
	client *kubernetes.Client

	nodes informersv1.NodeInformer
}

// NewKubernetes creates new Kubernetes registry.
func NewKubernetes(client *kubernetes.Client) *Kubernetes {
	return &Kubernetes{
		client: client,
	}
}

// AnnotationsFromAffiliate generates Kubernetes Node annotations from the Affiliate spec.
func AnnotationsFromAffiliate(affiliate *cluster.TypedResource[cluster.AffiliateSpec, cluster.Affiliate]) map[string]string {
	var kubeSpanAddress string

	if !affiliate.TypedSpec().KubeSpan.Address.IsZero() {
		kubeSpanAddress = affiliate.TypedSpec().KubeSpan.Address.String()
	}

	return map[string]string{
		constants.ClusterNodeIDAnnotation:            affiliate.Metadata().ID(),
		constants.NetworkSelfIPsAnnotation:           ipsToString(affiliate.TypedSpec().Addresses),
		constants.KubeSpanIPAnnotation:               kubeSpanAddress,
		constants.KubeSpanPublicKeyAnnotation:        affiliate.TypedSpec().KubeSpan.PublicKey,
		constants.KubeSpanAssignedPrefixesAnnotation: ipPrefixesToString(affiliate.TypedSpec().KubeSpan.AdditionalAddresses),
		constants.KubeSpanKnownEndpointsAnnotation:   ipPortsToString(affiliate.TypedSpec().KubeSpan.Endpoints),
	}
}

// AffiliateFromNode converts Kubernetes Node resource to Affiliate.
//
// If the Node resource doesn't have cluster discovery annotations, nil is returned.
//
//nolint:gocyclo
func AffiliateFromNode(node *v1.Node) *cluster.AffiliateSpec {
	nodeID, ok := node.Annotations[constants.ClusterNodeIDAnnotation]
	if !ok {
		// skip the node, not part of the cluster discovery process
		return nil
	}

	affiliate := &cluster.AffiliateSpec{
		NodeID: nodeID,
	}

	if selfIPs, ok := node.Annotations[constants.NetworkSelfIPsAnnotation]; ok {
		affiliate.Addresses = parseIPs(selfIPs)
	}

	// Nodename and hostname are pulled from native Kubernetes fields.
	affiliate.Nodename = node.Name

	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeHostName {
			affiliate.Hostname = addr.Address

			break
		}
	}

	// Machine type is derived from node roles.
	_, labelMaster := node.Labels[constants.LabelNodeRoleMaster]
	_, labelControlPlane := node.Labels[constants.LabelNodeRoleControlPlane]

	affiliate.MachineType = machine.TypeWorker

	if labelMaster || labelControlPlane {
		affiliate.MachineType = machine.TypeControlPlane
	}

	affiliate.OperatingSystem = node.Status.NodeInfo.OSImage

	// Every other field is pulled from node annotations.
	if publicKey, ok := node.Annotations[constants.KubeSpanPublicKeyAnnotation]; ok {
		affiliate.KubeSpan.PublicKey = publicKey
	}

	if ksIP, ok := node.Annotations[constants.KubeSpanIPAnnotation]; ok {
		affiliate.KubeSpan.Address, _ = netaddr.ParseIP(ksIP) //nolint:errcheck
	}

	if additionalAddresses, ok := node.Annotations[constants.KubeSpanAssignedPrefixesAnnotation]; ok {
		affiliate.KubeSpan.AdditionalAddresses = parseIPPrefixes(additionalAddresses)
	}

	if endpoints, ok := node.Annotations[constants.KubeSpanKnownEndpointsAnnotation]; ok {
		affiliate.KubeSpan.Endpoints = parseIPPorts(endpoints)
	}

	return affiliate
}

func ipsToString(in []netaddr.IP) string {
	items := make([]string, len(in))

	for i := range in {
		items[i] = in[i].String()
	}

	return strings.Join(items, ",")
}

func ipPrefixesToString(in []netaddr.IPPrefix) string {
	items := make([]string, len(in))

	for i := range in {
		items[i] = in[i].String()
	}

	return strings.Join(items, ",")
}

func ipPortsToString(in []netaddr.IPPort) string {
	items := make([]string, len(in))

	for i := range in {
		items[i] = in[i].String()
	}

	return strings.Join(items, ",")
}

func parseIPs(in string) []netaddr.IP {
	var result []netaddr.IP

	for _, item := range strings.Split(in, ",") {
		if ip, err := netaddr.ParseIP(item); err == nil {
			result = append(result, ip)
		}
	}

	return result
}

func parseIPPrefixes(in string) []netaddr.IPPrefix {
	var result []netaddr.IPPrefix

	for _, item := range strings.Split(in, ",") {
		if ip, err := netaddr.ParseIPPrefix(item); err == nil {
			result = append(result, ip)
		}
	}

	return result
}

func parseIPPorts(in string) []netaddr.IPPort {
	var result []netaddr.IPPort

	for _, item := range strings.Split(in, ",") {
		if ip, err := netaddr.ParseIPPort(item); err == nil {
			result = append(result, ip)
		}
	}

	return result
}

// Push updates Kubernetes Node resource to track Affiliate state.
func (r *Kubernetes) Push(ctx context.Context, affiliate *cluster.TypedResource[cluster.AffiliateSpec, cluster.Affiliate]) error {
	node, err := r.client.CoreV1().Nodes().Get(ctx, affiliate.TypedSpec().Nodename, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %q: %w", affiliate.TypedSpec().Nodename, err)
	}

	oldData, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal existing node data: %w", err)
	}

	for key, value := range AnnotationsFromAffiliate(affiliate) {
		if value == "" {
			delete(node.Annotations, key)
		} else {
			node.Annotations[key] = value
		}
	}

	newData, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal new data for node %q: %w", node.Name, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1.Node{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch: %w", err)
	}

	if _, err := r.client.CoreV1().Nodes().Patch(ctx, affiliate.TypedSpec().Nodename, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		if apierrors.IsConflict(err) {
			return fmt.Errorf("unable to update node %q due to conflict: %w", affiliate.TypedSpec().Nodename, err)
		}

		return fmt.Errorf("error patching node %q: %w", affiliate.TypedSpec().Nodename, err)
	}

	return nil
}

// List returns list of Affiliates coming from the registry.
//
// Watch should be called first for the List to return data.
func (r *Kubernetes) List(localNodeName string) ([]*cluster.AffiliateSpec, error) {
	if r.nodes == nil {
		return nil, fmt.Errorf("List() called without Watch() first")
	}

	nodes, err := r.nodes.Lister().List(labels.Everything())
	if err != nil {
		return nil, err
	}

	result := make([]*cluster.AffiliateSpec, 0, len(nodes))

	for _, node := range nodes {
		// skip this node, no need to pull itself
		if node.Name == localNodeName {
			continue
		}

		affiliate := AffiliateFromNode(node)
		if affiliate == nil {
			continue
		}

		result = append(result, affiliate)
	}

	return result, nil
}

// Watch starts watching Node state and notifies on updates via notify channel.
func (r *Kubernetes) Watch(ctx context.Context, logger *zap.Logger) (<-chan struct{}, error) {
	informerFactory := informers.NewSharedInformerFactory(r.client.Clientset, 30*time.Second)

	notifyCh := make(chan struct{}, 1)

	notify := func(_ interface{}) {
		select {
		case notifyCh <- struct{}{}:
		default:
		}
	}

	r.nodes = informerFactory.Core().V1().Nodes()
	r.nodes.Informer().SetWatchErrorHandler(func(r *cache.Reflector, err error) { //nolint:errcheck
		logger.Error("kubernetes registry node watch error", zap.Error(err))
	})
	r.nodes.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    notify,
		DeleteFunc: notify,
		UpdateFunc: func(_, _ interface{}) { notify(nil) },
	})

	informerFactory.Start(ctx.Done())

	return notifyCh, nil
}
