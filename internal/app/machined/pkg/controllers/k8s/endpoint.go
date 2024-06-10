// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
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

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		// unfortunately we can't use Watch or CachedInformer here as system:node role is only allowed verb 'Get'
		endpoints, err := client.CoreV1().Endpoints(corev1.NamespaceDefault).Get(ctx, "kubernetes", v1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error getting endpoints: %w", err)
		}

		if err = ctrl.updateEndpointsResource(ctx, r, logger, endpoints); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-r.EventCh():
		}
	}
}

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

		if err = ctrl.watchKubernetesEndpoint(ctx, r, logger, kubeconfig); err != nil {
			return err
		}
	}
}

func (ctrl *EndpointController) updateEndpointsResource(ctx context.Context, r controller.Runtime, logger *zap.Logger, endpoints *corev1.Endpoints) error {
	var addrs []netip.Addr

	for _, endpoint := range endpoints.Subsets {
		for _, addr := range endpoint.Addresses {
			ip, err := netip.ParseAddr(addr.IP)
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

			r.TypedSpec().Addresses = addrs

			return nil
		},
	); err != nil {
		return fmt.Errorf("error updating endpoints: %w", err)
	}

	return nil
}

func (ctrl *EndpointController) watchKubernetesEndpoint(ctx context.Context, r controller.Runtime, logger *zap.Logger, kubeconfig *rest.Config) error {
	client, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("error building Kubernetes client: %w", err)
	}

	defer client.Close() //nolint:errcheck

	// abort the watch on any return from this function
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	notifyCh, watchCloser, err := kubernetesEndpointWatcher(ctx, logger, client)
	if err != nil {
		return fmt.Errorf("error watching Kubernetes endpoint: %w", err)
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

func kubernetesEndpointWatcher(ctx context.Context, logger *zap.Logger, client *kubernetes.Client) (chan *corev1.Endpoints, func(), error) {
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		client.Clientset, 30*time.Second,
		informers.WithNamespace(corev1.NamespaceDefault),
		informers.WithTweakListOptions(func(options *v1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", "kubernetes").String()
		}),
	)

	notifyCh := make(chan *corev1.Endpoints, 1)

	informer := informerFactory.Core().V1().Endpoints().Informer()

	if err := informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		logger.Error("kubernetes endpoint watch error", zap.Error(err))
	}); err != nil {
		return nil, nil, fmt.Errorf("error setting watch error handler: %w", err)
	}

	if _, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj any) { notifyCh <- obj.(*corev1.Endpoints) },
		DeleteFunc: func(_ any) { notifyCh <- &corev1.Endpoints{} },
		UpdateFunc: func(_, obj any) { notifyCh <- obj.(*corev1.Endpoints) },
	}); err != nil {
		return nil, nil, fmt.Errorf("error adding watch event handler: %w", err)
	}

	informerFactory.Start(ctx.Done())

	return notifyCh, informerFactory.Shutdown, nil
}
