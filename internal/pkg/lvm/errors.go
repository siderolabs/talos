// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvm

import (
	"errors"
	"regexp"

	"github.com/siderolabs/go-cmd/pkg/cmd"
)

// LVM exit codes, mirroring tools/errors.h in the upstream source.
//
// Note that ECMD_PROCESSED (1) is translated to 0 by lvm_return_code() before
// the process exits, so callers will never observe 1.
const (
	_                  = 1 // ECMD_PROCESSED - never seen by callers; rewritten to 0 by lvm_return_code()
	exitNoSuchCmd      = 2 // ENO_SUCH_CMD       - unknown subcommand
	exitInvalidCmdLine = 3 // EINVALID_CMD_LINE  - bad/unknown flags or args
	exitInitFailed     = 4 // EINIT_FAILED       - lock/config/setup failure
	_                  = 5 // ECMD_FAILED        - every operational failure; handled in sentinelFor default branch
)

// Sentinel errors returned by the package. Callers should use errors.Is to
// dispatch on these rather than parsing the underlying lvm output, which is
// human-oriented and may change between LVM releases.
//
// The split between exit-code-derived sentinels (ErrInvalidCommand,
// ErrInitFailed) and stderr-derived ones (ErrNotFound, ErrInUse, ...) is
// driven by LVM2 itself: it collapses every operational failure into
// ECMD_FAILED=5, so exit codes can only distinguish "programmer/environment
// failure" from "the LVM rejected the operation". The actual reason has to
// be recovered from the human-oriented stderr.
var (
	// ErrNotFound is returned when the target VG/LV/PV does not exist.
	ErrNotFound = errors.New("lvm: resource not found")
	// ErrInUse is returned when a PV is still claimed by a VG, or an LV
	// has active holders / a mounted filesystem.
	ErrInUse = errors.New("lvm: resource in use")
	// ErrDevicePartitioned is returned when pvcreate is asked to initialize a
	// whole disk that carries a partition table. The disk's partitions should
	// be used as PVs instead.
	ErrDevicePartitioned = errors.New("lvm: device is partitioned")
	// ErrExists is returned when a create operation targets an object that
	// already exists (VG/LV already present, device already a PV). Callers
	// reconciling towards desired state can treat this as success.
	ErrExists = errors.New("lvm: resource already exists")
	// ErrNotEmpty is returned when a VG still contains logical volumes.
	ErrNotEmpty = errors.New("lvm: resource not empty")
	// ErrOpen is returned when the LV is open (mounted, snapshot origin,
	// pool data/metadata, RAID sub-LV, etc).
	ErrOpen = errors.New("lvm: logical volume is open")
	// ErrInvalidCommand is returned when LVM rejected the command itself
	// (unknown subcommand or invalid arguments). This indicates a Talos
	// bug rather than an end-user condition.
	ErrInvalidCommand = errors.New("lvm: invalid command")
	// ErrInitFailed is returned when LVM could not initialize its
	// environment (lock directory, config, ...). Operator action on the
	// host may be required.
	ErrInitFailed = errors.New("lvm: initialization failed")
	// ErrCommand is returned for any non-zero exit that does not match a
	// more specific sentinel.
	ErrCommand = errors.New("lvm: command failed")
)

