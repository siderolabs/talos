// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package blockhelpers provides helper functions for working with block resources.
package blockhelpers

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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
	// DevPath is the /dev path of the device to operate on. For a device backing a
	// machine config volume it is the volume's MountLocation, not the ciphertext
	// the selector matched on. See BuildMatchContexts.
	DevPath string
	// VolumeID is the machine config volume id backing this device (e.g.
	// "r-lvmdata"), empty for a device Talos does not manage as a volume.
	VolumeID string
	// CELContext holds the CEL variables bound for evaluation: `volume`, `disk`,
	// `volume_id` and `system_disk`. Partitions and disks without a matching Disk
	// resource get an empty `disk`, so disk-level predicates evaluate false rather
	// than erroring on an unbound variable. All four are always bound. A selector
	// parsed with a narrower environment ignores the extra bindings.
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
//
// An encrypted volume splits identity from device. The operator's GPT label is
// on the ciphertext partition, while the device to write to is the opened
// device-mapper device, which carries no partition label and whose /dev/dm-N
// path is assigned in open order. Such a device therefore matches on its
// ciphertext identity but is reported under the volume's MountLocation, with
// `volume_id` bound to the volume id.
//
// statuses and cfg also let withhold hold back a device the volume manager is
// not done with. cfg may be nil.
func BuildMatchContexts(
	disks []*block.Disk,
	volumes []*block.DiscoveredVolume,
	statuses []*block.VolumeStatus,
	cfg configconfig.Config,
	systemDiskDevPath string,
) ([]MatchContext, error) {
	diskByDevPath, err := diskSpecsByDevPath(disks)
	if err != nil {
		return nil, err
	}

	volumeIDByLocation, mountLocations := indexVolumeStatuses(statuses)
	hasPartitions := partitionedDevPaths(volumes)

	claimedDisks, err := wholeDiskVolumeDevPaths(cfg, diskByDevPath, systemDiskDevPath)
	if err != nil {
		return nil, err
	}

	out := make([]MatchContext, 0, len(volumes))

	for _, v := range volumes {
		context, ok, err := buildMatchContext(
			v, diskByDevPath, hasPartitions, volumeIDByLocation, mountLocations, claimedDisks, systemDiskDevPath,
		)
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

// volumeIDPrefixes are the prefixes of the volume ids Talos stamps as the
// partition label of a volume it provisions.
var volumeIDPrefixes = []string{constants.UserVolumePrefix, constants.RawVolumePrefix, constants.SwapVolumePrefix}

func namesVolume(partitionLabel string) bool {
	return slices.ContainsFunc(volumeIDPrefixes, func(prefix string) bool {
		return len(partitionLabel) > len(prefix) && strings.HasPrefix(partitionLabel, prefix)
	})
}

// wholeDiskVolumeDevPaths returns the disks matched by the disk selector of a
// whole-disk volume declared in cfg, evaluated with celenv.DiskLocator, the
// environment that parsed and validated it.
func wholeDiskVolumeDevPaths(
	cfg configconfig.Config,
	diskByDevPath map[string]*blockpb.DiskSpec,
	systemDiskDevPath string,
) (map[string]struct{}, error) {
	if cfg == nil {
		return nil, nil
	}

	claimed := map[string]struct{}{}

	for _, doc := range cfg.UserVolumeConfigs() {
		if doc.Type().ValueOr(block.VolumeTypePartition) != block.VolumeTypeDisk {
			continue
		}

		selector, ok := doc.Provisioning().DiskSelector().Get()
		if !ok {
			continue
		}

		for devPath, spec := range diskByDevPath {
			matches, err := selector.EvalBool(celenv.DiskLocator(), map[string]any{
				"disk":        spec,
				"system_disk": systemDiskDevPath != "" && devPath == systemDiskDevPath,
			})
			if err != nil {
				return nil, fmt.Errorf("evaluate disk selector of whole-disk volume %q: %w", doc.Name(), err)
			}

			if matches {
				claimed[devPath] = struct{}{}
			}
		}
	}

	return claimed, nil
}

// volumeLocation is what a VolumeStatus says about the device at its Location.
type volumeLocation struct {
	id string
	// mountLocation is empty while the volume manager has not prepared the volume.
	mountLocation string
}

// indexVolumeStatuses maps each volume's Location to the volume, and collects the
// set of MountLocations that redirect (the opened device of an encrypted volume).
func indexVolumeStatuses(statuses []*block.VolumeStatus) (map[string]volumeLocation, map[string]struct{}) {
	byLocation := make(map[string]volumeLocation, len(statuses))
	mountLocations := map[string]struct{}{}

	for _, status := range statuses {
		spec := status.TypedSpec()

		if spec.Location == "" {
			continue
		}

		byLocation[spec.Location] = volumeLocation{
			id:            status.Metadata().ID(),
			mountLocation: spec.MountLocation,
		}

		if spec.MountLocation != "" && spec.MountLocation != spec.Location {
			mountLocations[spec.MountLocation] = struct{}{}
		}
	}

	return byLocation, mountLocations
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

// withhold reports whether a volume is not done with a device, or another
// context already reports it.
func withhold(
	spec *blockpb.DiscoveredVolumeSpec,
	vol volumeLocation,
	claimed bool,
	mountLocations map[string]struct{},
	claimedDisks map[string]struct{},
) bool {
	if claimed {
		// Nothing knows yet which device this volume will end up on, so the raw
		// device must not go out.
		return vol.mountLocation == ""
	}

	// A ciphertext no volume claims; the prober already says it is not plaintext.
	if spec.Name == "luks" {
		return true
	}

	// A partition Talos provisioned for a volume that does not report it yet.
	if namesVolume(spec.PartitionLabel) {
		return true
	}

	// The opened device of an encrypted volume, reported under its ciphertext's
	// identity instead.
	if _, redirected := mountLocations[spec.DevPath]; redirected {
		return true
	}

	// A whole disk that a declared whole-disk volume is going to claim.
	_, willClaim := claimedDisks[spec.DevPath]

	return willClaim
}

func buildMatchContext(
	volume *block.DiscoveredVolume,
	diskByDevPath map[string]*blockpb.DiskSpec,
	hasPartitions map[string]struct{},
	volumeIDByLocation map[string]volumeLocation,
	mountLocations map[string]struct{},
	claimedDisks map[string]struct{},
	systemDiskDevPath string,
) (MatchContext, bool, error) {
	spec := &blockpb.DiscoveredVolumeSpec{}

	if err := proto.ResourceSpecToProto(volume, spec); err != nil {
		return MatchContext{}, false, fmt.Errorf("convert discovered volume %q to proto: %w", volume.Metadata().ID(), err)
	}

	if spec.DevPath == "" {
		return MatchContext{}, false, nil
	}

	vol, claimed := volumeIDByLocation[spec.DevPath]

	if withhold(spec, vol, claimed, mountLocations, claimedDisks) {
		return MatchContext{}, false, nil
	}

	// spec stays the identity to match on; the volume decides the device.
	devPath := cmp.Or(vol.mountLocation, spec.DevPath)

	disk, isDisk := matchContextDisk(spec, diskByDevPath)
	_, partitioned := hasPartitions[devPath]
	systemDisk := systemDiskDevPath != "" && (spec.DevPath == systemDiskDevPath || spec.ParentDevPath == systemDiskDevPath)

	return MatchContext{
		DevPath:  devPath,
		VolumeID: vol.id,
		CELContext: map[string]any{
			"volume":      spec,
			"disk":        disk,
			"volume_id":   vol.id,
			"system_disk": systemDisk,
		},
		Disk:        isDisk,
		Partitioned: isDisk && partitioned,
		SystemDisk:  systemDisk,
	}, true, nil
}

func matchContextDisk(
	volume *blockpb.DiscoveredVolumeSpec,
	diskByDevPath map[string]*blockpb.DiskSpec,
) (*blockpb.DiskSpec, bool) {
	if volume.ParentDevPath != "" {
		return &blockpb.DiskSpec{}, false
	}

	disk := diskByDevPath[volume.DevPath]
	if disk == nil {
		disk = &blockpb.DiskSpec{}
	}

	return disk, true
}
