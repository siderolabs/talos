// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvm

import (
	"context"
	"fmt"
)

// LV represents a logical volume as reported by `lvs --reportformat json`.
//
// Field semantics mirror the columns documented in lvs(8); see the manpage
// for the source-of-truth definitions. Numeric and tri-state columns are
// kept as raw strings so the caller can surface the original LVM value
// (including sentinels like "", "-1", "auto", "unknown") verbatim.
type LV struct {
	Path              string `json:"lv_path"`
	DMPath            string `json:"lv_dm_path"`
	Name              string `json:"lv_name"`
	FullName          string `json:"lv_full_name"`
	VGName            string `json:"vg_name"`
	UUID              string `json:"lv_uuid"`
	Layout            string `json:"lv_layout"`
	Role              string `json:"lv_role"`
	Permissions       string `json:"lv_permissions"`
	AllocationPolicy  string `json:"lv_allocation_policy"`
	AllocationLocked  string `json:"lv_allocation_locked"`
	FixedMinor        string `json:"lv_fixed_minor"`
	Active            string `json:"lv_active"`
	ActiveLocally     string `json:"lv_active_locally"`
	ActiveRemotely    string `json:"lv_active_remotely"`
	ActiveExclusively string `json:"lv_active_exclusively"`
	Suspended         string `json:"lv_suspended"`
	DeviceOpen        string `json:"lv_device_open"`
	SkipActivation    string `json:"lv_skip_activation"`
	Merging           string `json:"lv_merging"`
	Converting        string `json:"lv_converting"`
	Size              string `json:"lv_size"`
	MetadataSize      string `json:"lv_metadata_size"`
	ReadAhead         string `json:"lv_read_ahead"`
	KernelMajor       string `json:"lv_kernel_major"`
	KernelMinor       string `json:"lv_kernel_minor"`
	Origin            string `json:"origin"`
	OriginSize        string `json:"origin_size"`
	PoolLV            string `json:"pool_lv"`
	DataLV            string `json:"data_lv"`
	MetadataLV        string `json:"metadata_lv"`
	MovePV            string `json:"move_pv"`
	ConvertLV         string `json:"convert_lv"`
	WhenFull          string `json:"lv_when_full"`
	Tags              Tags   `json:"lv_tags"`
}

// LVS runs `lvm lvs -a -o +all --reportformat json --units b --nosuffix`
// and returns the parsed records.
func (lvm *LVM) LVS(ctx context.Context) ([]LV, error) {
	out, err := lvm.run(ctx, "lvs", commonReportArgs...)
	if err != nil {
		return nil, err
	}

	return parseLVS(out)
}

func parseLVS(out string) ([]LV, error) {
	lvs, err := decodeReport[LV](out, "lv")
	if err != nil {
		return nil, fmt.Errorf("failed to decode lvs report: %w", err)
	}

	return lvs, nil
}

// LVRemove runs `lvm lvremove --yes <vg>/<lv>` to remove a logical volume.
//
// --reportformat=json is intentionally NOT passed here: with that flag set,
// LVM redirects log_error() messages into the JSON `log` array on stdout
// (lib/log/log.c:640) instead of stderr, leaving stderr empty and breaking
// classifyError's stderr matchers.
//
// Errors propagate through (*LVM).run which normalises them to the sentinels
// declared in errors.go (ErrNotFound, ErrOpen, ...).
func (lvm *LVM) LVRemove(ctx context.Context, vg, lv string) error {
	if vg == "" || lv == "" {
		return fmt.Errorf("vg and lv must be non-empty")
	}

	_, err := lvm.run(ctx, "lvremove", "--yes", fmt.Sprintf("%s/%s", vg, lv))

	return err
}
