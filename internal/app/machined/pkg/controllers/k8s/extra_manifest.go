// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/AlekSi/pointer"
	"github.com/hashicorp/go-getter"
	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"

	"github.com/talos-systems/talos/internal/app/machined/pkg/resources/config"
	"github.com/talos-systems/talos/internal/app/machined/pkg/resources/k8s"
	"github.com/talos-systems/talos/internal/app/machined/pkg/resources/v1alpha1"
)

// ExtraManifestController renders manifests based on templates and config/secrets.
type ExtraManifestController struct{}

// Name implements controller.Controller interface.
func (ctrl *ExtraManifestController) Name() string {
	return "k8s.ExtraManifestController"
}

// ManagedResources implements controller.Controller interface.
func (ctrl *ExtraManifestController) ManagedResources() (resource.Namespace, resource.Type) {
	return k8s.ExtraNamespaceName, k8s.ManifestType
}

// Run implements controller.Controller interface.
//
//nolint: gocyclo
func (ctrl *ExtraManifestController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	if err := r.UpdateDependencies([]controller.Dependency{
		{
			Namespace: config.NamespaceName,
			Type:      config.K8sControlPlaneType,
			ID:        pointer.ToString(config.K8sExtraManifestsID),
			Kind:      controller.DependencyWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.ToString("networkd"),
			Kind:      controller.DependencyWeak,
		},
	}); err != nil {
		return fmt.Errorf("error setting up dependencies: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// wait for networkd to be healthy as networking is required to download extra manifests
		networkdResource, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "networkd", resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !networkdResource.(*v1alpha1.Service).Healthy() {
			continue
		}

		configResource, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.K8sControlPlaneType, config.K8sExtraManifestsID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error tearing down: %w", err)
				}

				continue
			}

			return err
		}

		config := configResource.(*config.K8sControlPlane).ExtraManifests()

		var multiErr *multierror.Error

		presentManifests := map[resource.ID]struct{}{}

		for _, manifest := range config.ExtraManifests {
			var id resource.ID

			id, err = ctrl.download(ctx, r, logger, manifest)
			if err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			presentManifests[id] = struct{}{}
		}

		if multiErr.ErrorOrNil() != nil {
			return multiErr.ErrorOrNil()
		}

		allManifests, err := r.List(ctx, resource.NewMetadata(k8s.ExtraNamespaceName, k8s.ManifestType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing extra manifests: %w", err)
		}

		for _, manifest := range allManifests.Items {
			if _, exists := presentManifests[manifest.Metadata().ID()]; !exists {
				if err = r.Destroy(ctx, manifest.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up extra manifest: %w", err)
				}
			}
		}
	}
}

func (ctrl *ExtraManifestController) download(ctx context.Context, r controller.Runtime, logger *log.Logger, manifest config.ExtraManifest) (id resource.ID, err error) {
	id = fmt.Sprintf("%s-%s", manifest.Priority, manifest.URL)

	var tmpDir string

	tmpDir, err = ioutil.TempDir("", "talos")
	if err != nil {
		return
	}

	defer os.RemoveAll(tmpDir) //nolint: errcheck

	// I wish we never used go-getter package, as it doesn't allow downloading into memory.
	// But there's not much we can do about it right now, as it supports lots of magic
	// users might rely upon now.

	// Disable netrc since we don't have getent installed, and most likely
	// never will.
	httpGetter := &getter.HttpGetter{
		Netrc:  false,
		Client: http.DefaultClient,
	}

	httpGetter.Header = make(http.Header)

	for k, v := range manifest.ExtraHeaders {
		httpGetter.Header.Add(k, v)
	}

	getter.Getters["http"] = httpGetter
	getter.Getters["https"] = httpGetter

	client := &getter.Client{
		Ctx:     ctx,
		Src:     manifest.URL,
		Dst:     filepath.Join(tmpDir, "manifest.yaml"),
		Pwd:     tmpDir,
		Mode:    getter.ClientModeFile,
		Options: []getter.ClientOption{},
	}

	if err = client.Get(); err != nil {
		err = fmt.Errorf("error downloading %q: %w", manifest.URL, err)

		return
	}

	logger.Printf("downloaded manifest %q", manifest.URL)

	var contents []byte

	contents, err = ioutil.ReadFile(client.Dst)
	if err != nil {
		return
	}

	if err = r.Update(ctx, k8s.NewManifest(k8s.ExtraNamespaceName, id),
		func(r resource.Resource) error {
			return r.(*k8s.Manifest).SetYAML(contents)
		}); err != nil {
		err = fmt.Errorf("error updating manifests: %w", err)

		return
	}

	return id, nil
}

func (ctrl *ExtraManifestController) teardownAll(ctx context.Context, r controller.Runtime) error {
	manifests, err := r.List(ctx, resource.NewMetadata(k8s.ExtraNamespaceName, k8s.ManifestType, "", resource.VersionUndefined))
	if err != nil {
		return fmt.Errorf("error listing extra manifests: %w", err)
	}

	for _, manifest := range manifests.Items {
		if err = r.Destroy(ctx, manifest.Metadata()); err != nil {
			return fmt.Errorf("error destroying extra manifest: %w", err)
		}
	}

	return nil
}
