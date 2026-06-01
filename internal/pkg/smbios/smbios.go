// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package smbios provides access to SMBIOS information.
package smbios

import (
	"sync"

	"github.com/siderolabs/go-smbios/smbios"
)

var (
	syncSMBIOS sync.Once
	connSMBIOS *smbios.SMBIOS
	errSMBIOS  error
)

// GetSMBIOSInfo returns the SMBIOS info.
func GetSMBIOSInfo() (*smbios.SMBIOS, error) {
	syncSMBIOS.Do(func() {
		connSMBIOS, errSMBIOS = smbios.New()
	})

	return connSMBIOS, errSMBIOS
}
