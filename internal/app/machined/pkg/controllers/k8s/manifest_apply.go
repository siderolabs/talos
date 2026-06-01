// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-kubernetes/kubernetes/ssa"
	"github.com/siderolabs/go-kubernetes/kubernetes/ssa/object"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
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
			ID:        optional.Some(secrets.KubernetesID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.ManifestType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("etcd"),
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
func (ctrl *ManifestApplyController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		secretsResources, err := safe.ReaderGetByID[*secrets.Kubernetes](ctx, r, secrets.KubernetesID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		secrets := secretsResources.TypedSpec()

		// wait for etcd to be healthy as controller relies on etcd for locking
		etcdResource, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "etcd")
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !etcdResource.TypedSpec().Healthy {
			continue
		}

		manifests, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.ManifestType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing manifests: %w", err)
		}

		slices.SortFunc(manifests.Items, func(a, b resource.Resource) int {
			return cmp.Compare(a.Metadata().ID(), b.Metadata().ID())
		})

		if len(manifests.Items) > 0 {
			if err = ctrl.applyManifests(ctx, logger, manifests, secrets); err != nil {
				return err
			}
		}

		if err = safe.WriterModify(ctx, r, k8s.NewManifestStatus(k8s.ControlPlaneNamespaceName), func(r *k8s.ManifestStatus) error {
			status := r.TypedSpec()

			status.ManifestsApplied = xslices.Map(manifests.Items, func(m resource.Resource) string {
				return m.Metadata().ID()
			})

			return nil
		}); err != nil {
			return fmt.Errorf("error updating manifest status: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *ManifestApplyController) applyManifests(
	ctx context.Context,
	logger *zap.Logger,
	manifests resource.List,
	secrets *secrets.KubernetesCertsSpec,
) error {
	kubeconfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
		return clientcmd.Load([]byte(secrets.LocalhostAdminKubeconfig))
	})
	if err != nil {
		return fmt.Errorf("error loading kubeconfig: %w", err)
	}

	kubeconfig.WarningHandler = rest.NewWarningWriter(logging.NewWriter(logger, zapcore.WarnLevel), rest.WarningWriterOptions{
		Deduplicate: true,
	})

	httpClient, err := rest.HTTPClientFor(kubeconfig)
	if err != nil {
		return fmt.Errorf("error building HTTP client for kubeconfig: %w", err)
	}

	defer httpClient.CloseIdleConnections()

	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(kubeconfig, httpClient)
	if err != nil {
		return fmt.Errorf("error building discovery client: %w", err)
	}

	dyn, err := dynamic.NewForConfigAndClient(kubeconfig, httpClient)
	if err != nil {
		return fmt.Errorf("error building dynamic client: %w", err)
	}

	dc := memory.NewMemCacheClient(discoveryClient)
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(dc)

	k8sClient, err := kubernetes.NewForConfigAndClient(kubeconfig, httpClient)
	if err != nil {
		return err
	}

	if err = etcd.WithLock(ctx, constants.EtcdTalosManifestApplyMutex, logger, func() error {
		inv, err := ssa.GetInventory(ctx, k8sClient, constants.KubernetesInventoryNamespace, constants.KubernetesBootstrapManifestsInventoryName)
		if err != nil {
			return fmt.Errorf("error getting inventory: %w", err)
		}

		inventoryContents := inv.Get()

		inventoryContents, applyErr := ctrl.apply(ctx, logger, mapper, dyn, manifests, inventoryContents)

		inv.Update(inventoryContents)

		// update inventory even if the apply process failed half way through
		err = inv.Write(ctx)
		if err != nil {
			err = fmt.Errorf("updating inventory failed: %w", err)
		}

		return errors.Join(applyErr, err)
	}); err != nil {
		return err
	}

	return nil
}

