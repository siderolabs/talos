// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import (
	"context"
	"maps"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/errdefs"
	"github.com/distribution/reference"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

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
		if err := createAlias(ctx, client, newRef, img.Target(), img.Labels()); err != nil {
			return err
		}
	}

	return nil
}

func createAlias(ctx context.Context, client *containerd.Client, name string, desc ocispec.Descriptor, labels map[string]string) error {
	img := images.Image{
		Name:   name,
		Target: desc,
		Labels: labels,
	}

	oldImg, err := client.ImageService().Create(ctx, img)
	if err == nil || !errdefs.IsAlreadyExists(err) {
		return err
	}

	if oldImg.Target.Digest == img.Target.Digest && maps.Equal(oldImg.Labels, img.Labels) {
		return nil
	}

	_, err = client.ImageService().Update(ctx, img, "target", "labels")

	return err
}
