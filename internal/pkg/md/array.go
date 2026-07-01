// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package md

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// sysBlockDir is where the kernel exposes block devices, including md arrays
// and their per-array state under <name>/md/array_state.
const sysBlockDir = "/sys/block"

// DevicePath returns the stable, udev-managed reference for an array name,
// e.g. "mdboot" -> "/dev/disk/by-id/md-name-mdboot".
//
// Talos does not populate the /dev/md/<name> alias directory, so the kernel
// auto-assigns a numbered node (e.g. /dev/md127) at assembly time. The only
// name-stable path is the by-id symlink udev derives from the array name in
// the superblock, so every query/manage/destroy operation references the
// array through it rather than a guessed /dev/mdN.
func DevicePath(name string) string {
	return "/dev/disk/by-id/md-name-" + name
}

// freeMDNode returns the first unused numbered md device node (e.g. /dev/md0).
//
// `mdadm --create` is given a numbered node rather than /dev/md/<name>: the
// latter needs the /dev/md alias directory which is populated by udev, absent
// in the installer container. A numbered node is created directly by the kernel
// (devtmpfs), and its partitions appear as /dev/mdNpX without udev. The array
// name is still stamped via --name, so the udev-managed by-id symlink
// (DevicePath) appears on the running system.
func freeMDNode() string {
	for n := range 128 {
		name := fmt.Sprintf("md%d", n)

		if _, err := os.Stat(filepath.Join(sysBlockDir, name)); os.IsNotExist(err) {
			return "/dev/" + name
		}
	}

	return "/dev/md0"
}

// Detail is the subset of `mdadm --detail --export` output the package needs.
type Detail struct {
	// RaidDevices is the number of active member slots in the array
	// (MD_DEVICES), not counting spares.
	RaidDevices int
	// Members are the block-device paths of every device attached to the
	// array (MD_DEVICE_*_DEV).
	Members []string
}

// InactiveArrays returns the /dev paths of md arrays that are assembled but
// not running - `array_state == "inactive"` in sysfs. This is the state udev's
// incremental assembly leaves a RAID1 in when a member is missing: it waits
// for the absent disk instead of starting degraded.
//
// Reading sysfs needs no mdadm binary; force-running the result does need
// mdadm (see RunArray).
func InactiveArrays() ([]string, error) {
	entries, err := os.ReadDir(sysBlockDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", sysBlockDir, err)
	}

	var inactive []string

	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "md") {
			continue
		}

		// array_state only exists for real md arrays; its absence (or any read
		// error) means this is not an md device we should touch.
		state, err := os.ReadFile(filepath.Join(sysBlockDir, name, "md", "array_state"))
		if err != nil {
			continue
		}

		if strings.TrimSpace(string(state)) == "inactive" {
			inactive = append(inactive, filepath.Join("/dev", name))
		}
	}

	return inactive, nil
}

// RunArray force-starts an assembled-but-inactive array via `mdadm --run`.
//
// Talos has no systemd, so the mdadm-last-resort timer that normally starts a
// degraded array after a member-wait timeout never fires. This is the explicit
// replacement: starting a degraded RAID1 on the surviving member is what makes
// single-disk-failure boot survival actually work.
func (md *MD) RunArray(ctx context.Context, device string) error {
	if device == "" {
		return fmt.Errorf("%w: device must be set", ErrInvalidArgument)
	}

	_, err := md.run(ctx, "--run", device)

	return err
}

// Create provisions a new array at a free numbered md node and returns that
// device path (e.g. /dev/md0).
//
// Runs `mdadm --create /dev/mdN --name <name> --run --level=<level>
// --raid-devices=<n> <devices...>`. --run suppresses the interactive prompt;
// --name stamps the array name so the udev by-id symlink (DevicePath) appears
// on the running system. The returned node is the path the caller must address
// the array by in environments without udev (e.g. the installer).
func (md *MD) Create(ctx context.Context, name string, level, raidDevices int, devices []string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("%w: name must be set", ErrInvalidArgument)
	}

	if len(devices) == 0 {
		return "", fmt.Errorf("%w: at least one device is required", ErrInvalidArgument)
	}

	node := freeMDNode()

	args := make([]string, 0, 7+len(devices))
	args = append(args,
		"--create", node,
		"--name", name,
		"--run",
		"--level="+strconv.Itoa(level),
		"--raid-devices="+strconv.Itoa(raidDevices),
	)
	args = append(args, devices...)

	if _, err := md.run(ctx, args...); err != nil {
		return "", err
	}

	return node, nil
}

