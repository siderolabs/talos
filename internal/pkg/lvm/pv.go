// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvm

import (
	"context"
	"fmt"
)

// PV represents a physical volume as reported by `pvs --reportformat json`.
//
// Field semantics mirror the columns documented in pvs(8); see the manpage
// for the source-of-truth definitions. Numeric and tri-state columns are
// kept as raw strings so the caller can surface the original LVM value
// (including sentinels like "", "-1") verbatim.
type PV struct {
	Device       string `json:"pv_name"`
	VGName       string `json:"vg_name"`
	UUID         string `json:"pv_uuid"`
	Format       string `json:"pv_fmt"`
	Allocatable  string `json:"pv_allocatable"`
	Exported     string `json:"pv_exported"`
	Missing      string `json:"pv_missing"`
	InUse        string `json:"pv_in_use"`
	Size         string `json:"pv_size"`
	DeviceSize   string `json:"dev_size"`
	Free         string `json:"pv_free"`
	Used         string `json:"pv_used"`
	PECount      string `json:"pv_pe_count"`
	PEAllocCount string `json:"pv_pe_alloc_count"`
	Major        string `json:"pv_major"`
	Minor        string `json:"pv_minor"`
	Tags         Tags   `json:"pv_tags"`
}

// PVS runs `lvm pvs -a -o +all --reportformat json --units b --nosuffix`
// and returns the parsed records.
func (lvm *LVM) PVS(ctx context.Context) ([]PV, error) {
	out, err := lvm.run(ctx, "pvs", commonReportArgs...)
	if err != nil {
		return nil, err
	}

	return parsePVS(out)
}

func parsePVS(out string) ([]PV, error) {
	pvs, err := decodeReport[PV](out, "pv")
	if err != nil {
		return nil, fmt.Errorf("failed to decode pvs report: %w", err)
	}

	return pvs, nil
}

// PVCreate runs `lvm pvcreate <device>` to initialize a block device as an
// LVM physical volume.
//
// --yes is intentionally NOT passed. Per lib/device/dev-type.c the prompt
// branch (lines 1172-1186) takes existing filesystem/RAID/swap signatures and
// either prompts the operator or, with --yes, wipes them silently. Talos runs
// pvcreate non-interactively, so omitting --yes means a device with a stale
// signature aborts the call with an error instead of being clobbered. Callers
// that knowingly want to overwrite the device must wipe it first via the
// explicit BlockDeviceWipe RPC.
//
// --reportformat=json is intentionally NOT passed here for the same reason
// documented on PVRemove.
//
// Errors propagate through (*LVM).run which normalises them to the sentinels
// declared in errors.go (ErrInUse, ErrInvalidCommand, ...).
func (lvm *LVM) PVCreate(ctx context.Context, device string) error {
	if device == "" {
		return fmt.Errorf("device must be non-empty")
	}

	_, err := lvm.run(ctx, "pvcreate", device)

	return err
}

// PVRemove runs `lvm pvremove --yes <device>` to wipe the LVM label/metadata
// from a physical volume.
//
// The PV must not be part of an active VG; remove the VG first with VGRemove.
//
// --reportformat=json is intentionally NOT passed here: with that flag set,
// LVM redirects log_error() messages into the JSON `log` array on stdout
// (lib/log/log.c:640) instead of stderr, leaving stderr empty and breaking
// classifyError's stderr matchers.
//
// Errors propagate through (*LVM).run which normalises them to the sentinels
// declared in errors.go (ErrNotFound, ErrInUse, ...).
func (lvm *LVM) PVRemove(ctx context.Context, device string) error {
	if device == "" {
		return fmt.Errorf("device must be non-empty")
	}

	_, err := lvm.run(ctx, "pvremove", "--yes", device)

	return err
}
