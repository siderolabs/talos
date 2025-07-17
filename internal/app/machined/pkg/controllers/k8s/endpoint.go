// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// EndpointController looks up control plane endpoints.
type EndpointController struct{}

// Name implements controller.Controller interface.
func (ctrl *EndpointController) Name() string {
	return "k8s.EndpointController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EndpointController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *EndpointController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.EndpointType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *EndpointController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        optional.Some(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		machineTypeRes, err := safe.ReaderGetByID[*config.MachineType](ctx, r, config.MachineTypeID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		machineType := machineTypeRes.MachineType()

		switch machineType { //nolint:exhaustive
		case machine.TypeWorker:
			if err = ctrl.watchEndpointsOnWorker(ctx, r, logger); err != nil {
				return err
			}
		case machine.TypeControlPlane, machine.TypeInit:
			if err = ctrl.watchEndpointsOnControlPlane(ctx, r, logger); err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *EndpointController) watchEndpointsOnWorker(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	logger.Debug("waiting for kubelet client config", zap.String("file", constants.KubeletKubeconfig))

	if err := conditions.WaitForKubeconfigReady(constants.KubeletKubeconfig).Wait(ctx); err != nil {
		return err
	}

	client, err := kubernetes.NewClientFromKubeletKubeconfig()
	if err != nil {
		return fmt.Errorf("error building Kubernetes client: %w", err)
	}

	defer client.Close() //nolint:errcheck

	r.QueueReconcile()

	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		if err = ctrl.watchKubernetesEndpointSlices(ctx, r, logger, client); err != nil {
			return err
		}
	}
}

//nolint:gocyclo
func (ctrl *EndpointController) watchEndpointsOnControlPlane(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        optional.Some(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesType,
			ID:        optional.Some(secrets.KubernetesID),
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

		secretsResources, err := safe.ReaderGetByID[*secrets.Kubernetes](ctx, r, secrets.KubernetesID)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		secrets := secretsResources.TypedSpec()

		kubeconfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
			// using here kubeconfig with cluster control plane endpoint, as endpoint discovery should work before local API server is ready
			return clientcmd.Load([]byte(secrets.AdminKubeconfig))
		})
		if err != nil {
			return fmt.Errorf("error loading kubeconfig: %w", err)
		}

		// closure to capture the deferred close on client
		watch := func() error {
			client, err := kubernetes.NewForConfig(kubeconfig)
			if err != nil {
				return fmt.Errorf("error building Kubernetes client: %w", err)
			}

			defer client.Close() //nolint:errcheck

			if err = ctrl.watchKubernetesEndpointSlices(ctx, r, logger, client); err != nil {
				return err
			}

			return nil
		}
		if err = watch(); err != nil {
			return err
		}
	}
}

//nolint:gocyclo
func (ctrl *EndpointController) updateEndpointsResource(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	object *discoveryv1.EndpointSlice,
) error {
	var addrs []netip.Addr

	for _, endpoint := range object.Endpoints {
		for _, addr := range endpoint.Addresses {
			ip, err := netip.ParseAddr(addr)
			if err == nil {
				addrs = append(addrs, ip)
			}
		}
	}

	slices.SortFunc(addrs, func(a, b netip.Addr) int { return a.Compare(b) })

	if err := safe.WriterModify(ctx,
		r,
		k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, k8s.ControlPlaneAPIServerEndpointsID),
		func(r *k8s.Endpoint) error {
			if !slices.Equal(r.TypedSpec().Addresses, addrs) {
				logger.Debug("updated controlplane endpoints", zap.Any("endpoints", addrs))
			}

			var addrIPv4, addrIPv6 []netip.Addr

			for _, addr := range r.TypedSpec().Addresses {
				switch {
				case addr.Is4():
					addrIPv4 = append(addrIPv4, addr)

				case addr.Is6():
					addrIPv6 = append(addrIPv6, addr)
				}
			}

			switch object.AddressType {
			case discoveryv1.AddressTypeIPv4:
				addrIPv4 = addrs

			case discoveryv1.AddressTypeIPv6:
				addrIPv6 = addrs

			case discoveryv1.AddressTypeFQDN:
				fallthrough

			default:
				// ignore all other cases
			}

			r.TypedSpec().Addresses = slices.Concat(addrIPv4, addrIPv6)

			return nil
		},
	); err != nil {
		return fmt.Errorf("error updating endpoints: %w", err)
	}

	return nil
}

func (ctrl *EndpointController) watchKubernetesEndpointSlices(ctx context.Context, r controller.Runtime, logger *zap.Logger, client *kubernetes.Client) error {
	// abort the watch on any return from this function
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	notifyCh, watchCloser, err := kubernetesEndpointSliceWatcher(ctx, logger, client)
	if err != nil {
		return fmt.Errorf("error watching Kubernetes endpoint slice: %w", err)
	}

	defer func() {
		cancel() // cancel the context before stopping the watcher

		watchCloser()
	}()

	for {
		select {
		case endpoints := <-notifyCh:
			if err = ctrl.updateEndpointsResource(ctx, r, logger, endpoints); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			// something got updated, probably kubeconfig, restart the watch
			r.QueueReconcile()

			return nil
		}
	}
}

func kubernetesEndpointSliceWatcher(ctx context.Context, logger *zap.Logger, client *kubernetes.Client) (chan *discoveryv1.EndpointSlice, func(), error) {
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		client.Clientset, constants.KubernetesInformerDefaultResyncPeriod,
		informers.WithNamespace(corev1.NamespaceDefault),
		informers.WithTweakListOptions(func(options *v1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", "kubernetes").String()
		}),
	)

	notifyCh := make(chan *discoveryv1.EndpointSlice, 1)

	informer := informerFactory.Discovery().V1().EndpointSlices().Informer()

	if err := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		logger.Error("kubernetes endpoint watch error", zap.Error(err))
	}); err != nil {
		return nil, nil, fmt.Errorf("error setting watch error handler: %w", err)
	}

	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { notifyCh <- obj.(*discoveryv1.EndpointSlice) },
		DeleteFunc: func(_ any) { notifyCh <- &discoveryv1.EndpointSlice{} },
		UpdateFunc: func(_, obj any) { notifyCh <- obj.(*discoveryv1.EndpointSlice) },
	}); err != nil {
		return nil, nil, fmt.Errorf("error adding watch event handler: %w", err)
	}

	informerFactory.Start(ctx.Done())

	return notifyCh, informerFactory.Shutdown, nil
}
