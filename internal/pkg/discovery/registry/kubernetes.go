// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/informers"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
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
func AnnotationsFromAffiliate(affiliate *cluster.Affiliate) map[string]string {
	var kubeSpanAddress string

	if !value.IsZero(affiliate.TypedSpec().KubeSpan.Address) {
		kubeSpanAddress = affiliate.TypedSpec().KubeSpan.Address.String()
	}

	var apiServerPort string

	if affiliate.TypedSpec().ControlPlane != nil {
		apiServerPort = strconv.Itoa(affiliate.TypedSpec().ControlPlane.APIServerPort)
	}

	return map[string]string{
		constants.ClusterNodeIDAnnotation:            affiliate.Metadata().ID(),
		constants.NetworkSelfIPsAnnotation:           ipsToString(affiliate.TypedSpec().Addresses),
		constants.NetworkAPIServerPortAnnotation:     apiServerPort,
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
	_, labelControlPlane := node.Labels[constants.LabelNodeRoleControlPlane]

	affiliate.MachineType = machine.TypeWorker

	if labelControlPlane {
		affiliate.MachineType = machine.TypeControlPlane
	}

	affiliate.OperatingSystem = node.Status.NodeInfo.OSImage

	// Every other field is pulled from node annotations.
	if publicKey, ok := node.Annotations[constants.KubeSpanPublicKeyAnnotation]; ok {
		affiliate.KubeSpan.PublicKey = publicKey
	}

	if ksIP, ok := node.Annotations[constants.KubeSpanIPAnnotation]; ok {
		affiliate.KubeSpan.Address, _ = netip.ParseAddr(ksIP) //nolint:errcheck
	}

	if additionalAddresses, ok := node.Annotations[constants.KubeSpanAssignedPrefixesAnnotation]; ok {
		affiliate.KubeSpan.AdditionalAddresses = parseIPPrefixes(additionalAddresses)
	}

	if endpoints, ok := node.Annotations[constants.KubeSpanKnownEndpointsAnnotation]; ok {
		affiliate.KubeSpan.Endpoints = parseIPPorts(endpoints)
	}

	if apiServerPort, ok := node.Annotations[constants.NetworkAPIServerPortAnnotation]; ok {
		if port, err := strconv.Atoi(apiServerPort); err == nil {
			affiliate.ControlPlane = &cluster.ControlPlane{
				APIServerPort: port,
			}
		}
	}

	return affiliate
}

func ipsToString(in []netip.Addr) string {
	return strings.Join(xslices.Map(in, netip.Addr.String), ",")
}

func ipPrefixesToString(in []netip.Prefix) string {
	return strings.Join(xslices.Map(in, netip.Prefix.String), ",")
}

func ipPortsToString(in []netip.AddrPort) string {
	return strings.Join(xslices.Map(in, netip.AddrPort.String), ",")
}

func parseIPs(in string) []netip.Addr {
	var result []netip.Addr

	for _, item := range strings.Split(in, ",") {
		if ip, err := netip.ParseAddr(item); err == nil {
			result = append(result, ip)
		}
	}

	return result
}

func parseIPPrefixes(in string) []netip.Prefix {
	var result []netip.Prefix

	for _, item := range strings.Split(in, ",") {
		if ip, err := netip.ParsePrefix(item); err == nil {
			result = append(result, ip)
		}
	}

	return result
}

func parseIPPorts(in string) []netip.AddrPort {
	var result []netip.AddrPort

	for _, item := range strings.Split(in, ",") {
		if ip, err := netip.ParseAddrPort(item); err == nil {
			result = append(result, ip)
		}
	}

	return result
}

// Push updates Kubernetes Node resource to track Affiliate state.
func (r *Kubernetes) Push(ctx context.Context, affiliate *cluster.Affiliate) error {
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
		return nil, errors.New("List() called without Watch() first")
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
func (r *Kubernetes) Watch(ctx context.Context, logger *zap.Logger) (<-chan struct{}, func(), error) {
	informerFactory := informers.NewSharedInformerFactory(r.client.Clientset, 30*time.Second)

	notifyCh := make(chan struct{}, 1)

	notify := func(_ any) {
		select {
		case notifyCh <- struct{}{}:
		default:
		}
	}

	r.nodes = informerFactory.Core().V1().Nodes()

	if err := r.nodes.Informer().SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		logger.Error("kubernetes registry node watch error", zap.Error(err))
	}); err != nil {
		return nil, nil, fmt.Errorf("failed to set watch error handler: %w", err)
	}

	if _, err := r.nodes.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    notify,
		DeleteFunc: notify,
		UpdateFunc: func(_, _ any) { notify(nil) },
	}); err != nil {
		return nil, nil, fmt.Errorf("failed to add event handler: %w", err)
	}

	informerFactory.Start(ctx.Done())

	return notifyCh, informerFactory.Shutdown, nil
}
