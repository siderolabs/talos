// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"sort"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"inet.af/netaddr"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
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
			ID:        pointer.ToString(config.MachineTypeID),
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

		machineTypeRes, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		machineType := machineTypeRes.(*config.MachineType).MachineType()

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
	}
}

//nolint:gocyclo
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

//nolint:gocyclo
func (ctrl *EndpointController) watchEndpointsOnControlPlane(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.ToString(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesType,
			ID:        pointer.ToString(secrets.KubernetesID),
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

		secretsResources, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesType, secrets.KubernetesID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return err
		}

		secrets := secretsResources.(*secrets.Kubernetes).Certs()

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
	addrs := []netaddr.IP{}

	for _, endpoint := range endpoints.Subsets {
		for _, addr := range endpoint.Addresses {
			ip, err := netaddr.ParseIP(addr.IP)
			if err == nil {
				addrs = append(addrs, ip)
			}
		}
	}

	sort.Slice(addrs, func(i, j int) bool { return addrs[i].Compare(addrs[j]) < 0 })

	if err := r.Modify(ctx,
		k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, k8s.ControlPlaneAPIServerEndpointsID),
		func(r resource.Resource) error {
			if !reflect.DeepEqual(r.(*k8s.Endpoint).TypedSpec().Addresses, addrs) {
				logger.Debug("updated controlplane endpoints", zap.Any("endpoints", addrs))
			}

			r.(*k8s.Endpoint).TypedSpec().Addresses = addrs

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

	notifyCh := kubernetesEndpointWatcher(ctx, logger, client)

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

func kubernetesEndpointWatcher(ctx context.Context, logger *zap.Logger, client *kubernetes.Client) chan *corev1.Endpoints {
	// TODO: move me around
	klog.SetOutput(io.Discard)

	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		client.Clientset, 30*time.Second,
		informers.WithNamespace(corev1.NamespaceDefault),
		informers.WithTweakListOptions(func(options *v1.ListOptions) {
			options.FieldSelector = fields.OneTermEqualSelector("metadata.name", "kubernetes").String()
		}),
	)

	notifyCh := make(chan *corev1.Endpoints, 1)

	informer := informerFactory.Core().V1().Endpoints().Informer()
	informer.SetWatchErrorHandler(func(r *cache.Reflector, err error) { //nolint:errcheck
		logger.Error("kubernetes endpoint watch error", zap.Error(err))
	})
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { notifyCh <- obj.(*corev1.Endpoints) },
		DeleteFunc: func(_ interface{}) { notifyCh <- &corev1.Endpoints{} },
		UpdateFunc: func(_, obj interface{}) { notifyCh <- obj.(*corev1.Endpoints) },
	})

	informerFactory.Start(ctx.Done())

	return notifyCh
}
