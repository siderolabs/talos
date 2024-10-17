// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// NodeApplyController watches k8s.NodeLabelSpecs, k8s.NodeTaintSpecs and applies them to the k8s Node object.
type NodeApplyController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodeApplyController) Name() string {
	return "k8s.NodeApplyController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeApplyController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodeAnnotationSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodeLabelSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodeTaintSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodeCordonedSpecType,
			Kind:      controller.InputWeak,
		},
		{
			// NodeStatus is used to trigger the controller on node status updates.
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesRootType,
			ID:        optional.Some(secrets.KubernetesRootID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        optional.Some(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        optional.Some(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NodeApplyController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *NodeApplyController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcileWithK8s(ctx, r, logger); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *NodeApplyController) getNodeLabelSpecs(ctx context.Context, r controller.Runtime) (map[string]string, error) {
	items, err := safe.ReaderListAll[*k8s.NodeLabelSpec](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing node label spec resources: %w", err)
	}

	result := make(map[string]string, items.Len())

	for res := range items.All() {
		result[res.TypedSpec().Key] = res.TypedSpec().Value
	}

	return result, nil
}

func (ctrl *NodeApplyController) getNodeAnnotationSpecs(ctx context.Context, r controller.Runtime) (map[string]string, error) {
	items, err := safe.ReaderListAll[*k8s.NodeAnnotationSpec](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing node annotation spec resources: %w", err)
	}

	result := make(map[string]string, items.Len())

	for res := range items.All() {
		result[res.TypedSpec().Key] = res.TypedSpec().Value
	}

	return result, nil
}

func (ctrl *NodeApplyController) getNodeTaintSpecs(ctx context.Context, r controller.Runtime) ([]k8s.NodeTaintSpecSpec, error) {
	items, err := safe.ReaderListAll[*k8s.NodeTaintSpec](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("error listing node taint spec resources: %w", err)
	}

	result := make([]k8s.NodeTaintSpecSpec, 0, items.Len())

	for res := range items.All() {
		result = append(result, *res.TypedSpec())
	}

	return result, nil
}

func (ctrl *NodeApplyController) getNodeCordoned(ctx context.Context, r controller.Runtime) (bool, error) {
	items, err := safe.ReaderListAll[*k8s.NodeCordonedSpec](ctx, r)
	if err != nil {
		return false, fmt.Errorf("error listing node cordoned spec resources: %w", err)
	}

	return items.Len() > 0, nil
}

func (ctrl *NodeApplyController) getK8sClient(ctx context.Context, r controller.Runtime, logger *zap.Logger) (*kubernetes.Client, error) {
	machineType, err := safe.ReaderGet[*config.MachineType](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
	if err != nil {
		return nil, fmt.Errorf("error getting machine type: %w", err)
	}

	if machineType.MachineType().IsControlPlane() {
		return kubernetes.NewTemporaryClientControlPlane(ctx, r)
	}

	logger.Debug("waiting for kubelet client config", zap.String("file", constants.KubeletKubeconfig))

	if err := conditions.WaitForKubeconfigReady(constants.KubeletKubeconfig).Wait(ctx); err != nil {
		return nil, err
	}

	return kubernetes.NewClientFromKubeletKubeconfig()
}

func (ctrl *NodeApplyController) reconcileWithK8s(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
) error {
	nodenameResource, err := safe.ReaderGet[*k8s.Nodename](ctx, r, resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, k8s.NodenameID, resource.VersionUndefined))
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if nodenameResource.TypedSpec().SkipNodeRegistration {
		// if the node registration is skipped, we don't need to do anything
		return nil
	}

	nodename := nodenameResource.TypedSpec().Nodename

	k8sClient, err := ctrl.getK8sClient(ctx, r, logger)
	if err != nil {
		return fmt.Errorf("error building kubernetes client: %w", err)
	}

	if k8sClient == nil {
		// not ready yet
		return nil
	}

	defer k8sClient.Close() //nolint:errcheck

	nodeLabelSpecs, err := ctrl.getNodeLabelSpecs(ctx, r)
	if err != nil {
		return err
	}

	nodeAnnotationSpecs, err := ctrl.getNodeAnnotationSpecs(ctx, r)
	if err != nil {
		return err
	}

	nodeTaintSpecs, err := ctrl.getNodeTaintSpecs(ctx, r)
	if err != nil {
		return err
	}

	nodeShouldCordon, err := ctrl.getNodeCordoned(ctx, r)
	if err != nil {
		return err
	}

	return ctrl.sync(ctx, logger, k8sClient, nodename, nodeLabelSpecs, nodeAnnotationSpecs, nodeTaintSpecs, nodeShouldCordon)
}

func (ctrl *NodeApplyController) sync(
	ctx context.Context,
	logger *zap.Logger,
	k8sClient *kubernetes.Client,
	nodeName string,
	nodeLabelSpecs, nodeAnnotationSpecs map[string]string,
	nodeTaintSpecs []k8s.NodeTaintSpecSpec,
	nodeShouldCordon bool,
) error {
	// run several attempts retrying conflict errors
	return retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(ctx, func(ctx context.Context) error {
		err := ctrl.syncOnce(ctx, logger, k8sClient, nodeName, nodeLabelSpecs, nodeAnnotationSpecs, nodeTaintSpecs, nodeShouldCordon)

		if err != nil && (apierrors.IsConflict(err) || apierrors.IsForbidden(err)) {
			return retry.ExpectedError(err)
		}

		return err
	})
}

func umarshalOwnedAnnotation(node *v1.Node, annotation string) (map[string]struct{}, error) {
	ownedJSON := []byte(node.Annotations[annotation])

	var owned []string

	if len(ownedJSON) > 0 {
		if err := json.Unmarshal(ownedJSON, &owned); err != nil {
			return nil, err
		}
	}

	ownedMap := xslices.ToSet(owned)
	if ownedMap == nil {
		ownedMap = map[string]struct{}{}
	}

	return ownedMap, nil
}

func marshalOwnedAnnotation(node *v1.Node, annotation string, ownedMap map[string]struct{}) error {
	owned := maps.Keys(ownedMap)
	slices.Sort(owned)

	if len(owned) > 0 {
		ownedJSON, err := json.Marshal(owned)
		if err != nil {
			return err
		}

		node.Annotations[annotation] = string(ownedJSON)
	} else {
		delete(node.Annotations, annotation)
	}

	return nil
}

func (ctrl *NodeApplyController) syncOnce(
	ctx context.Context,
	logger *zap.Logger,
	k8sClient *kubernetes.Client,
	nodeName string,
	nodeLabelSpecs, nodeAnnotationSpecs map[string]string,
	nodeTaintSpecs []k8s.NodeTaintSpecSpec,
	nodeShouldCordon bool,
) error {
	node, err := k8sClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting node: %w", err)
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	ownedLabelsMap, err := umarshalOwnedAnnotation(node, constants.AnnotationOwnedLabels)
	if err != nil {
		return fmt.Errorf("error unmarshaling owned labels: %w", err)
	}

	ownedAnnotationsMap, err := umarshalOwnedAnnotation(node, constants.AnnotationOwnedAnnotations)
	if err != nil {
		return fmt.Errorf("error unmarshaling owned annotations: %w", err)
	}

	ownedTaintsMap, err := umarshalOwnedAnnotation(node, constants.AnnotationOwnedTaints)
	if err != nil {
		return fmt.Errorf("error unmarshaling owned taints: %w", err)
	}

	ctrl.ApplyLabels(logger, node, ownedLabelsMap, nodeLabelSpecs)
	ctrl.ApplyAnnotations(logger, node, ownedAnnotationsMap, nodeAnnotationSpecs)
	ctrl.ApplyTaints(logger, node, ownedTaintsMap, nodeTaintSpecs)
	ctrl.ApplyCordoned(logger, node, nodeShouldCordon)

	if err = marshalOwnedAnnotation(node, constants.AnnotationOwnedLabels, ownedLabelsMap); err != nil {
		return fmt.Errorf("error marshaling owned labels: %w", err)
	}

	if err = marshalOwnedAnnotation(node, constants.AnnotationOwnedAnnotations, ownedAnnotationsMap); err != nil {
		return fmt.Errorf("error marshaling owned annotations: %w", err)
	}

	if err = marshalOwnedAnnotation(node, constants.AnnotationOwnedTaints, ownedTaintsMap); err != nil {
		return fmt.Errorf("error marshaling owned taints: %w", err)
	}

	_, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})

	return err
}

func (ctrl *NodeApplyController) applyNodeKV(logger *zap.Logger, nodeKV map[string]string, owned map[string]struct{}, spec map[string]string) {
	// set labels from the spec
	for key, value := range spec {
		currentValue, exists := nodeKV[key]

		// label is not set on the node yet, so take it over
		if !exists {
			nodeKV[key] = value
			owned[key] = struct{}{}

			continue
		}

		// no change to the label, skip it
		if currentValue == value {
			owned[key] = struct{}{}

			continue
		}

		if _, owned := owned[key]; !owned {
			logger.Debug("skipping label update, label is not owned", zap.String("key", key), zap.String("value", value))

			continue
		}

		nodeKV[key] = value
	}

	// remove labels which are owned but are not in the spec
	for key := range owned {
		if _, exists := spec[key]; !exists {
			delete(nodeKV, key)
			delete(owned, key)
		}
	}
}

// ApplyLabels performs the inner loop of the node label reconciliation.
//
// This method is exported for testing purposes.
func (ctrl *NodeApplyController) ApplyLabels(logger *zap.Logger, node *v1.Node, ownedLabels map[string]struct{}, nodeLabelSpecs map[string]string) {
	ctrl.applyNodeKV(logger, node.Labels, ownedLabels, nodeLabelSpecs)
}

// ApplyAnnotations performs the inner loop of the node annotation reconciliation.
//
// This method is exported for testing purposes.
func (ctrl *NodeApplyController) ApplyAnnotations(logger *zap.Logger, node *v1.Node, ownedAnnotations map[string]struct{}, nodeAnnotationSpecs map[string]string) {
	ctrl.applyNodeKV(logger, node.Annotations, ownedAnnotations, nodeAnnotationSpecs)
}

// ApplyTaints performs the inner loop of the node taints reconciliation.
//
// This method is exported for testing purposes.
//
//nolint:gocyclo
func (ctrl *NodeApplyController) ApplyTaints(logger *zap.Logger, node *v1.Node, ownedTaints map[string]struct{}, nodeTaints []k8s.NodeTaintSpecSpec) {
	// set taints from the spec
	for _, taint := range nodeTaints {
		var currentValue *v1.Taint

		for i, nodeTaint := range node.Spec.Taints {
			if nodeTaint.Key == taint.Key {
				currentValue = &node.Spec.Taints[i]
			}
		}

		if currentValue == nil {
			// taint is not set on the node yet, so take it over
			node.Spec.Taints = append(node.Spec.Taints, v1.Taint{
				Key:    taint.Key,
				Value:  taint.Value,
				Effect: v1.TaintEffect(taint.Effect),
			})
			ownedTaints[taint.Key] = struct{}{}
		} else {
			// taint with the same key exists, check if it is owned
			if _, owned := ownedTaints[taint.Key]; owned {
				// taint is owned, so update it
				currentValue.Value = taint.Value
				currentValue.Effect = v1.TaintEffect(taint.Effect)
			} else if currentValue.Value == taint.Value && currentValue.Effect == v1.TaintEffect(taint.Effect) {
				// no change to the taint, skip it, but mark it as owned
				ownedTaints[taint.Key] = struct{}{}
			} else {
				logger.Debug("skipping taint update, taint is not owned", zap.String("key", taint.Key), zap.String("value", taint.Value), zap.String("effect", taint.Effect))
			}
		}
	}

	// remove taints which are owned but are not in the spec
	node.Spec.Taints = xslices.FilterInPlace(node.Spec.Taints,
		func(nodeTaint v1.Taint) bool {
			if _, owned := ownedTaints[nodeTaint.Key]; !owned {
				return true
			}

			for _, taint := range nodeTaints {
				if nodeTaint.Key == taint.Key {
					return true
				}
			}

			delete(ownedTaints, nodeTaint.Key)

			return false
		})
}

// ApplyCordoned marks the node as unschedulable if it is cordoned.
//
// This method is exported for testing purposes.
func (ctrl *NodeApplyController) ApplyCordoned(logger *zap.Logger, node *v1.Node, shouldCordon bool) {
	switch {
	case shouldCordon && !node.Spec.Unschedulable:
		node.Spec.Unschedulable = true

		if node.Annotations == nil {
			node.Annotations = map[string]string{}
		}

		node.Annotations[constants.AnnotationCordonedKey] = constants.AnnotationCordonedValue
	case !shouldCordon && node.Spec.Unschedulable:
		if _, exists := node.Annotations[constants.AnnotationCordonedKey]; !exists {
			// not cordoned by Talos, skip
			return
		}

		node.Spec.Unschedulable = false
		delete(node.Annotations, constants.AnnotationCordonedKey)
	}
}
