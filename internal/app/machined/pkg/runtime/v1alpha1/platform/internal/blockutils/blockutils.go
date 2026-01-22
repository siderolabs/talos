// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package blockutils provides volume-related helpers for platform implementation.
package blockutils

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
)

// VolumeMatch returns a CEL expression that matches a volume by filesystem or partition label.
func VolumeMatch(labels []string) (*cel.Expression, error) {
	builder := cel.NewBuilder(celenv.VolumeLocator())

	// "(volume.label in ['%s', ...] || volume.partition_label in ['%s', ...]) && volume.name != ''"
	expr := builder.NewCall(
		builder.NextID(),
		operators.LogicalAnd,
		builder.NewCall(
			builder.NextID(),
			operators.LogicalOr,
			builder.NewCall(
				builder.NextID(),
				operators.In,
				builder.NewSelect(
					builder.NextID(),
					builder.NewIdent(builder.NextID(), "volume"),
					"label",
				),
				builder.NewList(
					builder.NextID(),
					xslices.Map(labels, func(label string) ast.Expr {
						return builder.NewLiteral(builder.NextID(), types.String(label))
					}),
					nil,
				),
			),
			builder.NewCall(
				builder.NextID(),
				operators.In,
				builder.NewSelect(
					builder.NextID(),
					builder.NewIdent(builder.NextID(), "volume"),
					"partition_label",
				),
				builder.NewList(
					builder.NextID(),
					xslices.Map(labels, func(label string) ast.Expr {
						return builder.NewLiteral(builder.NextID(), types.String(label))
					}),
					nil,
				),
			),
		),
		builder.NewCall(
			builder.NextID(),
			operators.NotEquals,
			builder.NewSelect(
				builder.NextID(),
				builder.NewIdent(builder.NextID(), "volume"),
				"name",
			),
			builder.NewLiteral(builder.NextID(), types.String("")),
		),
	)

	boolExpr, err := builder.ToBooleanExpression(expr)
	if err != nil {
		return nil, fmt.Errorf("error creating boolean expression: %w", err)
	}

	return boolExpr, nil
}

// ReadFromVolume tries to find a volume with the given label, mounts it
// as read-only, calls the provided function with xfs.Root and unmounts it.
//
// If the volume wasn't found, fs.ErrNotExist is returned.
func ReadFromVolume(ctx context.Context, r state.State, labels []string, cb func(xfs.Root, *block.VolumeStatus) error) error {
	if len(labels) == 0 {
		panic("at least one label must be provided")
	}

	volumeID := "platform/" + labels[0] + "/config"

	matchExr, err := VolumeMatch(labels)
	if err != nil {
		return fmt.Errorf("error creating volume match expression: %w", err)
	}

	// create a volume which matches the expected filesystem label
	vc := block.NewVolumeConfig(block.NamespaceName, volumeID)
	vc.Metadata().Labels().Set(block.PlatformLabel, "")
	vc.TypedSpec().Type = block.VolumeTypePartition
	vc.TypedSpec().Locator = block.LocatorSpec{
		Match: *matchExr,
	}

	if err := r.Create(ctx, vc); err != nil && !state.IsConflictError(err) {
		return fmt.Errorf("error creating user disk volume configuration: %w", err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := r.TeardownAndDestroy(ctx, vc.Metadata()); err != nil {
			log.Printf("error destroying volume config %s/%s: %v", vc.Metadata().Namespace(), vc.Metadata().ID(), err)
		}
	}()

	// wait for the volume to be either ready or missing (includes waiting for devices to be ready)
	volumeStatus, err := safe.StateWatchFor[*block.VolumeStatus](ctx,
		r,
		block.NewVolumeStatus(vc.Metadata().Namespace(), vc.Metadata().ID()).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			phase := r.(*block.VolumeStatus).TypedSpec().Phase

			return phase == block.VolumePhaseReady || phase == block.VolumePhaseMissing, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to watch for volume status: %w", err)
	}

	if volumeStatus.TypedSpec().Phase == block.VolumePhaseMissing {
		return fmt.Errorf("failed to find volume with machine configuration %s: %w", vc.TypedSpec().Locator.Match, fs.ErrNotExist)
	}

	manager := mount.NewManager(
		mount.WithReadOnly(),
		mount.WithPrinter(log.Printf),
		mount.WithFsopen(
			volumeStatus.TypedSpec().Filesystem.String(),
			fsopen.WithSource(volumeStatus.TypedSpec().MountLocation),
			fsopen.WithBoolParameter("ro"),
		),
		mount.WithDetached(),
	)

	// mount the volume, unmount when done
	p, err := manager.Mount()
	if err != nil {
		return fmt.Errorf("failed to mount volume: %w", err)
	}

	defer manager.Unmount() //nolint:errcheck

	return cb(p.Root(), volumeStatus)
}
