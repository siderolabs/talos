// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package opennebula provides the OpenNebula platform implementation.
package opennebula

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/blockutils"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/xfs"
)

const (
	configISOLabel = "context"
	oneContextPath = "context.sh"
)

func (o *OpenNebula) contextFromCD(ctx context.Context, r state.State) (oneContext []byte, err error) {
	err = blockutils.ReadFromVolume(ctx, r,
		[]string{strings.ToLower(configISOLabel), strings.ToUpper(configISOLabel)},
		func(root xfs.Root, volumeStatus *block.VolumeStatus) error {
			log.Printf("found config disk (context) at %s", volumeStatus.TypedSpec().Location)

			log.Printf("fetching context from: %s/", oneContextPath)

			oneContext, err = xfs.ReadFile(root, oneContextPath)
			if err != nil {
				return fmt.Errorf("read config: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	if oneContext == nil {
		return nil, errors.ErrNoConfigSource
	}

	return oneContext, nil
}
