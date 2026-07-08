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

var (
	sysBlockDir = "/sys/block"
	byIDDir     = "/dev/disk/by-id"
)

const mdHomeHost = "talos"

// SyncAction is the current MD sync action reported by sysfs.
type SyncAction string

const (
	// SyncActionIdle means no sync operation is running.
	SyncActionIdle SyncAction = "idle"
	// SyncActionResync means a resync is running.
	SyncActionResync SyncAction = "resync"
	// SyncActionRecover means recovery is running.
	SyncActionRecover SyncAction = "recover"
	// SyncActionCheck means a consistency check is running.
	SyncActionCheck SyncAction = "check"
	// SyncActionRepair means a repair is running.
	SyncActionRepair SyncAction = "repair"
	// SyncActionReshape means a reshape is running.
	SyncActionReshape SyncAction = "reshape"
	// SyncActionFrozen means sync operations are frozen.
	SyncActionFrozen SyncAction = "frozen"
)

// DevicePath returns the stable by-id path for an MD array name.
func DevicePath(name string) string {
	return byIDDir + "/md-name-" + mdHomeHost + ":" + name
}

// FindDeviceByMember returns the /dev/mdN node that contains member.
func (*MD) FindDeviceByMember(member string) (string, error) {
	return FindDeviceByMember(member)
}

// FindDeviceByMember returns the /dev/mdN node that contains member.
func FindDeviceByMember(member string) (string, error) {
	base := filepath.Base(member)

	entries, err := os.ReadDir(sysBlockDir)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", sysBlockDir, err)
	}

	for _, e := range entries {
		dev := e.Name()
		if !strings.HasPrefix(dev, "md") {
			continue
		}

		if _, err := os.Stat(filepath.Join(sysBlockDir, dev, "md", "dev-"+base)); err == nil {
			return filepath.Join("/dev", dev), nil
		}
	}

	return "", ErrNotFound
}

func freeMDNode() string {
	for n := range 128 {
		name := fmt.Sprintf("md%d", n)

		if _, err := os.Stat(filepath.Join(sysBlockDir, name)); os.IsNotExist(err) {
			return "/dev/" + name
		}
	}

	return "/dev/md0"
}

// Detail is the parsed subset of mdadm --detail --export output.
type Detail struct {
	// Level is the observed MD level, e.g. raid1.
	Level string
	// RaidDevices is the active RAID device count.
	RaidDevices int
	// UUID is the stable MD array UUID.
	UUID string
	// Name is the metadata-stamped array name.
	Name string
	// DevName is the mdadm map name, if known.
	DevName string
	// Metadata is the MD metadata format/version.
	Metadata string
	// ReshapeActive reports mdadm's reshape-active flag when exported.
	ReshapeActive string
	// Members is the list of attached member devices.
	Members []string
	// MemberRoles maps member device path to mdadm's role value (number or spare).
	MemberRoles map[string]string
}

// InactiveArrays returns assembled MD arrays whose sysfs state is inactive.
func (*MD) InactiveArrays() ([]string, error) {
	return InactiveArrays()
}

// InactiveArrays returns assembled MD arrays whose sysfs state is inactive.
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

// ArrayStateForDevice returns the current array state for an MD device.
func (*MD) ArrayStateForDevice(device string) (string, error) {
	return ArrayStateForDevice(device)
}

// ArrayStateForDevice returns the current array state for an MD device.
func ArrayStateForDevice(device string) (string, error) {
	return readMDAttribute(device, "array_state")
}

// SyncActionForDevice returns the current sync action for an MD device.
func (*MD) SyncActionForDevice(device string) (SyncAction, error) {
	return SyncActionForDevice(device)
}

// SyncActionForDevice returns the current sync action for an MD device.
func SyncActionForDevice(device string) (SyncAction, error) {
	if device == "" {
		return "", fmt.Errorf("%w: device must be set", ErrInvalidArgument)
	}

	action, err := readMDAttribute(device, "sync_action")
	if err != nil {
		return "", err
	}

	return SyncAction(action), nil
}

func readMDAttribute(device, attr string) (string, error) {
	if device == "" {
		return "", fmt.Errorf("%w: device must be set", ErrInvalidArgument)
	}

	out, err := os.ReadFile(filepath.Join(sysBlockDir, filepath.Base(device), "md", attr))
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}

		return "", err
	}

	return strings.TrimSpace(string(out)), nil
}

