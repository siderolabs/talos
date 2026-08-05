// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import (
	"context"
	"errors"
	"fmt"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/snapshotters"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/distribution/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-retry/retry"
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/containers/image/progress"
	"github.com/siderolabs/talos/internal/pkg/containers/image/verify"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Error tags used internally to identify retriable errors.
type (
	imageNotFoundTag                    struct{}
	imageSignatureVerificationFailedTag struct{}
	imageTerminalErrorTag               struct{}
)

// Pull performs a single attempt to pull an image from a registry.
//
// If configured, the pull is skipped if the image is already present and unpacked in the containerd client.
// The ctx should have a containerd namespace on it already.
func Pull(
	ctx context.Context,
	registryBuilder RegistriesBuilder,
	resources state.State,
	client *containerd.Client,
	ref string,
	opt ...PullOption,
) (img containerd.Image, err error) {
	opts := DefaultPullOptions()

	for _, o := range opt {
		o(&opts)
	}

	namedRef, err := reference.ParseDockerRef(ref)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference %q: %w", ref, err)
	}

	return pullInternal(ctx, registryBuilder, resources, client, namedRef, opts)
}

// PullWithRetriesAndTimeout performs a pull with retries and timeout.
//
// It is a wrapper around Pull that adds retry logic and timeout handling.
// This method is used in Talos internally when performing unattended pulls of images,
// e.g. for the kubelet/etcd services.
// When pulling via the API, or under retry/backoff control, use Pull method directly instead.
func PullWithRetriesAndTimeout(
	ctx context.Context,
	registryBuilder RegistriesBuilder,
	resources state.State,
	client *containerd.Client,
	ref string,
	opt ...PullOption,
) (img containerd.Image, err error) {
	opts := DefaultPullOptions()

	for _, o := range opt {
		o(&opts)
	}

	namedRef, err := reference.ParseDockerRef(ref)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference %q: %w", ref, err)
	}

	notFoundErrors := 0

	err = retry.Exponential(
		PullTimeout,
		retry.WithUnits(PullRetryInterval),
		retry.WithErrorLogging(true)).
		RetryWithContext(
			ctx,
			func(ctx context.Context) error {
				img, err = pullInternal(ctx, registryBuilder, resources, client, namedRef, opts)
				if err != nil {
					switch {
					case xerrors.TagIs[imageNotFoundTag](err):
						notFoundErrors++
						if notFoundErrors > opts.MaxNotFoundRetries {
							return err
						}

						return retry.ExpectedError(err)
					case xerrors.TagIs[imageSignatureVerificationFailedTag](err):
						return err // image verification failure is terminal
					case xerrors.TagIs[imageTerminalErrorTag](err):
						return err // other terminal error, no need to retry
					default:
						return retry.ExpectedError(err)
					}
				}

				return nil
			},
		)

	return img, err
}

//nolint:gocyclo
func pullInternal(
	ctx context.Context,
	registryBuilder RegistriesBuilder,
	resources state.State,
	client *containerd.Client,
	namedRef reference.Named,
	opts PullOptions,
) (img containerd.Image, err error) {
	// normalize reference
	ref := namedRef.String()

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
	containerdLogger.Out = opts.LogWriter
	containerdLogger.Formatter = &logrus.TextFormatter{
		DisableColors:    true,
		DisableQuote:     true,
		DisableTimestamp: true,
	}

	ctx = log.WithLogger(ctx, containerdLogger.WithField("image", ref))

	registriesConfig, err := registryBuilder(ctx)
	if err != nil {
		return nil, xerrors.NewTaggedf[imageTerminalErrorTag]("failed to get configured registries: %w", err)
	}

	resolver := NewResolver(registriesConfig)
	tagFetcher := NewTagFetcher(registriesConfig)

	verifyResult, err := verify.ImageSignature(ctx, zap.NewNop(), resources, resolver, tagFetcher, ref)
	if err != nil {
		switch status.Code(err) { //nolint:exhaustive
		case codes.PermissionDenied:
			// verification denied by matched rule, no need to retry
			return nil, xerrors.NewTagged[imageSignatureVerificationFailedTag](errors.New(status.Convert(err).Message()))
		case codes.NotFound:
			// verification failed because image not found
			return nil, xerrors.NewTagged[imageNotFoundTag](err)
		default:
			return nil, err
		}
	}

	containerdRemoteOpts := []containerd.RemoteOpt{
		containerd.WithPullUnpack,
		containerd.WithChildLabelMap(images.ChildGCLabelsFilterLayers),
		containerd.WithResolver(resolver),
	}

	pullRef := ref

	if verifyResult.Verified {
		containerdRemoteOpts = append(
			containerdRemoteOpts,
			containerd.WithPullLabel(constants.ImageLabelVerified, verifyResult.Message),
		)

		pullRef = verifyResult.DigestedImageRef
	}

	if opts.NewProgressReporter != nil {
		reporter := opts.NewProgressReporter(ref)

		reporter.Start()
		defer reporter.Stop()

		pp := progress.NewPullProgress(
			client.ContentStore(),
			client.SnapshotService("overlayfs"),
			reporter.Update,
		)

		finishProgress := pp.ShowProgress(ctx)
		defer finishProgress()

		containerdRemoteOpts = append(
			containerdRemoteOpts,
			containerd.WithImageHandler(
				images.HandlerFunc(
					func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
						if images.IsLayerType(desc.MediaType) {
							pp.Add(desc)
						}

						return nil, nil
					},
				),
			),
			containerd.WithImageHandlerWrapper(snapshotters.AppendInfoHandlerWrapper(ref)),
		)
	}

	img, err = client.Pull(
		ctx,
		pullRef,
		containerdRemoteOpts...,
	)
	if err != nil {
		err = fmt.Errorf("failed to pull image %q: %w", ref, err)
		if errors.Is(err, errdefs.ErrNotFound) {
			return nil, xerrors.NewTagged[imageNotFoundTag](err)
		}

		return nil, err
	}

	if err = manageAliases(ctx, client, namedRef, img); err != nil {
		return nil, xerrors.NewTagged[imageTerminalErrorTag](err)
	}

	return img, nil
}
