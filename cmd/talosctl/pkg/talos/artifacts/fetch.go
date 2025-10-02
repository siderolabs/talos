// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package artifacts handles manifest traversal for Overalys and Extensions.
package artifacts

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/images"
)

type imageHandler func(ctx context.Context, img v1.Image) error

// FetchTimeout controls overall timeout for fetching artifacts for a release.
const FetchTimeout = 20 * time.Minute

type fetchManager struct {
	imageRegistry name.Registry
	puller        *remote.Puller
}

func newManager() (*fetchManager, error) {
	imageRegistry, err := name.NewRegistry(images.Registry)
	if err != nil {
		return nil, fmt.Errorf("failed to create image registry: %w", err)
	}

	puller, err := remote.NewPuller(
		remote.WithPlatform(v1.Platform{
			Architecture: string(ArchAmd64),
			OS:           "linux",
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create puller: %w", err)
	}

	return &fetchManager{
		imageRegistry: imageRegistry,
		puller:        puller,
	}, nil
}

func (m *fetchManager) fetchImageByTag(imageName, tag string, imageHandler imageHandler) error {
	// set a timeout for fetching, but don't bind it to any context, as we want fetch operation to finish
	ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
	defer cancel()

	// light check first - if the image exists, and resolve the digest
	// it's important to do further checks by digest exactly
	repoRef := m.imageRegistry.Repo(imageName).Tag(tag)

	descriptor, err := m.puller.Head(ctx, repoRef)
	if err != nil {
		return err
	}

	digestRef := repoRef.Digest(descriptor.Digest.String())

	return m.fetchImageByDigest(digestRef, imageHandler)
}

// fetchImageByDigest fetches an image by digest, verifies signatures, and exports it to the storage.
func (m *fetchManager) fetchImageByDigest(digestRef name.Digest, imageHandler imageHandler) error {
	var err error

	// set a timeout for fetching, but don't bind it to any context, as we want fetch operation to finish
	ctx, cancel := context.WithTimeout(context.Background(), FetchTimeout)
	defer cancel()

	desc, err := m.puller.Get(ctx, digestRef)
	if err != nil {
		return fmt.Errorf("error pulling image %s: %w", digestRef, err)
	}

	img, err := desc.Image()
	if err != nil {
		return fmt.Errorf("error creating image from descriptor: %w", err)
	}

	return imageHandler(ctx, img)
}

// imageExportHandler exports the image for further processing.
func imageExportHandler(exportHandler func(r io.Reader) error) imageHandler {
	return func(_ context.Context, img v1.Image) error {
		r, w := io.Pipe()

		var eg errgroup.Group

		eg.Go(func() error {
			defer w.Close() //nolint:errcheck

			return crane.Export(img, w)
		})

		eg.Go(func() error {
			err := exportHandler(r)
			if err != nil {
				r.CloseWithError(err) // signal the exporter to stop
			}

			return err
		})

		if err := eg.Wait(); err != nil {
			return fmt.Errorf("error extracting the image: %w", err)
		}

		return nil
	}
}
