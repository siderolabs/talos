// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package uki

import (
	"debug/pe"
	"errors"
)

// GetSBAT returns the SBAT section from the PE file.
func GetSBAT(path string) ([]byte, error) {
	pefile, err := pe.Open(path)
	if err != nil {
		return nil, err
	}

	defer pefile.Close() //nolint:errcheck

	for _, section := range pefile.Sections {
		if section.Name == string(SectionSBAT) {
			data, err := section.Data()
			if err != nil {
				return nil, err
			}

			return data[:section.VirtualSize], nil
		}
	}

	return nil, errors.New("could not find SBAT section")
}
