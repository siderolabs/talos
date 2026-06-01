// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lvm provides a Go interface to the Linux Logical Volume Manager (LVM).
package lvm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/siderolabs/go-cmd/pkg/cmd"

	"github.com/siderolabs/talos/internal/pkg/selinux"
)

const (
	runDir     = "/run"
	lvmLockDir = runDir + "/lock/lvm"
)

// LVM provides methods for managing LVM volumes.
type LVM struct {
	lvm          string
	selinuxLabel string
}

// New creates a new LVM instance.
func New(opts ...Option) (*LVM, error) {
	lvm := &LVM{
		lvm:          "/sbin/lvm",
		selinuxLabel: "system_u:object_r:var_lock_t:s0",
	}

	for _, opt := range opts {
		opt(lvm)
	}

	return lvm, lvm.init()
}

// Option is a functional option for configuring the LVM instance.
type Option func(*LVM)

// WithLVMPath sets the path to the `lvm` binary. This is required if `lvm` is not in the default location.
func WithLVMPath(path string) Option {
	return func(lvm *LVM) {
		lvm.lvm = path
	}
}

// WithSELinuxLabel sets the SELinux label to apply to the LVM lock directory.
func WithSELinuxLabel(label string) Option {
	return func(lvm *LVM) {
		lvm.selinuxLabel = label
	}
}

// init performs initial setup for the LVM instance, such as creating the lock directory and applying SELinux labels.
func (lvm *LVM) init() error {
	if _, err := os.Stat(runDir); err != nil {
		return fmt.Errorf("%s directory does not exist: %w", runDir, err)
	}

	if err := os.MkdirAll(lvmLockDir, 0o755); err != nil {
		return fmt.Errorf("failed to create LVM lock directory %q: %w", lvmLockDir, err)
	}

	if lvm.selinuxLabel != "" {
		if err := selinux.SetLabel(lvmLockDir, lvm.selinuxLabel); err != nil {
			return fmt.Errorf("failed to set SELinux label on LVM lock directory: %w", err)
		}
	}

	return nil
}

// run executes `lvm <subcommand> <args...>` and returns stdout.
//
// Full stdout capture is required because the JSON report for a node with
// many LVs/PVs can easily exceed the 4 KiB circular buffer used by
// RunWithOptions by default.
//
// Errors are normalised through classifyError so every caller sees the same
// sentinel set (ErrNotFound, ErrInUse, ErrNotEmpty, ErrOpen,
// ErrInvalidCommand, ErrInitFailed, ErrCommand). The raw lvm stderr is kept
// out of the returned error chain — only the sentinel is wrapped — so it
// will not be surfaced to API clients by mistake.
func (lvm *LVM) run(ctx context.Context, subcommand string, args ...string) (string, error) {
	out, err := cmd.RunWithOptions(
		ctx,
		lvm.lvm,
		append([]string{subcommand}, args...),
		cmd.WithFullStdoutCapture(),
	)
	if err != nil {
		return "", fmt.Errorf("lvm %s failed: %w", subcommand, classifyError(err))
	}

	return out, nil
}

// commonReportArgs is the shared flag set used by every JSON-reporting query:
// scan every LV/PV/VG including hidden internal ones (-a), emit every
// available column (-o +all), report in bytes without unit suffixes, and
// produce machine-readable JSON.
//
// --readonly is intentionally NOT used: in that mode LVM skips the
// device-mapper ioctl path it normally uses to populate kernel-state
// columns, so lv_active / lv_device_open / lv_suspended / lv_permissions
// come back as the literal string "unknown" regardless of the real state.
// The default shared (read) lock taken by lvs/pvs/vgs is cheap and lets
// LVM query DM, which is the only way to get accurate active/open state.
var commonReportArgs = []string{
	"-a",
	"-o", "+all",
	"--reportformat", "json",
	"--units", "b",
	"--nosuffix",
}

// decodeReport unmarshals an LVM JSON report and returns the slice of
// records stored under `report[*].<section>[*]`.
//
// LVM emits its reports wrapped in a fixed envelope
//
//	{ "report": [ { "<section>": [ <records> ] } ] }
//
// so the per-resource helpers only have to provide the section name (e.g.
// "lv", "pv", "vg") and a record type whose json tags map to the column
// names emitted by lvs(8)/pvs(8)/vgs(8).
func decodeReport[T any](out, section string) ([]T, error) {
	var env struct {
		Report []map[string][]T `json:"report"`
	}

	if err := json.Unmarshal([]byte(out), &env); err != nil {
		return nil, err
	}

	var records []T

	for _, r := range env.Report {
		records = append(records, r[section]...)
	}

	return records, nil
}

// Tags is a comma-separated tag list column emitted by lvs/pvs/vgs.
type Tags []string

// UnmarshalJSON splits the comma-separated value into a slice; empty input
// yields a nil slice.
func (t *Tags) UnmarshalJSON(data []byte) error {
	s, err := unquote(data)
	if err != nil {
		return err
	}

	s = strings.TrimSpace(s)
	if s == "" {
		*t = nil

		return nil
	}

	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}

	*t = out

	return nil
}

// unquote strips JSON quoting from a single string value; null is treated as
// the empty string.
func unquote(data []byte) (string, error) {
	if string(data) == "null" {
		return "", nil
	}

	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return "", err
	}

	return s, nil
}
