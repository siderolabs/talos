// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sdboot

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"math"
	"os"
	"slices"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/blkid"

	"github.com/siderolabs/talos/internal/pkg/efivarfs"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// TalosBootEntryDescription is the description of the Talos Linux UKI UEFI boot entry.
const TalosBootEntryDescription = "Talos Linux UKI"

// SystemdBootStubInfoPath is the path to the SystemdBoot StubInfo EFI variable.
var SystemdBootStubInfoPath = constants.EFIVarsMountPoint + "/" + "StubInfo-" + efivarfs.ScopeSystemd.String()

// Variable names.
const (
	LoaderConfigTimeoutName     = "LoaderConfigTimeout"
	LoaderEntryDefaultName      = "LoaderEntryDefault"
	LoaderEntryOneShotName      = "LoaderEntryOneShot"
	LoaderEntryRebootReasonName = "LoaderEntryRebootReason"
	LoaderEntrySelectedName     = "LoaderEntrySelected"

	StubImageIdentifierName = "StubImageIdentifier"
)

// ReadVariable reads a SystemdBoot EFI variable.
func ReadVariable(name string) (string, error) {
	efi, err := efivarfs.NewFilesystemReaderWriter(false)
	if err != nil {
		return "", fmt.Errorf("failed to create efivarfs reader/writer: %w", err)
	}

	defer efi.Close() //nolint:errcheck

	data, _, err := efi.Read(efivarfs.ScopeSystemd, name)
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
	efi, err := efivarfs.NewFilesystemReaderWriter(true)
	if err != nil {
		return fmt.Errorf("failed to create efivarfs reader/writer: %w", err)
	}

	defer efi.Close() //nolint:errcheck

	out := make([]byte, (len(value)+1)*2)

	encoder := efivarfs.Encoding.NewEncoder()

	n, _, err := encoder.Transform(out, []byte(value), true)
	if err != nil {
		return err
	}

	out = append(out[:n], 0, 0)

	return efi.Write(efivarfs.ScopeSystemd, name, efivarfs.AttrBootserviceAccess|efivarfs.AttrRuntimeAccess|efivarfs.AttrNonVolatile, out)
}

// CreateBootEntry creates a UEFI boot entry named "Talos Linux UKI" and sets it as the first in the `BootOrder`
// The entry will point to the SystemdBoot PE binary located at the specified install disk path.
//
//nolint:gocyclo,cyclop
func CreateBootEntry(rw efivarfs.ReadWriter, blkidInfo *blkid.Info, printf func(format string, args ...interface{}), sdBootFilePath string) error {
	efiPartInfo := xslices.Filter(blkidInfo.Parts, func(part blkid.NestedProbeResult) bool {
		return part.PartitionLabel != nil && *part.PartitionLabel == constants.EFIPartitionLabel
	})

	if len(efiPartInfo) == 0 {
		return fmt.Errorf("EFI partition not found on install disk %q", blkidInfo.Name)
	}

	if len(efiPartInfo) > 1 {
		return fmt.Errorf("multiple EFI partitions found on install disk %q, expected only one", blkidInfo.Name)
	}

	partitionUUID := efiPartInfo[0].PartitionUUID

	if partitionUUID == nil {
		return fmt.Errorf("EFI partition UUID not found on install disk %q", blkidInfo.Name)
	}

	printf("using disk %s with partition %d and UUID %s", blkidInfo.Name, efiPartInfo[0].PartitionIndex, partitionUUID.String())

	bootOrder, err := efivarfs.GetBootOrder(rw)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			bootOrder = efivarfs.BootOrder{}
		} else {
			return fmt.Errorf("failed to get BootOrder: %w", err)
		}
	}

	printf("Current BootOrder: %v", bootOrder)

	bootEntries, err := efivarfs.ListBootEntries(rw)
	if err != nil {
		return fmt.Errorf("failed to list existing Talos boot entries: %w", err)
	}

	printf("Existing boot entries: %v", slices.Collect(maps.Keys(bootEntries)))

	var existingTalosBootEntryIndexes []int

	// Find all boot entries with the Talos Linux UKI description.
	for idx, entry := range bootEntries {
		if entry.Description == TalosBootEntryDescription {
			existingTalosBootEntryIndexes = append(existingTalosBootEntryIndexes, idx)
		}
	}

	// we sort the indexes to make sure we always keep the lowest index
	// when removing duplicate Talos Linux UKI boot entries
	slices.Sort(existingTalosBootEntryIndexes)

	printf("Found existing Talos Linux UKI boot entries: %v", existingTalosBootEntryIndexes)

	// Remove any existing Talos Linux UKI boot entries from the BootOrder.
	// We need to do this since Talos 1.11.x release assumed that the boot order set by the code stays even after a reboot,
	// but UEFI firmware settings can set a different boot order on boot, which lead to multiple Talos Linux UKI entries in the boot order,
	// causing some UEFI firmwares to fail to boot at all.
	// See https://github.com/siderolabs/talos/issues/11829

	// find the next minimal available index for the new Talos Linux UKI boot entry
	nextMinimalIndex := -1

	for i := range math.MaxUint16 {
		if _, ok := bootEntries[i]; !ok {
			nextMinimalIndex = i

			break
		}
	}

	if nextMinimalIndex == -1 {
		return errors.New("all 2^16 boot entry variables are occupied")
	}

	// remove all existing Talos Linux UKI boot entries except the first one
	// and use its index for the new/updated entry
	for i, idx := range existingTalosBootEntryIndexes {
		if i == 0 {
			nextMinimalIndex = idx

			continue
		}

		printf("Removing existing Talos Linux UKI boot entry at index %d", idx)

		if err := efivarfs.DeleteBootEntry(rw, idx); err != nil {
			return fmt.Errorf("failed to delete existing Talos boot entry at index %d: %w", idx, err)
		}
	}

	if err := efivarfs.SetBootEntry(rw, nextMinimalIndex, &efivarfs.LoadOption{
		Description: TalosBootEntryDescription,
		FilePath: efivarfs.DevicePath{
			&efivarfs.HardDrivePath{
				PartitionNumber:     uint32(efiPartInfo[0].PartitionIndex),
				PartitionStartBlock: efiPartInfo[0].PartitionOffset / uint64(blkidInfo.SectorSize),
				PartitionSizeBlocks: efiPartInfo[0].PartitionSize / uint64(blkidInfo.SectorSize),
				PartitionMatch: &efivarfs.PartitionGPT{
					PartitionUUID: *partitionUUID,
				},
			},
			efivarfs.FilePath("/" + sdBootFilePath),
		},
	}); err != nil {
		return fmt.Errorf("failed to create Talos Linux UKI boot entry at index %d: %w", nextMinimalIndex, err)
	}

	printf("created Talos Linux UKI boot entry at index %d", nextMinimalIndex)

	return nil
}
