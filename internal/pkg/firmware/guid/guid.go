// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package guid implements fwupd-compatible GUID generation from device instance IDs.
//
// LVFS firmware metadata references devices by GUIDs computed as RFC 4122 version 5
// (SHA-1) UUIDs over ASCII instance IDs (e.g. `NVME\VEN_1179&DEV_0115`) using the
// DNS namespace UUID, matching fwupd's fwupd_guid_hash_string().
package guid

import (
	"crypto/fips140"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// HashString returns the fwupd-compatible GUID for an instance ID.
//
// The SHA-1 use here is a name-derivation scheme (RFC 4122 v5 UUIDs) required
// for fwupd interoperability, not a cryptographic protection; bypass FIPS
// enforcement so the function is usable in FIPS-strict builds.
func HashString(instanceID string) string {
	var out string

	fips140.WithoutEnforcement(func() {
		out = uuid.NewSHA1(uuid.NameSpaceDNS, []byte(instanceID)).String()
	})

	return out
}

// NVMeInstanceIDs builds fwupd-compatible NVMe instance IDs from PCI IDs and model name.
//
// vid/did/subsysVID/subsysDID are PCI vendor/device/subsystem IDs; model is the stripped
// NVMe model number (MN) string. Zero IDs and empty model are skipped.
func NVMeInstanceIDs(vid, did, subsysVID, subsysDID uint16, model string) []string {
	var ids []string

	if vid != 0 && did != 0 {
		ids = append(ids, fmt.Sprintf(`NVME\VEN_%04X&DEV_%04X`, vid, did))

		if subsysVID != 0 && subsysDID != 0 {
			ids = append(ids, fmt.Sprintf(`NVME\VEN_%04X&DEV_%04X&SUBSYS_%04X%04X`, vid, did, subsysVID, subsysDID))
		}
	}

	if model = strings.TrimSpace(model); model != "" {
		ids = append(ids, model)
	}

	return ids
}

// GUIDs maps instance IDs to their fwupd-compatible GUIDs.
func GUIDs(instanceIDs []string) []string {
	guids := make([]string, 0, len(instanceIDs))

	for _, id := range instanceIDs {
		guids = append(guids, HashString(id))
	}

	return guids
}
