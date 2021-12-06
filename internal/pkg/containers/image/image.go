// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/talos-systems/go-retry/retry"

	containerdrunner "github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Image pull retry settings.
const (
	PullTimeout       = 20 * time.Minute
	PullRetryInterval = 5 * time.Second
)

// Image import retry settings.
const (
	ImportTimeout       = 5 * time.Minute
	ImportRetryInterval = 5 * time.Second
	ImportRetryJitter   = time.Second
)

// PullOption is an option for Pull function.
type PullOption func(*PullOptions)

// PullOptions configure Pull function.
type PullOptions struct {
	SkipIfAlreadyPulled bool
}

// WithSkipIfAlreadyPulled skips pulling if image is already pulled and unpacked.
func WithSkipIfAlreadyPulled() PullOption {
	return func(opts *PullOptions) {
		opts.SkipIfAlreadyPulled = true
	}
}

// Pull is a convenience function that wraps the containerd image pull func with
// retry functionality.
func Pull(ctx context.Context, reg config.Registries, client *containerd.Client, ref string, opt ...PullOption) (img containerd.Image, err error) {
	var opts PullOptions

	for _, o := range opt {
		o(&opts)
	}

	if opts.SkipIfAlreadyPulled {
		img, err = client.GetImage(ctx, ref)
		if err == nil {
			var unpacked bool

			unpacked, err = img.IsUnpacked(ctx, "")
			if err == nil && unpacked {
				return img, nil
			}
		}
	}

	resolver := NewResolver(reg)

	err = retry.Exponential(PullTimeout, retry.WithUnits(PullRetryInterval), retry.WithErrorLogging(true)).Retry(func() error {
		if img, err = client.Pull(
			ctx,
			ref,
			containerd.WithPullUnpack,
			containerd.WithResolver(resolver),
			containerd.WithChildLabelMap(images.ChildGCLabelsFilterLayers),
		); err != nil {
			err = fmt.Errorf("failed to pull image %q: %w", ref, err)

			if errdefs.IsNotFound(err) || errdefs.IsCanceled(err) {
				return err
			}

			return retry.ExpectedError(err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return img, nil
}

// Import is a convenience function that wraps containerd image import with retries.
func Import(ctx context.Context, imagePath, indexName string) error {
	importer := containerdrunner.NewImporter(constants.SystemContainerdNamespace, containerdrunner.WithContainerdAddress(constants.SystemContainerdAddress))

	return retry.Exponential(ImportTimeout, retry.WithUnits(ImportRetryInterval), retry.WithJitter(ImportRetryJitter), retry.WithErrorLogging(true)).Retry(func() error {
		err := retry.ExpectedError(importer.Import(ctx, &containerdrunner.ImportRequest{
			Path: imagePath,
			Options: []containerd.ImportOpt{
				containerd.WithIndexName(indexName),
			},
		}))

		if err != nil && os.IsNotExist(err) {
			return err
		}

		return retry.ExpectedError(err)
	})
}
