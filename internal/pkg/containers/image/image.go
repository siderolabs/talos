// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import (
	"context"
	"fmt"
	stdlog "log"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/distribution/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/siderolabs/go-retry/retry"
	"github.com/sirupsen/logrus"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// Image pull retry settings.
const (
	PullTimeout       = 20 * time.Minute
	PullRetryInterval = 5 * time.Second
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

// RegistriesBuilder is a function that returns registries configuration.
type RegistriesBuilder = func(context.Context) (config.Registries, error)

// Pull is a convenience function that wraps the containerd image pull func with
// retry functionality.
//
//nolint:gocyclo
func Pull(ctx context.Context, registryBuilder RegistriesBuilder, client *containerd.Client, ref string, opt ...PullOption) (img containerd.Image, err error) {
	var opts PullOptions

	for _, o := range opt {
		o(&opts)
	}

	namedRef, err := reference.ParseDockerRef(ref)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference %q: %w", ref, err)
	}

	// normalize reference
	ref = namedRef.String()

	if opts.SkipIfAlreadyPulled {
		img, err = client.GetImage(ctx, ref)
		if err == nil {
			var unpacked bool

			unpacked, err = img.IsUnpacked(ctx, "")
			if err == nil && unpacked {
				if err = manageAliases(ctx, client, namedRef, img); err == nil {
					return img, nil
				}
			}
		}
	}

	containerdLogger := logrus.New()
	containerdLogger.Out = stdlog.Default().Writer()
	containerdLogger.Formatter = &logrus.TextFormatter{
		DisableColors:    true,
		DisableQuote:     true,
		DisableTimestamp: true,
	}

	ctx = log.WithLogger(ctx, containerdLogger.WithField("image", ref))

	err = retry.Exponential(PullTimeout, retry.WithUnits(PullRetryInterval), retry.WithErrorLogging(true)).RetryWithContext(ctx, func(ctx context.Context) error {
		registriesConfig, err := registryBuilder(ctx)
		if err != nil {
			return fmt.Errorf("failed to get configured registries: %w", err)
		}

		resolver := NewResolver(registriesConfig)

		if img, err = client.Pull(
			ctx,
			ref,
			containerd.WithPullUnpack,
			containerd.WithResolver(resolver),
			containerd.WithChildLabelMap(images.ChildGCLabelsFilterLayers),
		); err != nil {
			err = fmt.Errorf("failed to pull image %q: %w", ref, err)

			if errdefs.IsNotFound(err) {
				return err
			}

			return retry.ExpectedError(err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if err = manageAliases(ctx, client, namedRef, img); err != nil {
		return nil, err
	}

	return img, nil
}

func manageAliases(ctx context.Context, client *containerd.Client, namedRef reference.Named, img containerd.Image) error {
	// re-tag pulled image
	imageDigest := img.Target().Digest.String()

	refs := []string{imageDigest}

	if _, ok := namedRef.(reference.NamedTagged); ok {
		refs = append(refs, namedRef.String())
	}

	if _, ok := namedRef.(reference.Canonical); ok {
		refs = append(refs, namedRef.String())
	} else {
		refs = append(refs, namedRef.Name()+"@"+imageDigest)
	}

	for _, newRef := range refs {
		if err := createAlias(ctx, client, newRef, img.Target()); err != nil {
			return err
		}
	}

	return nil
}

func createAlias(ctx context.Context, client *containerd.Client, name string, desc ocispec.Descriptor) error {
	img := images.Image{
		Name:   name,
		Target: desc,
	}

	oldImg, err := client.ImageService().Create(ctx, img)
	if err == nil || !errdefs.IsAlreadyExists(err) {
		return err
	}

	if oldImg.Target.Digest == img.Target.Digest {
		return nil
	}

	_, err = client.ImageService().Update(ctx, img, "target")

	return err
}
