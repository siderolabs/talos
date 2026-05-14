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
