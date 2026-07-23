// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package blockhelpers provides helper functions for working with block resources.
package blockhelpers

import (
	"context"
	"fmt"
	"sort"

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

// MatchContext is a discovered block device (whole disk or partition) prepared
// for CEL selector evaluation.
type MatchContext struct {
	// DevPath is the /dev path of the device.
	DevPath string
	// CELContext holds the CEL variables bound for evaluation: `volume`, `disk`
	// and `system_disk`. Partitions and disks without a matching Disk resource
	// get an empty `disk`, so disk-level predicates evaluate false rather than
	// erroring on an unbound variable. `system_disk` and `volume` are always
	// bound; a selector parsed with celenv.DiskLocator (no `volume`) or
	// celenv.VolumeLocator (no `system_disk`) simply ignores the extra binding.
	CELContext map[string]any
	// Disk reports whether this is a whole disk (no parent partition).
	Disk bool
	// Partitioned reports whether this whole disk holds partitions and therefore
	// cannot back a physical volume or RAID member directly.
	Partitioned bool
	// SystemDisk reports whether this device is, or belongs to, the Talos system disk.
	SystemDisk bool
}

// BuildMatchContexts prepares CEL evaluation contexts from discovered disks and
// volumes so callers can match both whole disks and partitions against a
// selector. Taking already-listed slices keeps it a pure function usable from
// either a controller.Reader or a state.State caller.
//
// Every volume gets a `volume` variable; `disk` is bound to the real disk only
// for whole-disk volumes (partitions get an empty DiskSpec). `system_disk` is
// true for the system disk and its partitions when systemDiskDevPath is known
// ("" if not).
func BuildMatchContexts(disks []*block.Disk, volumes []*block.DiscoveredVolume, systemDiskDevPath string) ([]MatchContext, error) {
	diskByDevPath, err := diskSpecsByDevPath(disks)
	if err != nil {
		return nil, err
	}

	hasPartitions := partitionedDevPaths(volumes)
	out := make([]MatchContext, 0, len(volumes))

	for _, v := range volumes {
		context, ok, err := buildMatchContext(v, diskByDevPath, hasPartitions, systemDiskDevPath)
		if err != nil {
			return nil, err
		}

		if ok {
			out = append(out, context)
		}
	}

	// Stable order for deterministic downstream iteration.
	sort.Slice(out, func(i, j int) bool { return out[i].DevPath < out[j].DevPath })

	return out, nil
}

func diskSpecsByDevPath(disks []*block.Disk) (map[string]*blockpb.DiskSpec, error) {
	diskByDevPath := make(map[string]*blockpb.DiskSpec, len(disks))

	for _, d := range disks {
		spec := &blockpb.DiskSpec{}

		if err := proto.ResourceSpecToProto(d, spec); err != nil {
			return nil, fmt.Errorf("convert disk %q to proto: %w", d.Metadata().ID(), err)
		}

		diskByDevPath[spec.DevPath] = spec
	}

	return diskByDevPath, nil
}

func partitionedDevPaths(volumes []*block.DiscoveredVolume) map[string]struct{} {
	// Devices that are the parent of at least one partition; a partitioned whole
	// disk cannot back a PV or array member directly.
	hasPartitions := map[string]struct{}{}

	for _, v := range volumes {
		if parent := v.TypedSpec().ParentDevPath; parent != "" {
			hasPartitions[parent] = struct{}{}
		}
	}

	return hasPartitions
}

func buildMatchContext(
	volume *block.DiscoveredVolume,
	diskByDevPath map[string]*blockpb.DiskSpec,
	hasPartitions map[string]struct{},
	systemDiskDevPath string,
) (MatchContext, bool, error) {
	spec := &blockpb.DiscoveredVolumeSpec{}

	if err := proto.ResourceSpecToProto(volume, spec); err != nil {
		return MatchContext{}, false, fmt.Errorf("convert discovered volume %q to proto: %w", volume.Metadata().ID(), err)
	}

	if spec.DevPath == "" {
		return MatchContext{}, false, nil
	}

	disk, isDisk, partitioned := matchContextDisk(spec, diskByDevPath, hasPartitions)

	systemDisk := systemDiskDevPath != "" && (spec.DevPath == systemDiskDevPath || spec.ParentDevPath == systemDiskDevPath)

	return MatchContext{
		DevPath: spec.DevPath,
		CELContext: map[string]any{
			"volume":      spec,
			"disk":        disk,
			"system_disk": systemDisk,
		},
		Disk:        isDisk,
		Partitioned: partitioned,
		SystemDisk:  systemDisk,
	}, true, nil
}

func matchContextDisk(
	volume *blockpb.DiscoveredVolumeSpec,
	diskByDevPath map[string]*blockpb.DiskSpec,
	hasPartitions map[string]struct{},
) (*blockpb.DiskSpec, bool, bool) {
	if volume.ParentDevPath != "" {
		return &blockpb.DiskSpec{}, false, false
	}

	disk := diskByDevPath[volume.DevPath]
	if disk == nil {
		disk = &blockpb.DiskSpec{}
	}

	_, partitioned := hasPartitions[volume.DevPath]

	return disk, true, partitioned
}
