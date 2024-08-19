// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
func (ctrl *ManifestController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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
					return k8sadapter.Manifest(r).SetYAML(renderedManifest.data)
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
	data []byte
}

func jsonify(input string) (string, error) {
	out, err := json.Marshal(input)

	return string(out), err
}

func (ctrl *ManifestController) render(cfg k8s.BootstrapManifestsConfigSpec, scrt *secrets.KubernetesRootSpec) ([]renderedManifest, error) {
	templateConfig := struct {
		k8s.BootstrapManifestsConfigSpec

		Secrets *secrets.KubernetesRootSpec

		KubernetesTalosAPIServiceName      string
		KubernetesTalosAPIServiceNamespace string

		ApidPort int

		TalosServiceAccount TalosServiceAccount
	}{
		BootstrapManifestsConfigSpec: cfg,
		Secrets:                      scrt,

		KubernetesTalosAPIServiceName:      constants.KubernetesTalosAPIServiceName,
		KubernetesTalosAPIServiceNamespace: constants.KubernetesTalosAPIServiceNamespace,

		ApidPort: constants.ApidPort,

		TalosServiceAccount: TalosServiceAccount{
			Group:            constants.ServiceAccountResourceGroup,
			Version:          constants.ServiceAccountResourceVersion,
			Kind:             constants.ServiceAccountResourceKind,
			ResourceSingular: constants.ServiceAccountResourceSingular,
			ResourcePlural:   constants.ServiceAccountResourcePlural,
			ShortName:        constants.ServiceAccountResourceShortName,
		},
	}

	type manifestDesc struct {
		name     string
		template string
	}

	defaultManifests := []manifestDesc{
		{"00-kubelet-bootstrapping-token", kubeletBootstrappingToken},
		{"01-csr-node-bootstrap", csrNodeBootstrapTemplate},
		{"01-csr-approver-role-binding", csrApproverRoleBindingTemplate},
		{"01-csr-renewal-role-binding", csrRenewalRoleBindingTemplate},
		{"11-kube-config-in-cluster", kubeConfigInClusterTemplate},
	}

	if cfg.CoreDNSEnabled {
		defaultManifests = slices.Concat(defaultManifests,
			[]manifestDesc{
				{"11-core-dns", coreDNSTemplate},
				{"11-core-dns-svc", coreDNSSvcTemplate},
			},
		)
	}

	if cfg.FlannelEnabled {
		defaultManifests = append(defaultManifests,
			manifestDesc{"05-flannel", flannelTemplate})
	}

	if cfg.ProxyEnabled {
		defaultManifests = append(defaultManifests,
			manifestDesc{"10-kube-proxy", kubeProxyTemplate})
	}

	if cfg.PodSecurityPolicyEnabled {
		defaultManifests = append(defaultManifests,
			manifestDesc{"03-default-pod-security-policy", podSecurityPolicy},
		)
	}

	if cfg.TalosAPIServiceEnabled {
		defaultManifests = slices.Concat(defaultManifests,
			[]manifestDesc{
				{"12-talos-api-service", talosAPIService},
				{"13-talos-service-account-crd", talosServiceAccountCRDTemplate},
			},
		)
	}

	manifests := make([]renderedManifest, len(defaultManifests))

	for i := range defaultManifests {
		tmpl, err := template.New(defaultManifests[i].name).
			Funcs(template.FuncMap{
				"json":     jsonify,
				"join":     strings.Join,
				"contains": strings.Contains,
			}).
			Parse(defaultManifests[i].template)
		if err != nil {
			return nil, fmt.Errorf("error parsing manifest template %q: %w", defaultManifests[i].name, err)
		}

		var buf bytes.Buffer

		if err = tmpl.Execute(&buf, &templateConfig); err != nil {
			return nil, fmt.Errorf("error executing template %q: %w", defaultManifests[i].name, err)
		}

		manifests[i].name = defaultManifests[i].name
		manifests[i].data = buf.Bytes()
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
