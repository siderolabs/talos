// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sdboot

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/efivarfs"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// TalosBootEntryDescription is the description of the Talos Linux UKI UEFI boot entry.
const TalosBootEntryDescription = "Talos Linux UKI"

// SystemdBootStubInfoPath is the path to the SystemdBoot StubInfo EFI variable.
var SystemdBootStubInfoPath = constants.EFIVarsMountPoint + "/" + "StubInfo-" + efivarfs.ScopeSystemd.String()

// Variable names.
const (
	LoaderEntryDefaultName  = "LoaderEntryDefault"
	LoaderEntrySelectedName = "LoaderEntrySelected"
	LoaderConfigTimeoutName = "LoaderConfigTimeout"

	StubImageIdentifierName = "StubImageIdentifier"
)

// ReadVariable reads a SystemdBoot EFI variable.
func ReadVariable(name string) (string, error) {
	data, _, err := efivarfs.Read(efivarfs.ScopeSystemd, name)
	if err != nil {
		// if the variable does not exist, return an empty string
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}

		return "", err
	}

	out := make([]byte, len(data))

	decoder := efivarfs.Encoding.NewDecoder()

	n, _, err := decoder.Transform(out, data, true)
	if err != nil {
		return "", err
	}

	if n > 0 && out[n-1] == 0 {
		n--
	}

	return string(out[:n]), nil
}

// WriteVariable reads a SystemdBoot EFI variable.
func WriteVariable(name, value string) error {
	// mount EFI vars as rw
	if err := unix.Mount("efivarfs", constants.EFIVarsMountPoint, "efivarfs", unix.MS_REMOUNT, ""); err != nil {
		return err
	}

	defer unix.Mount("efivarfs", constants.EFIVarsMountPoint, "efivarfs", unix.MS_REMOUNT|unix.MS_RDONLY, "") //nolint:errcheck

	out := make([]byte, (len(value)+1)*2)

	encoder := efivarfs.Encoding.NewEncoder()

	n, _, err := encoder.Transform(out, []byte(value), true)
	if err != nil {
		return err
	}

	out = append(out[:n], 0, 0)

	return efivarfs.Write(efivarfs.ScopeSystemd, name, efivarfs.AttrBootserviceAccess|efivarfs.AttrRuntimeAccess|efivarfs.AttrNonVolatile, out)
}

// CreateBootEntry creates a UEFI boot entry named "Talos Linux UKI" and sets it as the first in the `BootOrder`
// with the specified install disk and architecture.
// The entry will point to the SystemdBoot PE binary located at the specified install disk path.
//
//nolint:gocyclo
func CreateBootEntry(installDisk, sdBootFilePath string) error {
	// mount EFI vars as rw
	if err := unix.Mount("efivarfs", constants.EFIVarsMountPoint, "efivarfs", unix.MS_REMOUNT, ""); err != nil {
		return err
	}

	defer unix.Mount("efivarfs", constants.EFIVarsMountPoint, "efivarfs", unix.MS_REMOUNT|unix.MS_RDONLY, "") //nolint:errcheck

	rawBootOrderData, _, err := efivarfs.Read(efivarfs.ScopeGlobal, "BootOrder")
	if err != nil {
		return fmt.Errorf("failed to read BootOrder: %w", err)
	}

	bootOrder, err := efivarfs.UnmarshalBootOrder(rawBootOrderData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal BootOrder: %w", err)
	}

	// Let's assume we are adding a new boot entry for Talos Linux UKI.
	talosBootIndex := len(bootOrder)

	for _, idx := range bootOrder {
		bootEntry, err := efivarfs.GetBootEntry(int(idx))
		if err != nil {
			return fmt.Errorf("failed to get boot entry %d: %w", idx, err)
		}

		if bootEntry.Description == TalosBootEntryDescription {
			// If we already have a Talos Linux UKI boot entry, we will use its index.
			// This allows us to update the existing entry instead of creating a new one.
			talosBootIndex = int(idx)

			break
		}
	}

	info, err := blkid.ProbePath(installDisk, blkid.WithSkipLocking(true))
	if err != nil {
		return fmt.Errorf("failed to probe install disk %q: %w", installDisk, err)
	}

	efiPartInfo := xslices.Filter(info.Parts, func(part blkid.NestedProbeResult) bool {
		return part.PartitionLabel != nil && *part.PartitionLabel == constants.EFIPartitionLabel
	})

	if len(efiPartInfo) == 0 {
		return fmt.Errorf("EFI partition not found on install disk %q", installDisk)
	}

	if len(efiPartInfo) > 1 {
		return fmt.Errorf("multiple EFI partitions found on install disk %q, expected only one", installDisk)
	}

	partitionUUID := efiPartInfo[0].PartitionUUID

	if partitionUUID == nil {
		return fmt.Errorf("EFI partition UUID not found on install disk %q", installDisk)
	}

	if err := efivarfs.SetBootEntry(talosBootIndex, &efivarfs.LoadOption{
		Description: TalosBootEntryDescription,
		FilePath: efivarfs.DevicePath{
			&efivarfs.HardDrivePath{
				PartitionNumber:     uint32(efiPartInfo[0].PartitionIndex),
				PartitionStartBlock: efiPartInfo[0].PartitionOffset / uint64(info.SectorSize),
				PartitionSizeBlocks: efiPartInfo[0].PartitionSize / uint64(info.SectorSize),
				PartitionMatch: &efivarfs.PartitionGPT{
					PartitionUUID: *partitionUUID,
				},
			},
			efivarfs.FilePath("/" + sdBootFilePath),
		},
	}); err != nil {
		return fmt.Errorf("failed to set boot entry %d: %w", talosBootIndex, err)
	}

	currentBootOrder, err := efivarfs.GetBootOrder()
	if err != nil {
		return fmt.Errorf("failed to get current BootOrder: %w", err)
	}

	if currentBootOrder[0] == uint16(talosBootIndex) {
		// Talos Linux UKI boot entry is already first in the BootOrder
		return nil
	}

	if err := efivarfs.SetBootOrder(slices.Concat([]uint16{uint16(talosBootIndex)}, currentBootOrder)); err != nil {
		return fmt.Errorf("failed to set BootOrder: %w", err)
	}

	return nil
}