//nolint:gocyclo,cyclop
func (ctrl *ManifestApplyController) apply(
	ctx context.Context,
	logger *zap.Logger,
	mapper *restmapper.DeferredDiscoveryRESTMapper,
	dyn dynamic.Interface,
	manifests resource.List,
	inv object.ObjMetadataSet,
) (object.ObjMetadataSet, error) {
	// flatten list of objects to be applied
	objects := xslices.FlatMap(manifests.Items, func(m resource.Resource) []*unstructured.Unstructured {
		return k8sadapter.Manifest(m.(*k8s.Manifest)).Objects()
	})

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

	var multiErr *multierror.Error

	for _, obj := range objects {
		gvk := obj.GroupVersionKind()
		objName := fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, obj.GetName())

		objMeta, err := object.RuntimeToObjMeta(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve object metadata of %q: %w", objName, err)
		}

		// check if the resource is already in the inventory, if so, skip applying it
		if inv.Contains(objMeta) {
			continue
		}

		mapping, err := mapper.RESTMapping(obj.GroupVersionKind().GroupKind(), obj.GroupVersionKind().Version)
		if err != nil {
			switch {
			case apierrors.IsNotFound(err):
				fallthrough
			case apierrors.IsInvalid(err):
				fallthrough
			case meta.IsNoMatchError(err):
				// most probably a problem with the manifest, so we should continue with other manifests
				multiErr = multierror.Append(multiErr, fmt.Errorf("error creating mapping for object %s: %w", objName, err))

				continue
			default:
				// connection errors, etc.; it makes no sense to continue with other manifests
				return nil, fmt.Errorf("error creating mapping for object %s: %w", objName, err)
			}
		}

		var dr dynamic.ResourceInterface

		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// default the namespace if it's not set in the manifest
			if obj.GetNamespace() == "" {
				obj.SetNamespace(corev1.NamespaceDefault)
			}

			// namespaced resources should specify the namespace
			dr = dyn.Resource(mapping.Resource).Namespace(obj.GetNamespace())
		} else {
			// for cluster-wide resources
			dr = dyn.Resource(mapping.Resource)
		}

		_, err = dr.Get(ctx, obj.GetName(), metav1.GetOptions{})
		if err == nil {
			// already exists,
			// backfill the inventory if the resource is missing (to migrate to inventory-based apply)
			inv = inv.Union(object.ObjMetadataSet{objMeta})

			continue
		}

		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("error checking resource existence: %w", err)
		}

		// Set inventory annotation.
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		inventoryAnnotation, inventoryAnnotationSet := annotations[ssa.InventoryAnnotationKey]

		if inventoryAnnotationSet && inventoryAnnotation != constants.KubernetesBootstrapManifestsInventoryName {
			multiErr = multierror.Append(multiErr, fmt.Errorf("unexpected foreign inventory annotation on %s ", objName))

			continue
		}

		annotations[ssa.InventoryAnnotationKey] = constants.KubernetesBootstrapManifestsInventoryName
		obj.SetAnnotations(annotations)

		_, err = dr.Apply(ctx, obj.GetName(), obj, metav1.ApplyOptions{
			FieldManager: constants.KubernetesFieldManagerName,
		})
		if err != nil {
			switch {
			case apierrors.IsMethodNotSupported(err):
				fallthrough
			case apierrors.IsBadRequest(err):
				fallthrough
			case apierrors.IsInvalid(err):
				// resource is malformed, continue with other manifests
				multiErr = multierror.Append(multiErr, fmt.Errorf("error creating %s: %w", objName, err))
			default:
				// connection errors, etc.; it makes no sense to continue with other manifests
				return nil, fmt.Errorf("error creating %s: %w", objName, err)
			}
		} else {
			logger.Sugar().Infof("created %s", objName)

			inv = inv.Union(object.ObjMetadataSet{objMeta})
		}
	}

	return inv, multiErr.ErrorOrNil()
}

func isNamespace(gvk schema.GroupVersionKind) bool {
	return gvk.Kind == "Namespace" && gvk.Version == "v1"
}

func isCRD(gvk schema.GroupVersionKind) bool {
	return gvk.Kind == "CustomResourceDefinition" && gvk.Group == "apiextensions.k8s.io"
}
