// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package smbios

import (
	"strings"
)

// ReadOEMVariable reads the OEM variable from SMBIOS and returns its value.
func ReadOEMVariable(name string) ([]string, error) {
	smbiosInfo, err := GetSMBIOSInfo()
	if err != nil {
		return nil, err
	}

	var result []string

	for _, str := range smbiosInfo.OEMStrings.Strings {
		key, val, ok := strings.Cut(str, "=")
		if !ok {
			continue
		}

		if key == name {
			result = append(result, val)
		}
	}

	return result, nil
}
