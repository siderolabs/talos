// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeaccess

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"

	"github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubeaccess"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// EndpointController manages Kubernetes endpoints resource for Talos API endpoints.
type EndpointController struct{}

// Name implements controller.Controller interface.
func (ctrl *EndpointController) Name() string {
	return "kubeaccess.EndpointController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EndpointController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      kubeaccess.ConfigType,
			ID:        optional.Some(kubeaccess.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesType,
			ID:        optional.Some(secrets.KubernetesID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.EndpointType,
			Kind:      controller.InputWeak,
		},
	}
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
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		kubeaccessConfig, err := safe.ReaderGet[*kubeaccess.Config](ctx, r, kubeaccess.NewConfig(config.NamespaceName, kubeaccess.ConfigID).Metadata())
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error fetching kubeaccess config: %w", err)
			}
		}

		if kubeaccessConfig == nil || !kubeaccessConfig.TypedSpec().Enabled {
			// disabled, do not do anything
			continue
		}

		// use only api-server endpoints to leave only kubelet node IPs
		endpointResource, err := safe.ReaderGet[*k8s.Endpoint](ctx, r, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.EndpointType, k8s.ControlPlaneAPIServerEndpointsID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting endpoints resources: %w", err)
			}
		}

		var endpointAddrs k8s.EndpointList

		if endpointResource != nil {
			endpointAddrs = endpointAddrs.Merge(endpointResource)
		}

		if len(endpointAddrs) == 0 {
			continue
		}

		secretsResources, err := safe.ReaderGet[*secrets.Kubernetes](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesType, secrets.KubernetesID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		secrets := secretsResources.TypedSpec()

		kubeconfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
			return clientcmd.Load([]byte(secrets.LocalhostAdminKubeconfig))
		})
		if err != nil {
			return fmt.Errorf("error loading kubeconfig: %w", err)
		}

		if err = ctrl.manageEndpoints(ctx, logger, kubeconfig, endpointAddrs); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *EndpointController) manageEndpoints(ctx context.Context, logger *zap.Logger, kubeconfig *rest.Config, endpointAddrs k8s.EndpointList) error {
	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("error building Kubernetes client: %w", err)
	}

	defer client.Close() //nolint:errcheck

	// create the Service before creating the Endpoints, as Kubernetes EndpointController will clean up orphaned Endpoints
	if err = ctrl.ensureTalosService(ctx, client); err != nil {
		return fmt.Errorf("error ensuring Talos API service: %w", err)
	}

	// now create or update the EndpointSlices resource
	if err = ctrl.ensureTalosEndpointSlices(ctx, logger, client, endpointAddrs); err != nil {
		return fmt.Errorf("error ensuring Talos API endpoint slices: %w", err)
	}

	// clean-up deprecated endpoints
	if err = ctrl.cleanupTalosEndpoints(ctx, logger, client); err != nil {
		return fmt.Errorf("error cleaning up dangling Talos API endpoints: %w", err)
	}

	return nil
}

func (ctrl *EndpointController) ensureTalosService(ctx context.Context, client *kubernetes.Client) error {
	_, err := client.CoreV1().Services(constants.KubernetesTalosAPIServiceNamespace).Get(ctx, constants.KubernetesTalosAPIServiceName, metav1.GetOptions{})
	if err == nil {
		// service already exists, nothing to do
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting Talos API service: %w", err)
	}

	// create the service if it does not exist
	newService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.KubernetesTalosAPIServiceName,
			Namespace: constants.KubernetesTalosAPIServiceNamespace,
			Labels: map[string]string{
				"provider":  constants.KubernetesTalosProvider,
				"component": "apid",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "apid",
					Port:       constants.ApidPort,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(constants.ApidPort),
				},
			},
		},
	}

	_, err = client.CoreV1().Services(constants.KubernetesTalosAPIServiceNamespace).Create(ctx, newService, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating Talos API service: %w", err)
	}

	return nil
}

//nolint:gocyclo
func (ctrl *EndpointController) ensureTalosEndpointSlices(ctx context.Context, logger *zap.Logger, client *kubernetes.Client, endpointAddrs k8s.EndpointList) error {
	var (
		addrsIPv4 k8s.EndpointList
		addrsIPv6 k8s.EndpointList
	)

	for _, addr := range endpointAddrs {
		switch {
		case addr.Is4():
			addrsIPv4 = append(addrsIPv4, addr)

		case addr.Is6():
			addrsIPv6 = append(addrsIPv6, addr)

		default:
			// ignore other address types
		}
	}

	if len(addrsIPv4) == 0 {
		if err := ctrl.deleteTalosEndpointSlicesTyped(ctx, logger, client, discoveryv1.AddressTypeIPv4); err != nil {
			return fmt.Errorf("error deleting Talos API endpoint slices for IPv4: %w", err)
		}
	} else {
		if err := ctrl.ensureTalosEndpointSlicesTyped(ctx, logger, client, addrsIPv4, discoveryv1.AddressTypeIPv4); err != nil {
			return fmt.Errorf("error ensuring Talos API endpoint slices for IPv4: %w", err)
		}
	}

	if len(addrsIPv6) == 0 {
		if err := ctrl.deleteTalosEndpointSlicesTyped(ctx, logger, client, discoveryv1.AddressTypeIPv6); err != nil {
			return fmt.Errorf("error deleting Talos API endpoint slices for IPv6: %w", err)
		}
	} else {
		if err := ctrl.ensureTalosEndpointSlicesTyped(ctx, logger, client, addrsIPv6, discoveryv1.AddressTypeIPv6); err != nil {
			return fmt.Errorf("error ensuring Talos API endpoint slices for IPv6: %w", err)
		}
	}

	return nil
}

