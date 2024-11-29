// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cri contains resources related to the Container Runtime Interface (CRI).
package cri

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

//go:generate deep-copy -type RegistriesConfigSpec -type ImageCacheConfigSpec -type SeccompProfileSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

//go:generate enumer -type=ImageCacheStatus -type=ImageCacheCopyStatus -linecomment -text

// NamespaceName contains resources related to stats.
const NamespaceName resource.Namespace = "cri"

// RegistryBuilder implements image.RegistriesBuilder.
func RegistryBuilder(st state.State) func(ctx context.Context) (config.Registries, error) {
	return func(ctx context.Context) (config.Registries, error) {
		regs, err := safe.StateWatchFor[*RegistriesConfig](ctx, st, NewRegistriesConfig().Metadata(), state.WithEventTypes(state.Created, state.Updated))
		if err != nil {
			return nil, err
		}

		return regs.TypedSpec(), nil
	}
}

// WaitForImageCache waits for the image cache config to be either disabled or ready.
func WaitForImageCache(ctx context.Context, st state.State) error {
	_, err := st.WatchFor(ctx, NewImageCacheConfig().Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			imageCacheConfig, ok := r.(*ImageCacheConfig)
			if !ok {
				return false, fmt.Errorf("unexpected resource type: %T", r)
			}

			s := imageCacheConfig.TypedSpec().Status

			return s == ImageCacheStatusDisabled || s == ImageCacheStatusReady, nil
		}),
	)

	return err
}

// WaitForImageCacheCopy waits for the image cache copy to be done (or skipped).
func WaitForImageCacheCopy(ctx context.Context, st state.State) error {
	_, err := st.WatchFor(ctx, NewImageCacheConfig().Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			imageCacheConfig, ok := r.(*ImageCacheConfig)
			if !ok {
				return false, fmt.Errorf("unexpected resource type: %T", r)
			}

			s := imageCacheConfig.TypedSpec().CopyStatus

			return s == ImageCacheCopyStatusReady || s == ImageCacheCopyStatusSkipped, nil
		}),
	)

	return err
}
