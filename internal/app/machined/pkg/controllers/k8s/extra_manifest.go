// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-getter/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/pkg/httpdefaults"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ExtraManifestController renders manifests based on templates and config/secrets.
type ExtraManifestController struct{}

// Name implements controller.Controller interface.
func (ctrl *ExtraManifestController) Name() string {
	return "k8s.ExtraManifestController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ExtraManifestController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.ControlPlaneNamespaceName,
			Type:      k8s.ExtraManifestsConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.StatusType,
			ID:        optional.Some(network.StatusID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ExtraManifestController) Outputs() []controller.Output {
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
func (ctrl *ExtraManifestController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// wait for network to be ready as networking is required to download extra manifests
		networkResource, err := safe.ReaderGetByID[*network.Status](ctx, r, network.StatusID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		networkStatus := networkResource.TypedSpec()

		if !(networkStatus.AddressReady && networkStatus.ConnectivityReady) {
			continue
		}

		configResource, err := safe.ReaderGetByID[*k8s.ExtraManifestsConfig](ctx, r, k8s.ExtraManifestsConfigID)
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

		var multiErr *multierror.Error

		presentManifests := map[resource.ID]struct{}{}

		for _, manifest := range config.ExtraManifests {
			var id resource.ID

			id, err = ctrl.process(ctx, r, logger, manifest)
			if err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			presentManifests[id] = struct{}{}
		}

		if multiErr.ErrorOrNil() != nil {
			return multiErr.ErrorOrNil()
		}

		allManifests, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.ManifestType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing extra manifests: %w", err)
		}

		for _, manifest := range allManifests.Items {
			if manifest.Metadata().Owner() != ctrl.Name() {
				continue
			}

			if _, exists := presentManifests[manifest.Metadata().ID()]; !exists {
				if err = r.Destroy(ctx, manifest.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up extra manifest: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *ExtraManifestController) process(ctx context.Context, r controller.Runtime, logger *zap.Logger, manifest k8s.ExtraManifest) (id resource.ID, err error) {
	id = fmt.Sprintf("%s-%s", manifest.Priority, manifest.Name)

	// inline manifests don't require download
	if manifest.InlineManifest != "" {
		return id, ctrl.processInline(ctx, r, manifest, id)
	}

	return id, ctrl.processURL(ctx, r, logger, manifest, id)
}

func (ctrl *ExtraManifestController) processURL(ctx context.Context, r controller.Runtime, logger *zap.Logger, manifest k8s.ExtraManifest, id resource.ID) (err error) {
	var tmpDir string

	tmpDir, err = os.MkdirTemp("", "talos")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tmpDir) //nolint:errcheck

	// I wish we never used go-getter package, as it doesn't allow downloading into memory.
	// But there's not much we can do about it right now, as it supports lots of magic
	// users might rely upon.

	// Disable netrc since we don't have getent installed, and most likely
	// never will.
	httpGetter := &getter.HttpGetter{
		Netrc: false,
		Client: &http.Client{
			Transport: httpdefaults.PatchTransport(cleanhttp.DefaultTransport()),
		},
	}

	httpGetter.Header = make(http.Header)

	for k, v := range manifest.ExtraHeaders {
		httpGetter.Header.Add(k, v)
	}

	client := &getter.Client{
		Getters: []getter.Getter{
			httpGetter,
		},
	}

	dst := filepath.Join(tmpDir, "manifest.yaml")

	if _, err = client.Get(ctx, &getter.Request{
		Src:     manifest.URL,
		Dst:     dst,
		Pwd:     tmpDir,
		GetMode: getter.ModeFile,
	}); err != nil {
		err = fmt.Errorf("error downloading %q: %w", manifest.URL, err)

		return err
	}

	logger.Sugar().Infof("downloaded manifest %q", manifest.URL)

	var contents []byte

	contents, err = os.ReadFile(dst)
	if err != nil {
		return err
	}

	if err = safe.WriterModify(ctx, r, k8s.NewManifest(k8s.ControlPlaneNamespaceName, id),
		func(r *k8s.Manifest) error {
			return k8sadapter.Manifest(r).SetYAML(contents)
		}); err != nil {
		err = fmt.Errorf("error updating manifests: %w", err)

		return err
	}

	return nil
}

func (ctrl *ExtraManifestController) processInline(ctx context.Context, r controller.Runtime, manifest k8s.ExtraManifest, id resource.ID) error {
	err := safe.WriterModify(
		ctx,
		r,
		k8s.NewManifest(k8s.ControlPlaneNamespaceName, id),
		func(r *k8s.Manifest) error {
			return k8sadapter.Manifest(r).SetYAML([]byte(manifest.InlineManifest))
		},
	)
	if err != nil {
		return fmt.Errorf("error updating manifests: %w", err)
	}

	return nil
}

//nolint:dupl
func (ctrl *ExtraManifestController) teardownAll(ctx context.Context, r controller.Runtime) error {
	manifests, err := r.List(ctx, resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.ManifestType, "", resource.VersionUndefined))
	if err != nil {
		return fmt.Errorf("error listing extra manifests: %w", err)
	}

	for _, manifest := range manifests.Items {
		if manifest.Metadata().Owner() != ctrl.Name() {
			continue
		}

		if err = r.Destroy(ctx, manifest.Metadata()); err != nil {
			return fmt.Errorf("error destroying extra manifest: %w", err)
		}
	}

	return nil
}
