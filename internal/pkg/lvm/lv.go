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

// LVCreateOptions configures LVCreate.
type LVCreateOptions struct {
	// Type is the LV layout.
	Type string
	// Mirrors is the number of mirror copies (raid1/raid10), passed as
	// `--mirrors`.
	Mirrors uint32
	// Stripes is the number of stripes (raid0/raid10), passed as `--stripes`.
	Stripes uint32
	// SizeBytes is the absolute LV size, passed as `-L <n>b`. Used when
	// SizePercentVG is zero.
	SizeBytes uint64
	// SizePercentVG, when non-zero, sizes the LV as a percentage of the VG
	// (`-l <n>%VG`) and takes precedence over SizeBytes.
	SizePercentVG uint32
}

// LVCreate runs `lvm lvcreate` to create a logical volume in the given VG.
//
// Supported layouts and their flags:
//   - linear: none
//   - raid0:  --type raid0 --stripes N
//   - raid1:  --type raid1 --mirrors M
//   - raid10: --type raid10 --mirrors M --stripes N
//
// Size is either absolute (`-L <bytes>b`) or a percentage of the VG
// (`-l <n>%VG`).
//
// --yes is intentionally NOT passed: the LV is freshly allocated, and we do not
// want to silently wipe any signature lvcreate might detect. --reportformat=json
// is omitted for the same stderr-classification reason documented on LVRemove.
//
// Errors propagate through (*LVM).run which normalises them to the sentinels
// declared in errors.go.
//
//nolint:gocyclo
func (lvm *LVM) LVCreate(ctx context.Context, vg, lv string, opts LVCreateOptions) error {
	if vg == "" || lv == "" {
		return fmt.Errorf("vg and lv must be non-empty")
	}

	args := []string{"-n", lv}

	switch opts.Type {
	case "", "linear":
		// linear is the default layout; no extra flags.
	case "raid0":
		if opts.Stripes < 2 {
			return fmt.Errorf("raid0 requires at least 2 stripes")
		}

		args = append(args, "--type", "raid0", "--stripes", fmt.Sprintf("%d", opts.Stripes))
	case "raid1":
		if opts.Mirrors < 1 {
			return fmt.Errorf("raid1 requires at least 1 mirror")
		}

		args = append(args, "--type", "raid1", "--mirrors", fmt.Sprintf("%d", opts.Mirrors))
	case "raid10":
		if opts.Mirrors < 1 {
			return fmt.Errorf("raid10 requires at least 1 mirror")
		}

		if opts.Stripes < 2 {
			return fmt.Errorf("raid10 requires at least 2 stripes")
		}

		args = append(args, "--type", "raid10", "--mirrors", fmt.Sprintf("%d", opts.Mirrors), "--stripes", fmt.Sprintf("%d", opts.Stripes))
	default:
		return fmt.Errorf("unsupported logical volume type %q", opts.Type)
	}

	switch {
	case opts.SizePercentVG > 0:
		args = append(args, "-l", fmt.Sprintf("%d%%VG", opts.SizePercentVG))
	case opts.SizeBytes > 0:
		args = append(args, "-L", fmt.Sprintf("%db", opts.SizeBytes))
	default:
		return fmt.Errorf("either SizeBytes or SizePercentVG must be set")
	}

	args = append(args, vg)

	_, err := lvm.run(ctx, "lvcreate", args...)

	return err
}

// LVExtendOptions configures LVExtend. SizePercentVG takes precedence over
// SizeBytes when non-zero.
type LVExtendOptions struct {
	// SizeBytes is the target absolute size, passed as `-L <n>b`.
	SizeBytes uint64
	// SizePercentVG, when non-zero, sizes the LV as a percentage of the VG
	// (`-l <n>%VG`).
	SizePercentVG uint32
}

// LVExtend runs `lvm lvextend` to grow a logical volume to the given target
// size (absolute bytes or a percentage of the VG).
//
// Grow-only: lvextend refuses to shrink, so this never destroys data. The
// filesystem on the LV is NOT resized here (--resizefs is not passed); that is
// the responsibility of the consuming volume layer.
//
// --reportformat=json is omitted for the same stderr-classification reason
// documented on LVRemove. Errors propagate through (*LVM).run.
func (lvm *LVM) LVExtend(ctx context.Context, vg, lv string, opts LVExtendOptions) error {
	if vg == "" || lv == "" {
		return fmt.Errorf("vg and lv must be non-empty")
	}

	var sizeArgs []string

	switch {
	case opts.SizePercentVG > 0:
		sizeArgs = []string{"-l", fmt.Sprintf("%d%%VG", opts.SizePercentVG)}
	case opts.SizeBytes > 0:
		sizeArgs = []string{"-L", fmt.Sprintf("%db", opts.SizeBytes)}
	default:
		return fmt.Errorf("either SizeBytes or SizePercentVG must be set")
	}

	_, err := lvm.run(ctx, "lvextend", append(sizeArgs, fmt.Sprintf("%s/%s", vg, lv))...)

	return err
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