// stderr matchers below are derived from the upstream LVM2 source tree.
// Each is annotated with the source file:line that emits the string
// so future maintainers can re-validate against a new release. The patterns
// are deliberately narrow - a false positive that misclassifies, say, a
// transient I/O error as ErrNotFound would be worse than collapsing to
// ErrCommand.
var (
	// vgremove: "Volume group \"%s\" still contains %u logical volume(s)"
	//   lib/metadata/metadata.c:641
	vgNotEmptyRE = regexp.MustCompile(`still contains \d+ logical volume`)

	// pvremove: "PV %s is used by VG %s so please use vgreduce first."
	//   tools/toollib.c:5218
	// vgreduce: "Physical volume \"%s\" still in use"
	//   lib/metadata/vg.c:724
	pvInUseRE = regexp.MustCompile(`(is used by VG|still in use)`)

	// pvcreate against a whole disk with a partition table:
	//   "Cannot use %s: device is partitioned"
	//   reason string: lib/cache/lvmcache.c (DEV_FILTERED_PARTITIONED branch)
	//   flag set by:   lib/filters/filter-partitioned.c
	devicePartitionedRE = regexp.MustCompile(`device is partitioned`)

	// create-against-existing family (idempotent from a reconciler's view).
	// Verified against the lvm2 source tree:
	//   "A volume group called %s already exists."                       tools/vgcreate.c:93
	//   "Logical volume %s already exists in Volume group %s."           tools/lvcreate.c:1585
	//   "Physical volume '%s' is already in volume group '%s'"           tools/toollib.c:5177, lib/metadata/metadata.c:334
	//   "Can't initialize PV '%s' without -ff."                          tools/toollib.c:5173
	//   "Can't initialize physical volume \"%s\" of volume group ...     tools/toollib.c:5175
	alreadyExistsRE = regexp.MustCompile(`(already exists|is already in volume group|Can't initialize (physical volume|PV) )`)

	// lvremove path (lib/activate/activate.c:982,1006):
	//   "Logical volume %s contains a filesystem in use."
	//   "Logical volume %s in use."
	// lvm_manip.c "Can't remove logical volume %s used by ..." family
	//   (snapshot/external origin/mirror/RAID/pool/locked) - these are LV
	//   open in the sense that another LVM object is holding it.
	lvOpenRE = regexp.MustCompile(`(contains a filesystem in use|Logical volume \S+ in use|Can't remove (locked |merging snapshot )?logical volume \S+ (under snapshot|used by|used as))`)

	// vg/lv/pv not-found family:
	//   "Volume group \"%s\" not found"                 (lib/metadata/metadata.c)
	//   "Logical volume %s not found in volume group"   (tools/lvrename.c:49 etc)
	//   "No physical volume label read from %s"         (label scan)
	notFoundRE = regexp.MustCompile(`(not found|does not exist|No physical volume label read)`)
)

// ExecError carries the classified sentinel together with the raw exit code
// and stderr from a failed lvm subcommand. It is returned by (*LVM).run when
// the underlying command exits non-zero.
//
// The structured fields let server-side handlers log the full diagnostic
// (exit code, stderr, sentinel) without surfacing the raw lvm output to the
// API client — Error() only renders the sentinel for that reason.
//
// Callers may also `errors.Is(err, lvm.ErrXxx)` because Unwrap returns the
// embedded sentinel.
type ExecError struct {
	Sentinel error
	ExitCode int
	Stderr   []byte
}

// Error implements the error interface. Returns only the sentinel message so
// raw lvm stderr is never leaked through an %s or err.Error() call.
func (e *ExecError) Error() string {
	return e.Sentinel.Error()
}

// Unwrap returns the sentinel so errors.Is/As routes through it.
func (e *ExecError) Unwrap() error {
	return e.Sentinel
}

// classifyError maps a *cmd.ExitError to an *ExecError carrying the matching
// sentinel together with the raw exit code and stderr. Non-ExitError inputs
// (context cancellation, exec failures) are returned untouched so callers can
// still use errors.Is(err, context.Canceled).
//
// Order matters: stderr-based matchers run first because LVM collapses every
// operational failure into ECMD_FAILED=5, so the exit code alone cannot tell
// "VG not empty" from "PV in use" from "LV missing". Exit-code-derived
// sentinels handle only the cases where stderr is uninformative (unknown
// subcommand, malformed CLI, init failure).
func classifyError(err error) error {
	var exit *cmd.ExitError

	if !errors.As(err, &exit) {
		return err
	}

	sentinel := sentinelFor(exit)

	return &ExecError{
		Sentinel: sentinel,
		ExitCode: exit.ExitCode,
		Stderr:   exit.Output,
	}
}

// sentinelFor picks the most specific sentinel for an lvm ExitError, using
// stderr matchers first and falling back to exit-code categories.
func sentinelFor(exit *cmd.ExitError) error {
	if matched := matchStderr(exit.Output); matched != nil {
		return matched
	}

	switch exit.ExitCode {
	case exitNoSuchCmd, exitInvalidCmdLine:
		return ErrInvalidCommand
	case exitInitFailed:
		return ErrInitFailed
	}

	return ErrCommand
}

// matchStderr returns the sentinel that best describes the given lvm stderr,
// or nil if no narrow pattern fits.
func matchStderr(out []byte) error {
	switch {
	case alreadyExistsRE.Match(out):
		return ErrExists
	case devicePartitionedRE.Match(out):
		return ErrDevicePartitioned
	case vgNotEmptyRE.Match(out):
		return ErrNotEmpty
	case pvInUseRE.Match(out):
		return ErrInUse
	case lvOpenRE.Match(out):
		return ErrOpen
	case notFoundRE.Match(out):
		return ErrNotFound
	}

	return nil
}
