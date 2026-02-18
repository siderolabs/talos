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
	gokube "github.com/siderolabs/go-kubernetes/kubernetes/manifests"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubectl/pkg/cmd/util"
	"sigs.k8s.io/cli-utils/pkg/apis/actuation"
	"sigs.k8s.io/cli-utils/pkg/inventory"
	"sigs.k8s.io/cli-utils/pkg/object"

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
			var (
				kubeconfig *rest.Config
				dyn        dynamic.Interface
			)

			kubeconfig, err = clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
				return clientcmd.Load([]byte(secrets.LocalhostAdminKubeconfig))
			})
			if err != nil {
				return fmt.Errorf("error loading kubeconfig: %w", err)
			}

			kubeconfig.WarningHandler = rest.NewWarningWriter(logging.NewWriter(logger, zapcore.WarnLevel), rest.WarningWriterOptions{
				Deduplicate: true,
			})

			discoveryClient, err := discovery.NewDiscoveryClientForConfig(kubeconfig)
			if err != nil {
				return fmt.Errorf("error building discovery client: %w", err)
			}

			dc := memory.NewMemCacheClient(discoveryClient)

			mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))

			dyn, err = dynamic.NewForConfig(kubeconfig)
			if err != nil {
				return fmt.Errorf("error building dynamic client: %w", err)
			}

			if err = etcd.WithLock(ctx, constants.EtcdTalosManifestApplyMutex, logger, func() error {
				inventoryClient, inv, err := getInventory(ctx, kubeconfig, mapper, dc)
				if err != nil {
					return err
				}

				applyErr := ctrl.apply(ctx, logger, mapper, dyn, manifests, inv)

				// update inventory even if the apply process failed half way through
				err = inventoryClient.CreateOrUpdate(ctx, inv, inventory.UpdateOptions{})
				if err != nil {
					err = fmt.Errorf("updating inventory failed: %w", err)
				}

				return errors.Join(applyErr, err)
			}); err != nil {
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

//nolint:gocyclo,cyclop
func (ctrl *ManifestApplyController) apply(
	ctx context.Context,
	logger *zap.Logger,
	mapper *restmapper.DeferredDiscoveryRESTMapper,
	dyn dynamic.Interface,
	manifests resource.List,
	inv inventory.Inventory,
) error {
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
		objMeta := object.UnstructuredToObjMetadata(obj)

		// check if the resource is already in the inventory, if so, skip applying it
		if inv.GetObjectRefs().Contains(objMeta) {
			continue
		}

		gvk := obj.GroupVersionKind()
		objName := fmt.Sprintf("%s/%s/%s/%s", gvk.Group, gvk.Version, gvk.Kind, obj.GetName())

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
				return fmt.Errorf("error creating mapping for object %s: %w", objName, err)
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
			inventoryAdd(inv, objMeta, obj, actuation.ActuationSucceeded)

			continue
		}

		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("error checking resource existence: %w", err)
		}

		// Set inventory annotation.
		annotations := obj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		inventoryAnnotation, inventoryAnnotationSet := annotations["config.k8s.io/owning-inventory"]

		if inventoryAnnotationSet && inventoryAnnotation != constants.KubernetesBootstrapManifestsInventoryName {
			return fmt.Errorf("unexpected foreign inventory annotation on %s ", objName)
		}

		annotations["config.k8s.io/owning-inventory"] = constants.KubernetesBootstrapManifestsInventoryName
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
				return fmt.Errorf("error creating %s: %w", objName, err)
			}
		} else {
			logger.Sugar().Infof("created %s", objName)

			inventoryAdd(inv, objMeta, obj, actuation.ActuationSucceeded)
		}
	}

	return multiErr.ErrorOrNil()
}

func getInventory(
	ctx context.Context,
	kubeconfig *rest.Config,
	mapper *restmapper.DeferredDiscoveryRESTMapper,
	dc discovery.CachedDiscoveryInterface,
) (inventory.Client, inventory.Inventory, error) {
	clientGetter := gokube.K8sRESTClientGetter{
		RestConfig:      kubeconfig,
		Mapper:          mapper,
		DiscoveryClient: dc,
	}

	factory := util.NewFactory(clientGetter)

	inventoryClient, err := inventory.ConfigMapClientFactory{StatusEnabled: true}.NewClient(factory)
	if err != nil {
		return nil, nil, err
	}

	inventoryInfo := inventory.NewSingleObjectInfo(inventory.ID(
		constants.KubernetesBootstrapManifestsInventoryName),
		types.NamespacedName{
			Namespace: constants.KubernetesInventoryNamespace,
			Name:      constants.KubernetesBootstrapManifestsInventoryName,
		})

	err = gokube.AssureInventory(ctx, inventoryClient, inventoryInfo)
	if err != nil {
		return nil, nil, err
	}

	inv, err := inventoryClient.Get(ctx, inventoryInfo, inventory.GetOptions{})
	if err != nil {
		return nil, nil, err
	}

	return inventoryClient, inv, err
}

func inventoryAdd(inv inventory.Inventory, objMeta object.ObjMetadata, obj *unstructured.Unstructured, actuationStatus actuation.ActuationStatus) {
	inv.SetObjectRefs(slices.Concat(inv.GetObjectRefs(), object.ObjMetadataSet{objMeta}))
	inv.SetObjectStatuses(slices.Concat(inv.GetObjectStatuses(), object.ObjectStatusSet{actuation.ObjectStatus{
		ObjectReference: inventory.ObjectReferenceFromObjMetadata(objMeta),
		Strategy:        actuation.ActuationStrategyApply,
		Actuation:       actuationStatus,
		Reconcile:       actuation.ReconcileUnknown,
		UID:             obj.GetUID(),
		Generation:      obj.GetGeneration(),
	}}))
}

func isNamespace(gvk schema.GroupVersionKind) bool {
	return gvk.Kind == "Namespace" && gvk.Version == "v1"
}

func isCRD(gvk schema.GroupVersionKind) bool {
	return gvk.Kind == "CustomResourceDefinition" && gvk.Group == "apiextensions.k8s.io"
}
