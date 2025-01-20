// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/nodewatch"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// watchErrorsThreshold is the number of consecutive watch errors before the controller stops watching.
const watchErrorsThreshold = 5

// NodeStatusController pulls list of Affiliate resource from the Kubernetes registry.
type NodeStatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodeStatusController) Name() string {
	return "k8s.NodeStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			ID:        optional.Some(k8s.NodenameID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NodeStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.NodeStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *NodeStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var (
		kubernetesClient *kubernetes.Client
		nodewatcher      *nodewatch.NodeWatcher
		watchCtxCancel   context.CancelFunc
		notifyCh         <-chan struct{}
		watchErrCh       <-chan error
		notifyCloser     func()
		watchErrors      int
		watchReady       bool
	)

	closeWatcher := func() {
		if watchCtxCancel != nil {
			watchCtxCancel()
			watchCtxCancel = nil
		}

		if notifyCloser != nil {
			notifyCloser()
			notifyCloser = nil
			notifyCh = nil
			watchErrCh = nil
		}

		if kubernetesClient != nil {
			kubernetesClient.Close() //nolint:errcheck

			kubernetesClient = nil
		}

		watchErrors = 0
		watchReady = false
		nodewatcher = nil
	}

	defer closeWatcher()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-notifyCh:
			watchErrors = 0
			watchReady = true
		case watchErr := <-watchErrCh:
			logger.Error("node watch error", zap.Error(watchErr), zap.Int("error_count", watchErrors))

			watchErrors++

			if watchErrors >= watchErrorsThreshold {
				closeWatcher()
			} else {
				// keep waiting
				continue
			}
		}

		nodename, err := safe.ReaderGetByID[*k8s.Nodename](ctx, r, k8s.NodenameID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting nodename: %w", err)
			}

			continue
		}

		if nodename.TypedSpec().SkipNodeRegistration {
			// node is not registered with Kubernetes, so we can't pull the status
			closeWatcher()

			continue
		}

		if err = conditions.WaitForKubeconfigReady(constants.KubeletKubeconfig).Wait(ctx); err != nil {
			return err
		}

		if nodewatcher != nil && nodewatcher.Nodename() != nodename.TypedSpec().Nodename {
			// nodename changed, so we need to reinitialize the watcher
			closeWatcher()
		}

		if kubernetesClient == nil {
			kubernetesClient, err = kubernetes.NewClientFromKubeletKubeconfig()
			if err != nil {
				return fmt.Errorf("error building kubernetes client: %w", err)
			}
		}

		if nodewatcher == nil {
			nodewatcher = nodewatch.NewNodeWatcher(kubernetesClient, nodename.TypedSpec().Nodename)
		}

		if notifyCh == nil {
			var watchCtx context.Context
			watchCtx, watchCtxCancel = context.WithCancel(ctx) //nolint:govet

			notifyCh, watchErrCh, notifyCloser, err = nodewatcher.Watch(watchCtx, logger)
			if err != nil {
				return fmt.Errorf("error setting up node watcher: %w", err) //nolint:govet
			}
		}

		if !watchReady {
			// node watcher is not ready yet, skip updating output resource
			continue
		}

		touchedIDs := make(map[resource.ID]struct{})

		node, err := nodewatcher.Get()
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting node: %w", err)
		}

		if node != nil {
			if err = safe.WriterModify(ctx, r, k8s.NewNodeStatus(k8s.NamespaceName, node.Name),
				func(res *k8s.NodeStatus) error {
					res.TypedSpec().Nodename = node.Name
					res.TypedSpec().Unschedulable = node.Spec.Unschedulable
					res.TypedSpec().Labels = node.Labels
					res.TypedSpec().Annotations = node.Annotations
					res.TypedSpec().NodeReady = false

					for _, condition := range node.Status.Conditions {
						if condition.Type == v1.NodeReady {
							res.TypedSpec().NodeReady = condition.Status == v1.ConditionTrue
						}
					}

					return nil
				},
			); err != nil {
				return err
			}

			touchedIDs[node.Name] = struct{}{}
		}

		items, err := safe.ReaderListAll[*k8s.NodeStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing node statuses: %w", err)
		}

		for res := range items.All() {
			if _, touched := touchedIDs[res.Metadata().ID()]; touched {
				continue
			}

			if err = r.Destroy(ctx, res.Metadata()); err != nil {
				return fmt.Errorf("error destroying node status: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}
