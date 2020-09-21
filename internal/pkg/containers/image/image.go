// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/pkg/machinery/config"
)

// Image pull retry settings.
const (
	PullTimeout       = 20 * time.Minute
	PullRetryInterval = 5 * time.Second
)

// Pull is a convenience function that wraps the containerd image pull func with
// retry functionality.
func Pull(ctx context.Context, reg config.Registries, client *containerd.Client, ref string) (img containerd.Image, err error) {
	resolver := NewResolver(reg)

	err = retry.Exponential(PullTimeout, retry.WithUnits(PullRetryInterval), retry.WithErrorLogging(true)).Retry(func() error {
		if img, err = client.Pull(ctx, ref, containerd.WithPullUnpack, containerd.WithResolver(resolver)); err != nil {
			err = fmt.Errorf("failed to pull image %q: %w", ref, err)

			if errdefs.IsNotFound(err) {
				return retry.UnexpectedError(err)
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