// IsSyncing reports whether an MD device is currently doing sync work.
func (*MD) IsSyncing(device string) (bool, error) {
	return IsSyncing(device)
}

// IsSyncing reports whether an MD device is currently doing sync work.
func IsSyncing(device string) (bool, error) {
	action, err := SyncActionForDevice(device)
	if err != nil {
		return false, err
	}

	return action != "" && action != SyncActionIdle, nil
}

// RunArray force-starts an assembled but inactive array.
func (md *MD) RunArray(ctx context.Context, device string) error {
	if device == "" {
		return fmt.Errorf("%w: device must be set", ErrInvalidArgument)
	}

	_, err := md.run(ctx, "--run", device)

	return err
}

// CreateOptions configures Create.
type CreateOptions struct {
	// Level is the mdadm RAID level.
	Level int
	// Metadata is the mdadm metadata format (e.g. "1.0"); empty lets mdadm pick its default.
	Metadata string
	// RaidDevices is the number of active member slots.
	RaidDevices int
	// Devices are the member block devices.
	Devices []string
}

// Create creates a new MD array and returns its /dev/mdN node.
func (md *MD) Create(ctx context.Context, name string, opts CreateOptions) (string, error) {
	if name == "" {
		return "", fmt.Errorf("%w: name must be set", ErrInvalidArgument)
	}

	if len(opts.Devices) == 0 {
		return "", fmt.Errorf("%w: at least one device is required", ErrInvalidArgument)
	}

	node := freeMDNode()
	args := make([]string, 0, 8+len(opts.Devices))
	args = append(args,
		"--create", node,
		"--name", name,
		"--homehost="+mdHomeHost,
		"--run",
		"--assume-clean",
		"--level="+strconv.Itoa(opts.Level),
		"--raid-devices="+strconv.Itoa(opts.RaidDevices),
	)

	if opts.Metadata != "" {
		args = append(args, "--metadata="+opts.Metadata)
	}

	args = append(args, opts.Devices...)

	if _, err := md.run(ctx, args...); err != nil {
		return "", err
	}

	return node, nil
}

// Add attaches member devices to an existing MD array.
func (md *MD) Add(ctx context.Context, device string, members ...string) error {
	if device == "" {
		return fmt.Errorf("%w: device must be set", ErrInvalidArgument)
	}

	if len(members) == 0 {
		return fmt.Errorf("%w: at least one member is required", ErrInvalidArgument)
	}

	_, err := md.run(ctx, append([]string{"--add", device}, members...)...)

	return err
}

// Grow changes the active RAID device count for an MD array.
func (md *MD) Grow(ctx context.Context, device string, raidDevices int) error {
	if device == "" {
		return fmt.Errorf("%w: device must be set", ErrInvalidArgument)
	}

	if raidDevices < 1 {
		return fmt.Errorf("%w: raid devices must be positive", ErrInvalidArgument)
	}

	_, err := md.run(ctx, "--grow", device, "--raid-devices="+strconv.Itoa(raidDevices))

	return err
}

// Fail marks a member device failed in an MD array.
func (md *MD) Fail(ctx context.Context, device, member string) error {
	if device == "" || member == "" {
		return fmt.Errorf("%w: device and member must be set", ErrInvalidArgument)
	}

	_, err := md.run(ctx, device, "--fail", member)

	return err
}

// Remove detaches a member device from an MD array.
func (md *MD) Remove(ctx context.Context, device, member string) error {
	if device == "" || member == "" {
		return fmt.Errorf("%w: device and member must be set", ErrInvalidArgument)
	}

	_, err := md.run(ctx, device, "--remove", member)

	return err
}

// Stop stops an MD array.
func (md *MD) Stop(ctx context.Context, device string) error {
	if device == "" {
		return fmt.Errorf("%w: device must be set", ErrInvalidArgument)
	}

	_, err := md.run(ctx, "--stop", device)

	return err
}

// ZeroSuperblock clears MD metadata from member devices.
func (md *MD) ZeroSuperblock(ctx context.Context, members ...string) error {
	if len(members) == 0 {
		return fmt.Errorf("%w: at least one member is required", ErrInvalidArgument)
	}

	_, err := md.run(ctx, append([]string{"--zero-superblock"}, members...)...)

	return err
}

