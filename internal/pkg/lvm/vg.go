// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvm

import (
	"context"
	"fmt"
)

// VG represents a volume group as reported by `vgs --reportformat json`.
//
// Field semantics mirror the columns documented in vgs(8); see the manpage
// for the source-of-truth definitions. Numeric and tri-state columns are
// kept as raw strings so the caller can surface the original LVM value
// (including sentinels like "", "-1", "unknown", "unmanaged") verbatim.
type VG struct {
	Name             string `json:"vg_name"`
	UUID             string `json:"vg_uuid"`
	Format           string `json:"vg_fmt"`
	Permissions      string `json:"vg_permissions"`
	Extendable       string `json:"vg_extendable"`
	Exported         string `json:"vg_exported"`
	Partial          string `json:"vg_partial"`
	AllocationPolicy string `json:"vg_allocation_policy"`
	Clustered        string `json:"vg_clustered"`
	Shared           string `json:"vg_shared"`
	Size             string `json:"vg_size"`
	Free             string `json:"vg_free"`
	ExtentSize       string `json:"vg_extent_size"`
	ExtentCount      string `json:"vg_extent_count"`
	FreeExtentCount  string `json:"vg_free_count"`
	MaxLV            string `json:"max_lv"`
	MaxPV            string `json:"max_pv"`
	LVCount          string `json:"lv_count"`
	PVCount          string `json:"pv_count"`
	SnapCount        string `json:"snap_count"`
	MissingPVCount   string `json:"vg_missing_pv_count"`
	SeqNo            string `json:"vg_seqno"`
	LockType         string `json:"vg_lock_type"`
	SystemID         string `json:"vg_systemid"`
	Tags             Tags   `json:"vg_tags"`
}

// VGS runs `lvm vgs -a -o +all --reportformat json --units b --nosuffix`
// and returns the parsed records.
func (lvm *LVM) VGS(ctx context.Context) ([]VG, error) {
	out, err := lvm.run(ctx, "vgs", commonReportArgs...)
	if err != nil {
		return nil, err
	}

	return parseVGS(out)
}

func parseVGS(out string) ([]VG, error) {
	vgs, err := decodeReport[VG](out, "vg")
	if err != nil {
		return nil, fmt.Errorf("failed to decode vgs report: %w", err)
	}

	return vgs, nil
}

// VGCreate runs `lvm vgcreate --yes <vg> <pvs...>` to create a new volume
// group spanning the supplied physical volumes.
//
// All listed devices must already be initialized as PVs (see PVCreate) and
// must not belong to another VG; pvcreate is NOT bundled here so the
// controller can manage initialisation order explicitly.
//
// --reportformat=json is intentionally NOT passed here for the same reason
// documented on VGRemove.
//
// Errors propagate through (*LVM).run which normalises them to the sentinels
// declared in errors.go.
func (lvm *LVM) VGCreate(ctx context.Context, vg string, pvs ...string) error {
	if vg == "" {
		return fmt.Errorf("vg must be non-empty")
	}

	if len(pvs) == 0 {
		return fmt.Errorf("at least one physical volume is required")
	}

	args := append([]string{"--yes", vg}, pvs...)

	_, err := lvm.run(ctx, "vgcreate", args...)

	return err
}

// VGExtend runs `lvm vgextend --yes <vg> <pvs...>` to add already-initialized
// physical volumes to an existing volume group.
//
// All listed devices must already be initialized as PVs (see PVCreate).
//
// --reportformat=json is intentionally NOT passed here for the same reason
// documented on VGRemove.
//
// Errors propagate through (*LVM).run which normalises them to the sentinels
// declared in errors.go.
func (lvm *LVM) VGExtend(ctx context.Context, vg string, pvs ...string) error {
	if vg == "" {
		return fmt.Errorf("vg must be non-empty")
	}

	if len(pvs) == 0 {
		return fmt.Errorf("at least one physical volume is required")
	}

	args := append([]string{"--yes", vg}, pvs...)

	_, err := lvm.run(ctx, "vgextend", args...)

	return err
}

// VGReduce runs `lvm vgreduce --yes <vg> <pvs...>` to detach physical volumes
// from a volume group. The PVs keep their LVM labels and may be re-used; pass
// them to PVRemove if they should be fully wiped.
//
// vgreduce refuses to remove a PV that still holds allocated extents; the
// caller must lvremove / pvmove first. This is intentional: silent data loss
// is the worst possible failure mode for the storage controller.
//
// --reportformat=json is intentionally NOT passed here for the same reason
// documented on VGRemove.
//
// Errors propagate through (*LVM).run which normalises them to the sentinels
// declared in errors.go.
func (lvm *LVM) VGReduce(ctx context.Context, vg string, pvs ...string) error {
	if vg == "" {
		return fmt.Errorf("vg must be non-empty")
	}

	if len(pvs) == 0 {
		return fmt.Errorf("at least one physical volume is required")
	}

	args := append([]string{"--yes", vg}, pvs...)

	_, err := lvm.run(ctx, "vgreduce", args...)

	return err
}

// VGRemove runs `lvm vgremove --yes <vg>` to remove a volume group.
//
// CASCADE: --yes (DONT_PROMPT in lvm2 terms; see tools/vgremove.c:42) makes
// vgremove iterate every LV in the group through lvremove_single before
// dropping the VG. Callers that need per-LV control must invoke LVRemove
// first. The underlying PVs keep their LVM labels and require a separate
// PVRemove to be fully wiped.
//
// --reportformat=json is intentionally NOT passed here: with that flag set,
// LVM redirects log_error() messages into the JSON `log` array on stdout
// (lib/log/log.c:640) instead of stderr, leaving stderr empty and breaking
// classifyError's stderr matchers.
//
// Errors propagate through (*LVM).run which normalises them to the sentinels
// declared in errors.go (ErrNotFound, ErrNotEmpty, ...).
func (lvm *LVM) VGRemove(ctx context.Context, vg string) error {
	if vg == "" {
		return fmt.Errorf("vg must be non-empty")
	}

	_, err := lvm.run(ctx, "vgremove", "--yes", vg)

	return err
}