// Extend adds member devices to an existing array, growing the number of
// active devices.
//
// For RAID1 this increases the mirror count: each device is added (initially a
// spare) with `mdadm --add`, then `--grow --raid-devices` promotes them to
// active and triggers a resync.
func (md *MD) Extend(ctx context.Context, name string, devices []string) error {
	if len(devices) == 0 {
		return fmt.Errorf("%w: at least one device is required", ErrInvalidArgument)
	}

	dev := DevicePath(name)

	detail, err := md.Detail(ctx, name)
	if err != nil {
		return err
	}

	addArgs := append([]string{"--add", dev}, devices...)
	if _, err := md.run(ctx, addArgs...); err != nil {
		return err
	}

	target := detail.RaidDevices + len(devices)

	_, err = md.run(ctx, "--grow", dev, "--raid-devices="+strconv.Itoa(target))

	return err
}

// Shrink removes member devices from an existing array, reducing the number of
// active devices.
//
// Each device is failed and removed, the array is grown down to the new
// active-device count, and the removed members have their superblocks zeroed
// so the disks can be reused.
func (md *MD) Shrink(ctx context.Context, name string, devices []string) error {
	if len(devices) == 0 {
		return fmt.Errorf("%w: at least one device is required", ErrInvalidArgument)
	}

	dev := DevicePath(name)

	detail, err := md.Detail(ctx, name)
	if err != nil {
		return err
	}

	target := detail.RaidDevices - len(devices)
	if target < 1 {
		return fmt.Errorf("%w: cannot shrink array below one active device", ErrInvalidArgument)
	}

	for _, d := range devices {
		if _, err := md.run(ctx, dev, "--fail", d, "--remove", d); err != nil {
			return err
		}
	}

	if _, err := md.run(ctx, "--grow", dev, "--raid-devices="+strconv.Itoa(target)); err != nil {
		return err
	}

	zeroArgs := append([]string{"--zero-superblock"}, devices...)

	_, err = md.run(ctx, zeroArgs...)

	return err
}

// Destroy stops the array and clears the superblock on every member.
//
// Members are read with Detail before the array is stopped (afterwards mdadm
// can no longer enumerate them), then `--stop` releases the array and
// `--zero-superblock` wipes each member so its disk can be reused.
func (md *MD) Destroy(ctx context.Context, name string) error {
	dev := DevicePath(name)

	detail, err := md.Detail(ctx, name)
	if err != nil {
		return err
	}

	if _, err := md.run(ctx, "--stop", dev); err != nil {
		return err
	}

	if len(detail.Members) == 0 {
		return nil
	}

	zeroArgs := append([]string{"--zero-superblock"}, detail.Members...)

	_, err = md.run(ctx, zeroArgs...)

	return err
}

// Detail queries `mdadm --detail --export <dev>` and parses the export
// key=value output.
func (md *MD) Detail(ctx context.Context, name string) (Detail, error) {
	out, err := md.run(ctx, "--detail", "--export", DevicePath(name))
	if err != nil {
		return Detail{}, err
	}

	return parseDetailExport(out), nil
}

// parseDetailExport parses the `mdadm --detail --export` key=value format:
//
//	MD_LEVEL=raid1
//	MD_DEVICES=2
//	MD_DEVICE_dev_sda_DEV=/dev/sda
//	MD_DEVICE_dev_sda_ROLE=0
//	...
//
// MD_DEVICES gives the active-device count; every MD_DEVICE_*_DEV value is a
// member path.
func parseDetailExport(out string) Detail {
	var d Detail

	for line := range strings.SplitSeq(out, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}

		switch {
		case key == "MD_DEVICES":
			if n, err := strconv.Atoi(value); err == nil {
				d.RaidDevices = n
			}
		case strings.HasPrefix(key, "MD_DEVICE_") && strings.HasSuffix(key, "_DEV"):
			if value != "" {
				d.Members = append(d.Members, value)
			}
		}
	}

	return d
}
