// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.etcd.io/etcd/client/v3/concurrency"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/k8s"
	"github.com/talos-systems/talos/pkg/resources/secrets"
	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
)

// ManifestApplyController applies manifests via control plane endpoint.
type ManifestApplyController struct{}

// Name implements controller.Controller interface.
func (ctrl *ManifestApplyController) Name() string {
	return "k8s.ManifestApplyController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ManifestApplyController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesType,
			ID:        pointer.ToString(secrets.KubernetesID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.ManifestType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.BootstrapStatusType,
			ID:        pointer.ToString(v1alpha1.BootstrapStatusID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ManifestApplyController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.ManifestStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ManifestApplyController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		secretsResources, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesType, secrets.KubernetesID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		secrets := secretsResources.(*secrets.Kubernetes).Certs()

		bootstrapStatus, err := r.Get(ctx, v1alpha1.NewBootstrapStatus().Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if bootstrapStatus.(*v1alpha1.BootstrapStatus).Status().SelfHostedControlPlane {
			logger.Print("skipped as running self-hosted control plane")

			continue
		}

		manifests, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.ManifestType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing manifests: %w", err)
		}

		sort.Slice(manifests.Items, func(i, j int) bool {
			return manifests.Items[i].Metadata().ID() < manifests.Items[j].Metadata().ID()
		})

		if len(manifests.Items) > 0 {
			var (
				kubeconfig *rest.Config
				dc         *discovery.DiscoveryClient
				dyn        dynamic.Interface
			)

			kubeconfig, err = clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
				return clientcmd.Load([]byte(secrets.AdminKubeconfig))
			})
			if err != nil {
				return fmt.Errorf("error loading kubeconfig: %w", err)
			}

			kubeconfig.WarningHandler = rest.NewWarningWriter(logger.Writer(), rest.WarningWriterOptions{
				Deduplicate: true,
			})

			dc, err = discovery.NewDiscoveryClientForConfig(kubeconfig)
			if err != nil {
				return fmt.Errorf("error building discovery client: %w", err)
			}

			mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))

			dyn, err = dynamic.NewForConfig(kubeconfig)
			if err != nil {
				return fmt.Errorf("error building dynamic client: %w", err)
			}

			if err = ctrl.etcdLock(ctx, logger, func() error {
				return ctrl.apply(ctx, logger, mapper, dyn, manifests)
			}); err != nil {
				return err
			}
		}

		if err = r.Modify(ctx, k8s.NewManifestStatus(k8s.ControlPlaneNamespaceName), func(r resource.Resource) error {
			status := r.(*k8s.ManifestStatus).Status()

			status.ManifestsApplied = make([]string, 0, len(manifests.Items))

			for _, manifest := range manifests.Items {
				status.ManifestsApplied = append(status.ManifestsApplied, manifest.Metadata().ID())
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error updating manifest status: %w", err)
		}
	}
}

func (ctrl *ManifestApplyController) etcdLock(ctx context.Context, logger *log.Logger, f func() error) error {
	etcdClient, err := etcd.NewLocalClient()
	if err != nil {
		return fmt.Errorf("error creating etcd client: %w", err)
	}

	defer etcdClient.Close() //nolint:errcheck

	session, err := concurrency.NewSession(etcdClient.Client)
	if err != nil {
		return fmt.Errorf("error creating etcd session: %w", err)
	}

	defer session.Close() //nolint:errcheck

	mutex := concurrency.NewMutex(session, constants.EtcdTalosManifestApplyMutex)

	logger.Printf("waiting for mutex")

	if err := mutex.Lock(ctx); err != nil {
		return fmt.Errorf("error acquiring mutex: %w", err)
	}

	logger.Printf("mutex acquired")

	defer mutex.Unlock(ctx) //nolint:errcheck

	return f()
}

//nolint:gocyclo
func (ctrl *ManifestApplyController) apply(ctx context.Context, logger *log.Logger, mapper *restmapper.DeferredDiscoveryRESTMapper, dyn dynamic.Interface, manifests resource.List) error {
	// flatten list of objects to be applied
	objects := make([]*unstructured.Unstructured, 0, len(manifests.Items))

	for _, manifest := range manifests.Items {
		objects = append(objects, manifest.(*k8s.Manifest).Objects()...)
	}

	// sort the list so that namespaces come first, followed by CRDs and everything else after that
	sort.SliceStable(objects, func(i, j int) bool {
		objL := objects[i]
		objR := objects[j]

		gvkL := objL.GroupVersionKind()
		gvkR := objR.GroupVersionKind()

		if isNamespace(gvkL) {
			if !isNamespace(gvkR) {
				return true
			}

			return objL.GetName() < objR.GetName()
		}

		if isNamespace(gvkR) {
			return false
		}

		if isCRD(gvkL) {
			if !isCRD(gvkR) {
				return true
			}

			return objL.GetName() < objR.GetName()
		}

		if isCRD(gvkR) {
			return false
		}

		return false
	})

	for _, obj := range objects {
		gvk := obj.GroupVersionKind()
		objName := fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, obj.GetName())

		mapping, err := mapper.RESTMapping(obj.GroupVersionKind().GroupKind(), obj.GroupVersionKind().Version)
		if err != nil {
			return fmt.Errorf("error creating mapping for object %s: %w", objName, err)
		}

		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// namespaced resources should specify the namespace
			dr = dyn.Resource(mapping.Resource).Namespace(obj.GetNamespace())
		} else {
			// for cluster-wide resources
			dr = dyn.Resource(mapping.Resource)
		}

		_, err = dr.Get(ctx, obj.GetName(), metav1.GetOptions{})
		if err == nil {
			// already exists
			continue
		}

		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error checking resource existence: %w", err)
		}

		_, err = dr.Create(ctx, obj, metav1.CreateOptions{
			FieldManager: "talos",
		})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// later on we might want to do something here, e.g. do server-side apply, for now do nothing
			} else {
				return fmt.Errorf("error creating %s: %w", objName, err)
			}
		} else {
			logger.Printf("created %s", objName)
		}
	}

	return nil
}

func isNamespace(gvk schema.GroupVersionKind) bool {
	return gvk.Kind == "Namespace" && gvk.Version == "v1"
}

func isCRD(gvk schema.GroupVersionKind) bool {
	return gvk.Kind == "CustomResourceDefinition" && gvk.Group == "apiextensions.k8s.io"
}
