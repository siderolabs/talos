// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// PortRange is a port range.
type PortRange struct {
	Lo uint16
	Hi uint16
}

// UnmarshalYAML is a custom unmarshaller for `PortRange`.
func (pr *PortRange) UnmarshalYAML(unmarshal func(any) error) error {
	var port uint16

	if err := unmarshal(&port); err == nil {
		pr.Lo = port
		pr.Hi = port

		return nil
	}

	var rangeStr string

	if err := unmarshal(&rangeStr); err != nil {
		return err
	}

	lo, hi, ok := strings.Cut(rangeStr, "-")
	if !ok {
		return fmt.Errorf("invalid port range: %q", rangeStr)
	}

	prLo, err := strconv.ParseUint(lo, 10, 16)
	if err != nil {
		return fmt.Errorf("invalid port range: %q", rangeStr)
	}

	prHi, err := strconv.ParseUint(hi, 10, 16)
	if err != nil {
		return fmt.Errorf("invalid port range: %q", rangeStr)
	}

	pr.Lo, pr.Hi = uint16(prLo), uint16(prHi)

	return nil
}

// MarshalYAML is a custom marshaller for `PortRange`.
func (pr PortRange) MarshalYAML() (any, error) {
	if pr.Lo == pr.Hi {
		return pr.Lo, nil
	}

	return fmt.Sprintf("%d-%d", pr.Lo, pr.Hi), nil
}

// String implements fmt.Stringer interface.
func (pr PortRange) String() string {
	return fmt.Sprintf("%d-%d", pr.Lo, pr.Hi)
}

// PortRanges is a slice of port ranges.
type PortRanges []PortRange

// Validate the port ranges.
func (prs PortRanges) Validate() error {
	clone := slices.Clone(prs)
	slices.SortFunc(clone, func(a, b PortRange) int {
		return cmp.Compare(a.Lo, b.Lo)
	})

	for i, pr := range clone {
		if pr.Lo > pr.Hi {
			return fmt.Errorf("invalid port range: %s", pr)
		}

		if i > 0 {
			prev := clone[i-1]

			if pr.Lo == prev.Lo {
				return fmt.Errorf("invalid port range: %s, overlaps with %s", pr, prev)
			}

			if pr.Lo <= prev.Hi {
				return fmt.Errorf("invalid port range: %s, overlaps with %s", pr, prev)
			}
		}
	}

	return nil
}

func examplePortRanges1() PortRanges {
	return PortRanges{
		{Lo: 80, Hi: 80},
		{Lo: 443, Hi: 443},
	}
}

func examplePortRanges2() PortRanges {
	return PortRanges{
		{Lo: 1200, Hi: 1299},
		{Lo: 8080, Hi: 8080},
	}
}