func (ctrl *EndpointController) deleteTalosEndpointSlicesTyped(ctx context.Context, logger *zap.Logger, client *kubernetes.Client, addressType discoveryv1.AddressType) error {
	endpointSliceName := constants.KubernetesTalosAPIServiceName + "-" + strings.ToLower(string(addressType))

	err := client.DiscoveryV1().EndpointSlices(constants.KubernetesTalosAPIServiceNamespace).Delete(ctx, endpointSliceName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("error deleting Talos API endpoint slices: %w", err)
	}

	logger.Info("deleted Talos API endpoint slices in Kubernetes", zap.String("addressType", string(addressType)))

	return nil
}

//nolint:gocyclo
func (ctrl *EndpointController) ensureTalosEndpointSlicesTyped(
	ctx context.Context,
	logger *zap.Logger,
	client *kubernetes.Client,
	endpointAddrs k8s.EndpointList,
	addressType discoveryv1.AddressType,
) error {
	for {
		esc := client.DiscoveryV1().EndpointSlices(constants.KubernetesTalosAPIServiceNamespace)
		name := constants.KubernetesTalosAPIServiceName + "-" + strings.ToLower(string(addressType))

		oldEndpointSlice, err := esc.Get(ctx, name, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting endpoints: %w", err)
		}

		var newEndpointSlice *discoveryv1.EndpointSlice

		if apierrors.IsNotFound(err) {
			newEndpointSlice = &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: constants.KubernetesTalosAPIServiceNamespace,
					Labels: map[string]string{
						"kubernetes.io/service-name": constants.KubernetesTalosAPIServiceName,
						"provider":                   constants.KubernetesTalosProvider,
						"component":                  "apid",
					},
				},
				AddressType: addressType,
			}
			oldEndpointSlice = nil
		} else {
			newEndpointSlice = oldEndpointSlice.DeepCopy()
		}

		newEndpointSlice.Ports = []discoveryv1.EndpointPort{
			{
				Name:     ptr.To("apid"),
				Port:     ptr.To[int32](constants.ApidPort),
				Protocol: ptr.To(corev1.ProtocolTCP),
			},
		}

		for _, addr := range endpointAddrs {
			newEndpointSlice.Endpoints = append(
				newEndpointSlice.Endpoints,
				discoveryv1.Endpoint{
					Addresses: []string{addr.String()},
					Conditions: discoveryv1.EndpointConditions{
						Ready:       ptr.To(true),
						Serving:     ptr.To(true),
						Terminating: ptr.To(false),
					},
				},
			)
		}

		newEndpointSlice.Endpoints = xslices.Deduplicate(newEndpointSlice.Endpoints, func(e discoveryv1.Endpoint) string {
			return e.Addresses[0]
		})

		if oldEndpointSlice != nil &&
			(reflect.DeepEqual(oldEndpointSlice.Endpoints, newEndpointSlice.Endpoints) &&
				reflect.DeepEqual(oldEndpointSlice.Ports, newEndpointSlice.Ports)) {
			// no change, bail out
			return nil
		}

		if oldEndpointSlice == nil {
			_, err = client.DiscoveryV1().EndpointSlices(constants.KubernetesTalosAPIServiceNamespace).Create(ctx, newEndpointSlice, metav1.CreateOptions{})
		} else {
			_, err = client.DiscoveryV1().EndpointSlices(constants.KubernetesTalosAPIServiceNamespace).Update(ctx, newEndpointSlice, metav1.UpdateOptions{})
		}

		switch {
		case err == nil:
			logger.Info("updated Talos API endpoint slices in Kubernetes", zap.Strings("endpoints", endpointAddrs.Strings()))

			return nil
		case apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err):
			// retry
		default:
			return fmt.Errorf("error updating Kubernetes Talos API endpoint slices: %w", err)
		}
	}
}

//nolint:gocyclo
func (ctrl *EndpointController) cleanupTalosEndpoints(ctx context.Context, logger *zap.Logger, client *kubernetes.Client) error {
	for {
		err := client.CoreV1().Endpoints(constants.KubernetesTalosAPIServiceNamespace).Delete(ctx, constants.KubernetesTalosAPIServiceName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("error getting endpoints: %w", err)
		}

		switch {
		case err == nil:
			logger.Info("deleted dangling Talos API endpoints in Kubernetes")

			return nil
		case apierrors.IsNotFound(err):
			logger.Info("no dangling Talos API endpoints in Kubernetes")

			return nil

		default:
			return fmt.Errorf("error deleting dangling Kubernetes Talos API endpoints: %w", err)
		}
	}
}
