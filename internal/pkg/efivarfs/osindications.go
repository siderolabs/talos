// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Copyright The Monogon Project Authors.
// SPDX-License-Identifier: Apache-2.0

package efivarfs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sync"
)

// OSIndications is a bitset used to indicate firmware support for various
// features as well as to trigger some of these features.
// If a constant ends in Supported, it cannot be triggered, the others
// can be.
type OSIndications uint64

const (
	// BootToFirmwareUI indicates that on next boot firmware should boot to a firmware-provided
	// UI instead of the normal boot order.
	BootToFirmwareUI = OSIndications(1 << iota)
	// TimestampRevocationSupported indicates that firmware supports timestamp-based revocation and the
	// "dbt" authorized timestamp database variable.
	TimestampRevocationSupported
	// FileCapsuleDelivery indicates that on next boot firmware should look for an EFI update
	// capsule on an EFI system partition and try to install it.
	FileCapsuleDelivery
	// FirmwareManagementProtocolCapsuleSupported indicates that firmware supports UEFI FMP update capsules.
	FirmwareManagementProtocolCapsuleSupported
	// CapsuleResultVarSupported indicates that firmware supports reporting results of deferred (i.e.
	// processed on next boot) capsule installs via variables.
	CapsuleResultVarSupported
	// StartOSRecovery indicates that firmware should skip Boot# processing on next boot
	// and instead use OsRecovery# for selecting a load option.
	StartOSRecovery
	// StartPlatformRecovery indicates that firmware should skip Boot# processing on next boot
	// and instead use PlatformRecovery# for selecting a load option.
	StartPlatformRecovery
	// JSONConfigDataRefresh indicates that firmware should collect the current config and report
	// the data to the EFI system configuration table on next boot.
	JSONConfigDataRefresh
)

// osIndicationMutex protects against race conditions in read-modify-write
// sequences on the OsIndications EFI variable.
var osIndicationsMutex sync.Mutex

// OSIndicationsSupported indicates which of the OS indication features and
// actions that the firmware supports.
func OSIndicationsSupported() (OSIndications, error) {
	osIndicationsRaw, _, err := Read(ScopeGlobal, "OsIndicationsSupported")
	if err != nil {
		return 0, fmt.Errorf("unable to read OsIndicationsSupported: %w", err)
	}

	if len(osIndicationsRaw) != 8 {
		return 0, fmt.Errorf("value of OsIndicationsSupported is not 8 bytes / 64 bits, is %d bytes", len(osIndicationsRaw))
	}

	return OSIndications(binary.LittleEndian.Uint64(osIndicationsRaw)), nil
}

// SetOSIndications sets all OS indication bits set in i in firmware. It does
// not clear any already-set bits, use ClearOSIndications for that.
func SetOSIndications(i OSIndications) error {
	return modifyOSIndications(func(prev OSIndications) OSIndications {
		return prev | i
	})
}

// ClearOSIndications clears all OS indication bits set in i in firmware.
// Note that this effectively inverts i, bits set in i will be cleared.
func ClearOSIndications(i OSIndications) error {
	return modifyOSIndications(func(prev OSIndications) OSIndications {
		return prev & ^i
	})
}

func modifyOSIndications(f func(prev OSIndications) OSIndications) error {
	osIndicationsMutex.Lock()
	defer osIndicationsMutex.Unlock()

	var osIndications OSIndications

	rawIn, _, err := Read(ScopeGlobal, "OsIndications")
	if err == nil && len(rawIn) == 8 {
		osIndications = OSIndications(binary.LittleEndian.Uint64(rawIn))
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("unable to read OsIndications variable: %w", err)
	}

	osIndications = f(osIndications)

	var raw [8]byte
	binary.LittleEndian.PutUint64(raw[:], uint64(osIndications))

	if err := Write(ScopeGlobal, "OsIndications", AttrNonVolatile|AttrRuntimeAccess, raw[:]); err != nil {
		return fmt.Errorf("failed to write OSIndications variable: %w", err)
	}

	return nil
}
