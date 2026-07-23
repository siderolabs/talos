// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"hash/fnv"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// VolumeTrimScheduleType is type of VolumeTrimSchedule resource.
const VolumeTrimScheduleType = resource.Type("VolumeTrimSchedules.block.talos.dev")

// VolumeTrimSchedule resource describes when a volume should be trimmed (fstrim).
//
// The resource ID is the volume ID.
type VolumeTrimSchedule = typed.Resource[VolumeTrimScheduleSpec, VolumeTrimScheduleExtension]

// VolumeTrimScheduleSpec is the spec for VolumeTrimSchedule resource.
//
//gotagsrewrite:gen
type VolumeTrimScheduleSpec struct {
	// Filesystem is the filesystem type of the volume to be trimmed.
	Filesystem FilesystemType `yaml:"filesystem" protobuf:"1"`
	// Interval is the trim interval for the volume.
	Interval time.Duration `yaml:"interval" protobuf:"2"`
	// NextTrim is the next scheduled trim time for the volume.
	NextTrim time.Time `yaml:"nextTrim" protobuf:"3"`
}

// TrimScheduleOffset returns the stable offset within the trim interval for a seed.
//
// The offset is derived by hashing the seed (e.g. node ID + volume ID), so it stays
// constant for a given seed and interval, spreading trims across the interval - both
// across volumes on a node and across nodes in a cluster.
func TrimScheduleOffset(seed string, interval time.Duration) time.Duration {
	if interval <= 0 {
		return 0
	}

	h := fnv.New64a()
	h.Write([]byte(seed)) //nolint:errcheck // hash.Hash.Write never returns an error

	return time.Duration(h.Sum64() % uint64(interval))
}

// NextTrimTime returns the earliest trim slot strictly after t for the seed.
//
// Trim slots form a stable lattice anchored at the Unix epoch: offset, offset+interval,
// offset+2*interval, ... where offset is derived from the seed.
func NextTrimTime(seed string, interval time.Duration, t time.Time) time.Time {
	if interval <= 0 {
		return time.Time{}
	}

	anchor := time.Unix(0, int64(TrimScheduleOffset(seed, interval)))

	return TrimSlotAfter(anchor, interval, t)
}

// TrimSlotBefore returns the most recent trim slot at or before t on the lattice
// anchored at the given slot (anchor + k*interval for integer k).
//
// It only needs a single known slot (anchor) and the interval, so it does not depend
// on the seed used to compute the schedule.
func TrimSlotBefore(anchor time.Time, interval time.Duration, t time.Time) time.Time {
	if interval <= 0 {
		return time.Time{}
	}

	step := int64(interval)
	diff := t.UnixNano() - anchor.UnixNano()

	// number of full intervals between the anchor and t (floored).
	k := diff / step
	if diff%step < 0 {
		k--
	}

	return anchor.Add(time.Duration(k) * interval)
}

// TrimSlotAfter returns the earliest trim slot strictly after t on the lattice
// anchored at the given slot.
func TrimSlotAfter(anchor time.Time, interval time.Duration, t time.Time) time.Time {
	if interval <= 0 {
		return time.Time{}
	}

	slot := TrimSlotBefore(anchor, interval, t)
	if !slot.After(t) {
		slot = slot.Add(interval)
	}

	return slot
}

// NewVolumeTrimSchedule initializes a VolumeTrimSchedule resource.
func NewVolumeTrimSchedule(namespace resource.Namespace, id resource.ID) *VolumeTrimSchedule {
	return typed.NewResource[VolumeTrimScheduleSpec, VolumeTrimScheduleExtension](
		resource.NewMetadata(namespace, VolumeTrimScheduleType, id, resource.VersionUndefined),
		VolumeTrimScheduleSpec{},
	)
}

// VolumeTrimScheduleExtension is auxiliary resource data for VolumeTrimSchedule.
type VolumeTrimScheduleExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VolumeTrimScheduleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeTrimScheduleType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Filesystem",
				JSONPath: `{.filesystem}`,
			},
			{
				Name:     "Interval",
				JSONPath: `{.interval}`,
			},
			{
				Name:     "Next Trim",
				JSONPath: `{.nextTrim}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(VolumeTrimScheduleType, &VolumeTrimSchedule{})
	if err != nil {
		panic(err)
	}
}
