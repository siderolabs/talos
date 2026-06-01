// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package blockhelpers provides helper functions for working with block resources.
package blockhelpers

import (
	"context"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// MatchDisks returns a list of disks that match the given expression.
func MatchDisks(ctx context.Context, st state.State, expression *cel.Expression) ([]*block.Disk, error) {
	disks, err := safe.StateListAll[*block.Disk](ctx, st)
	if err != nil {
		return nil, err
	}

	var matchedDisks []*block.Disk

	for disk := range disks.All() {
		spec := &blockpb.DiskSpec{}

		if err = proto.ResourceSpecToProto(disk, spec); err != nil {
			return nil, err
		}

		matches, err := expression.EvalBool(celenv.DiskLocator(), map[string]any{
			"disk":        spec,
			"system_disk": false,
		})
		if err != nil {
			return nil, err
		}

		if matches {
			matchedDisks = append(matchedDisks, disk)
		}
	}

	return matchedDisks, nil
}
