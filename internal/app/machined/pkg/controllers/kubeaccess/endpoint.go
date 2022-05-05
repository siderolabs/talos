// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeaccess

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/kubeaccess"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
)

// EndpointController manages Kubernetes endpoints resource for Talos API endpoints.
type EndpointController struct{}

// Name implements controller.Controller interface.
func (ctrl *EndpointController) Name() string {
	return "kubeaccess.EndpointController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EndpointController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *EndpointController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *EndpointController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		if err := r.UpdateInputs([]controller.Input{
			{
				Namespace: config.NamespaceName,
				Type:      kubeaccess.ConfigType,
				ID:        pointer.To(kubeaccess.ConfigID),
				Kind:      controller.InputWeak,
			},
		}); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		kubeaccessConfig, err := r.Get(ctx, kubeaccess.NewConfig(config.NamespaceName, kubeaccess.ConfigID).Metadata())
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error fetching kubeaccess config: %w", err)
			}
		}

		if kubeaccessConfig == nil || !kubeaccessConfig.(*kubeaccess.Config).TypedSpec().Enabled {
			// disabled, nothing to do
			continue
		}

		if err = ctrl.reconcile(ctx, r, logger); err != nil {
			return err
		}
	}
}

//nolint:gocyclo
func (ctrl *EndpointController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      kubeaccess.ConfigType,
			ID:        pointer.To(kubeaccess.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesType,
			ID:        pointer.To(secrets.KubernetesID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.EndpointType,
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return err
	}

	r.QueueReconcile()

	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		kubeaccessConfig, err := r.Get(ctx, kubeaccess.NewConfig(config.NamespaceName, kubeaccess.ConfigID).Metadata())
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error fetching kubeaccess config: %w", err)
			}
		}

		if kubeaccessConfig == nil || !kubeaccessConfig.(*kubeaccess.Config).TypedSpec().Enabled {
			// disabled, bail out
			return nil
		}

		endpointResources, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.EndpointType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error getting endpoints resources: %w", err)
		}

		var endpointAddrs k8s.EndpointList

		// merge all endpoints into a single list
		for _, res := range endpointResources.Items {
			endpointAddrs = endpointAddrs.Merge(res.(*k8s.Endpoint))
		}

		if len(endpointAddrs) == 0 {
			continue
		}

		secretsResources, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesType, secrets.KubernetesID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		secrets := secretsResources.(*secrets.Kubernetes).TypedSpec()

		kubeconfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
			return clientcmd.Load([]byte(secrets.LocalhostAdminKubeconfig))
		})
		if err != nil {
			return fmt.Errorf("error loading kubeconfig: %w", err)
		}

		if err = ctrl.updateTalosEndpoints(ctx, r, logger, kubeconfig, endpointAddrs); err != nil {
			return err
		}
	}
}

//nolint:gocyclo
func (ctrl *EndpointController) updateTalosEndpoints(ctx context.Context, r controller.Runtime, logger *zap.Logger, kubeconfig *rest.Config, endpointAddrs k8s.EndpointList) error {
	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("error building Kubernetes client: %w", err)
	}

	defer client.Close() //nolint:errcheck

	for {
		oldEndpoints, err := client.CoreV1().Endpoints(constants.KubernetesTalosAPIServiceNamespace).Get(ctx, constants.KubernetesTalosAPIServiceName, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting endpoints: %w", err)
		}

		var newEndpoints *corev1.Endpoints

		if apierrors.IsNotFound(err) {
			newEndpoints = &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.KubernetesTalosAPIServiceName,
					Namespace: constants.KubernetesTalosAPIServiceNamespace,
					Labels: map[string]string{
						"provider":  "talos",
						"component": "apid",
					},
				},
			}
		} else {
			newEndpoints = oldEndpoints.DeepCopy()
		}

		newEndpoints.Subsets = []corev1.EndpointSubset{
			{
				Ports: []corev1.EndpointPort{
					{
						Name:     "apid",
						Port:     50000,
						Protocol: "TCP",
					},
				},
			},
		}

		for _, addr := range endpointAddrs {
			newEndpoints.Subsets[0].Addresses = append(newEndpoints.Subsets[0].Addresses,
				corev1.EndpointAddress{
					IP: addr.String(),
				},
			)
		}

		if oldEndpoints != nil && reflect.DeepEqual(oldEndpoints.Subsets, newEndpoints.Subsets) {
			// no change, bail out
			return nil
		}

		if oldEndpoints == nil {
			_, err = client.CoreV1().Endpoints(constants.KubernetesTalosAPIServiceNamespace).Create(ctx, newEndpoints, metav1.CreateOptions{})
		} else {
			_, err = client.CoreV1().Endpoints(constants.KubernetesTalosAPIServiceNamespace).Update(ctx, newEndpoints, metav1.UpdateOptions{})
		}

		switch {
		case err == nil:
			logger.Info("updated Talos API endpoints in Kubernetes", zap.Strings("endpoints", endpointAddrs.Strings()))

			return nil
		case apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err):
			// retry
		default:
			return fmt.Errorf("error updating Kubernetes Talos API endpoints: %w", err)
		}
	}
}