// DetailDevice returns parsed mdadm --detail --export information for an array.
func (md *MD) DetailDevice(ctx context.Context, device string) (Detail, error) {
	out, err := md.run(ctx, "--detail", "--export", device)
	if err != nil {
		return Detail{}, err
	}

	return parseDetailExport(out), nil
}

func parseDetailExport(out string) Detail {
	d := Detail{MemberRoles: map[string]string{}}
	roles := map[string]string{}
	devices := map[string]string{}

	for line := range strings.SplitSeq(out, "\n") {
		parseDetailLine(line, &d, devices, roles)
	}

	for key, dev := range devices {
		if role, ok := roles[key]; ok {
			d.MemberRoles[dev] = role
		}
	}

	return d
}

func parseDetailLine(line string, d *Detail, devices, roles map[string]string) {
	key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
	if !ok {
		return
	}

	if parseDetailScalar(key, value, d) {
		return
	}

	parseDetailMember(key, value, d, devices, roles)
}

func parseDetailScalar(key, value string, d *Detail) bool {
	switch key {
	case "MD_LEVEL":
		d.Level = value
	case "MD_DEVICES":
		if n, err := strconv.Atoi(value); err == nil {
			d.RaidDevices = n
		}
	case "MD_UUID":
		d.UUID = value
	case "MD_NAME":
		d.Name = value
	case "MD_DEVNAME":
		d.DevName = value
	case "MD_METADATA":
		d.Metadata = value
	case "MD_RESHAPE_ACTIVE":
		d.ReshapeActive = value
	default:
		return false
	}

	return true
}

func parseDetailMember(key, value string, d *Detail, devices, roles map[string]string) {
	if !strings.HasPrefix(key, "MD_DEVICE_") {
		return
	}

	switch {
	case strings.HasSuffix(key, "_DEV") && value != "":
		memberKey := strings.TrimSuffix(strings.TrimPrefix(key, "MD_DEVICE_"), "_DEV")
		devices[memberKey] = value
		d.Members = append(d.Members, value)
	case strings.HasSuffix(key, "_ROLE"):
		memberKey := strings.TrimSuffix(strings.TrimPrefix(key, "MD_DEVICE_"), "_ROLE")
		roles[memberKey] = value
	}
}

// ExtendOptions configures Extend.
type ExtendOptions struct {
	// Devices are member devices to add before growing.
	Devices []string
	// RaidDevices is the target active RAID device count.
	RaidDevices int
}

// Extend adds member devices and grows the active RAID device count.
func (md *MD) Extend(ctx context.Context, dev string, opts ExtendOptions) error {
	if len(opts.Devices) > 0 {
		if err := md.Add(ctx, dev, opts.Devices...); err != nil {
			return err
		}
	}

	detail, err := md.DetailDevice(ctx, dev)
	if err != nil {
		return err
	}

	if detail.RaidDevices >= opts.RaidDevices {
		return nil
	}

	return md.Grow(ctx, dev, opts.RaidDevices)
}

// Shrink removes member devices and reduces the active RAID device count.
func (md *MD) Shrink(ctx context.Context, dev string, devices []string) error {
	detail, err := md.DetailDevice(ctx, dev)
	if err != nil {
		return err
	}

	target := detail.RaidDevices - len(devices)
	if target < 1 {
		return fmt.Errorf("%w: cannot shrink array below one active device", ErrInvalidArgument)
	}

	for _, d := range devices {
		if err := md.Fail(ctx, dev, d); err != nil {
			return err
		}

		if err := md.Remove(ctx, dev, d); err != nil {
			return err
		}
	}

	if err := md.Grow(ctx, dev, target); err != nil {
		return err
	}

	return md.ZeroSuperblock(ctx, devices...)
}

// Destroy stops an MD array and clears member superblocks.
func (md *MD) Destroy(ctx context.Context, device string) error {
	detail, err := md.DetailDevice(ctx, device)
	if err != nil {
		return err
	}

	if err := md.Stop(ctx, device); err != nil {
		return err
	}

	if len(detail.Members) == 0 {
		return nil
	}

	return md.ZeroSuperblock(ctx, detail.Members...)
}
