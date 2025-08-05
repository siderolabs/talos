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
	"k8s.io/apimachinery/pkg/runtime"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/k8stemplates"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// ManifestController renders manifests based on templates and config/secrets.
type ManifestController struct{}

// Name implements controller.Controller interface.
func (ctrl *ManifestController) Name() string {
	return "k8s.ManifestController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ManifestController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.BootstrapManifestsConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesRootType,
			ID:        optional.Some(secrets.KubernetesRootID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ManifestController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.ManifestType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ManifestController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		configResource, err := safe.ReaderGetByID[*k8s.BootstrapManifestsConfig](ctx, r, k8s.BootstrapManifestsConfigID)
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error tearing down: %w", err)
				}

				continue
			}

			return err
		}

		config := *configResource.TypedSpec()

		secretsResources, err := safe.ReaderGetByID[*secrets.KubernetesRoot](ctx, r, secrets.KubernetesRootID)
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error tearing down: %w", err)
				}

				continue
			}

			return err
		}

		secrets := secretsResources.TypedSpec()

		renderedManifests, err := ctrl.render(config, secrets)
		if err != nil {
			return err
		}

		for _, renderedManifest := range renderedManifests {
			if err = safe.WriterModify(ctx, r, k8s.NewManifest(k8s.ControlPlaneNamespaceName, renderedManifest.name),
				func(r *k8s.Manifest) error {
					return k8sadapter.Manifest(r).SetObjects(renderedManifest.objs)
				}); err != nil {
				return fmt.Errorf("error updating manifest %q: %w", renderedManifest.name, err)
			}
		}

		// remove any manifests which weren't rendered
		manifests, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.ManifestType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing manifests: %w", err)
		}

		manifestsToDelete := make(map[string]struct{}, len(manifests.Items))

		for _, manifest := range manifests.Items {
			if manifest.Metadata().Owner() != ctrl.Name() {
				continue
			}

			manifestsToDelete[manifest.Metadata().ID()] = struct{}{}
		}

		for _, renderedManifest := range renderedManifests {
			delete(manifestsToDelete, renderedManifest.name)
		}

		for id := range manifestsToDelete {
			if err = r.Destroy(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.ManifestType, id, resource.VersionUndefined)); err != nil {
				return fmt.Errorf("error cleaning up manifests: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}

type renderedManifest struct {
	name string
	objs []runtime.Object
}

func (ctrl *ManifestController) render(cfg k8s.BootstrapManifestsConfigSpec, scrt *secrets.KubernetesRootSpec) ([]renderedManifest, error) {
	manifests := []renderedManifest{
		{
			"00-kubelet-bootstrapping-token",
			[]runtime.Object{
				k8stemplates.KubeletBootstrapTokenSecret(scrt),
			},
		},
		{
			"01-csr-node-bootstrap",
			[]runtime.Object{
				k8stemplates.CSRNodeBootstrapTemplate(),
			},
		},
		{
			"01-csr-approver-role-binding",
			[]runtime.Object{
				k8stemplates.CSRApproverRoleBindingTemplate(),
			},
		},
		{
			"01-csr-renewal-role-binding",
			[]runtime.Object{
				k8stemplates.CSRRenewalRoleBindingTemplate(),
			},
		},
		{
			"11-kube-config-in-cluster",
			[]runtime.Object{
				k8stemplates.KubeconfigInClusterTemplate(&cfg),
			},
		},
		{
			"11-talos-node-rbac-template",
			[]runtime.Object{
				k8stemplates.TalosNodesRBACClusterRoleBinding(),
				k8stemplates.TalosNodesRBACClusterRole(),
			},
		},
	}

	if cfg.CoreDNSEnabled {
		manifests = append(manifests,
			renderedManifest{
				"11-core-dns",
				[]runtime.Object{
					k8stemplates.CoreDNSServiceAccount(),
					k8stemplates.CoreDNSClusterRole(),
					k8stemplates.CoreDNSClusterRoleBinding(),
					k8stemplates.CoreDNSConfigMap(&cfg),
					k8stemplates.CoreDNSDeployment(&cfg),
				},
			},
			renderedManifest{
				"11-core-dns-svc",
				[]runtime.Object{
					k8stemplates.CoreDNSService(&cfg),
				},
			},
		)
	}

	if cfg.FlannelEnabled {
		manifests = append(manifests,
			renderedManifest{
				"05-flannel",
				[]runtime.Object{
					k8stemplates.FlannelClusterRoleTemplate(),
					k8stemplates.FlannelClusterRoleBindingTemplate(),
					k8stemplates.FlannelServiceAccountTemplate(),
					k8stemplates.FlannelConfigMapTemplate(&cfg),
					k8stemplates.FlannelDaemonSetTemplate(&cfg),
				},
			},
		)
	}

	if cfg.ProxyEnabled {
		manifests = append(manifests,
			renderedManifest{
				"10-kube-proxy",
				[]runtime.Object{
					k8stemplates.KubeProxyDaemonSetTemplate(&cfg),
					k8stemplates.KubeProxyServiceAccount(),
					k8stemplates.KubeProxyClusterRoleBinding(),
				},
			},
		)
	}

	if cfg.PodSecurityPolicyEnabled {
		return nil, fmt.Errorf("pod security policies are not supported anymore, please remove the flag from the configuration")
	}

	if cfg.TalosAPIServiceEnabled {
		manifests = append(manifests,
			renderedManifest{
				"13-talos-service-account-crd",
				[]runtime.Object{
					k8stemplates.TalosServiceAccountCRDTemplate(),
				},
			},
		)
	}

	return manifests, nil
}

//nolint:dupl
func (ctrl *ManifestController) teardownAll(ctx context.Context, r controller.Runtime) error {
	manifests, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.ManifestType, "", resource.VersionUndefined))
	if err != nil {
		return fmt.Errorf("error listing manifests: %w", err)
	}

	for _, manifest := range manifests.Items {
		if manifest.Metadata().Owner() != ctrl.Name() {
			continue
		}

		if err = r.Destroy(ctx, manifest.Metadata()); err != nil {
			return fmt.Errorf("error destroying manifest: %w", err)
		}
	}

	return nil
}

// TalosServiceAccount is a struct used by the template engine which contains the needed variables to
// be able to construct the Talos Service Account CRD.
type TalosServiceAccount struct {
	Group            string
	Version          string
	Kind             string
	ResourceSingular string
	ResourcePlural   string
	ShortName        string
}
