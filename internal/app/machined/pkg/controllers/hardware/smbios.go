// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"sync"

	"github.com/talos-systems/go-smbios/smbios"
)

// GetSMBIOSInfo returns the SMBIOS info.
func GetSMBIOSInfo() (*smbios.SMBIOS, error) {
	var (
		sync sync.Once
		conn *smbios.SMBIOS
		err  error
	)

	sync.Do(func() {
		conn, err = smbios.New()
	})

	return conn, err
}
