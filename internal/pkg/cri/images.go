// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// PullImage pulls container image.
func (c *Client) PullImage(ctx context.Context, image *runtimeapi.ImageSpec, sandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	resp, err := c.imagesClient.PullImage(ctx, &runtimeapi.PullImageRequest{
		Image:         image,
		SandboxConfig: sandboxConfig,
	})
	if err != nil {
		return "", fmt.Errorf("error pulling image %s: %w", image, err)
	}

	return resp.ImageRef, nil
}

// ListImages lists available images.
func (c *Client) ListImages(ctx context.Context, filter *runtimeapi.ImageFilter) ([]*runtimeapi.Image, error) {
	resp, err := c.imagesClient.ListImages(ctx, &runtimeapi.ListImagesRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, fmt.Errorf("error listing images: %w", err)
	}

	return resp.Images, nil
}

// ImageStatus returns the status of the image.
func (c *Client) ImageStatus(ctx context.Context, image *runtimeapi.ImageSpec) (*runtimeapi.Image, error) {
	resp, err := c.imagesClient.ImageStatus(ctx, &runtimeapi.ImageStatusRequest{
		Image: image,
	})
	if err != nil {
		return nil, fmt.Errorf("ImageStatus %q from image service failed: %w", image.Image, err)
	}

	if resp.Image != nil {
		if resp.Image.Id == "" || resp.Image.Size_ == 0 {
			return nil, fmt.Errorf("id or size of image %q is not set", image.Image)
		}
	}

	return resp.Image, nil
}
