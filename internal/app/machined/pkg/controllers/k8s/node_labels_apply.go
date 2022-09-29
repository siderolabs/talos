// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/slices"
	"github.com/siderolabs/go-pointer"
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

// NodeLabelsApplyController watches k8s.NodeLabelSpec's and applies them to the k8s Node object.
type NodeLabelsApplyController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodeLabelsApplyController) Name() string {
	return "k8s.NodeLabelsApplyController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeLabelsApplyController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodeLabelSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesRootType,
			ID:        pointer.To(secrets.KubernetesRootID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        pointer.To(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.To(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NodeLabelsApplyController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *NodeLabelsApplyController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcileWithK8s(ctx, r, logger); err != nil {
			return err
		}
	}
}

func (ctrl *NodeLabelsApplyController) getNodeLabelSpecs(ctx context.Context, r controller.Runtime) (map[string]string, error) {
	items, err := safe.ReaderList[*k8s.NodeLabelSpec](ctx, r, resource.NewMetadata(k8s.NamespaceName, k8s.NodeLabelSpecType, "", resource.VersionUndefined))
	if err != nil {
		return nil, fmt.Errorf("error listing node label spec resources: %w", err)
	}

	result := make(map[string]string, items.Len())

	for iter := safe.IteratorFromList(items); iter.Next(); {
		result[iter.Value().TypedSpec().Key] = iter.Value().TypedSpec().Value
	}

	return result, nil
}

func (ctrl *NodeLabelsApplyController) getK8sClient(ctx context.Context, r controller.Runtime, logger *zap.Logger) (*kubernetes.Client, error) {
	machineType, err := safe.ReaderGet[*config.MachineType](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
	if err != nil {
		return nil, fmt.Errorf("error getting machine type: %w", err)
	}

	if machineType.MachineType().IsControlPlane() {
		k8sRoot, err := safe.ReaderGet[*secrets.KubernetesRoot](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesRootType, secrets.KubernetesRootID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, nil
			}

			return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
		}

		k8sRootSpec := k8sRoot.TypedSpec()

		return kubernetes.NewTemporaryClientFromPKI(k8sRootSpec.CA, k8sRootSpec.Endpoint)
	}

	logger.Debug("waiting for kubelet client config", zap.String("file", constants.KubeletKubeconfig))

	if err := conditions.WaitForKubeconfigReady(constants.KubeletKubeconfig).Wait(ctx); err != nil {
		return nil, err
	}

	return kubernetes.NewClientFromKubeletKubeconfig()
}

func (ctrl *NodeLabelsApplyController) reconcileWithK8s(
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

	return ctrl.syncLabels(ctx, logger, k8sClient, nodename, nodeLabelSpecs)
}

func (ctrl *NodeLabelsApplyController) syncLabels(
	ctx context.Context,
	logger *zap.Logger,
	k8sClient *kubernetes.Client,
	nodeName string,
	nodeLabelSpecs map[string]string,
) error {
	// run several attempts retrying conflict errors
	return retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).RetryWithContext(ctx, func(ctx context.Context) error {
		err := ctrl.syncLabelsOnce(ctx, logger, k8sClient, nodeName, nodeLabelSpecs)

		if err != nil && apierrors.IsConflict(err) {
			return retry.ExpectedError(err)
		}

		return err
	})
}

func (ctrl *NodeLabelsApplyController) syncLabelsOnce(
	ctx context.Context,
	logger *zap.Logger,
	k8sClient *kubernetes.Client,
	nodeName string,
	nodeLabelSpecs map[string]string,
) error {
	node, err := k8sClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting node: %w", err)
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	ownedJSON := []byte(node.Annotations[constants.AnnotationOwnedLabels])
	ownedLabels := []string{}

	if len(ownedJSON) > 0 {
		if err = json.Unmarshal(ownedJSON, &ownedLabels); err != nil {
			return fmt.Errorf("error unmarshaling owned labels: %w", err)
		}
	}

	ownedLabelsMap := slices.ToSet(ownedLabels)
	if ownedLabelsMap == nil {
		ownedLabelsMap = map[string]struct{}{}
	}

	ctrl.ApplyLabels(logger, node, ownedLabelsMap, nodeLabelSpecs)

	ownedLabels = maps.Keys(ownedLabelsMap)
	sort.Strings(ownedLabels)

	if len(ownedLabels) > 0 {
		ownedJSON, err = json.Marshal(ownedLabels)
		if err != nil {
			return fmt.Errorf("error marshaling owned labels: %w", err)
		}

		node.Annotations[constants.AnnotationOwnedLabels] = string(ownedJSON)
	} else {
		delete(node.Annotations, constants.AnnotationOwnedLabels)
	}

	_, err = k8sClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})

	return err
}

// ApplyLabels performs the inner loop of the node label reconciliation.
//
// This method is exported for testing purposes.
func (ctrl *NodeLabelsApplyController) ApplyLabels(logger *zap.Logger, node *v1.Node, ownedLabels map[string]struct{}, nodeLabelSpecs map[string]string) {
	// set labels from the spec
	for key, value := range nodeLabelSpecs {
		currentValue, exists := node.Labels[key]

		// label is not set on the node yet, so take it over
		if !exists {
			node.Labels[key] = value
			ownedLabels[key] = struct{}{}

			continue
		}

		// no change to the label, skip it
		if currentValue == value {
			continue
		}

		if _, owned := ownedLabels[key]; !owned {
			logger.Debug("skipping label update, label is not owned", zap.String("key", key), zap.String("value", value))

			continue
		}

		node.Labels[key] = value
	}

	// remove labels which are owned but are not in the spec
	for key := range ownedLabels {
		if _, exists := nodeLabelSpecs[key]; !exists {
			delete(node.Labels, key)
			delete(ownedLabels, key)
		}
	}
}
