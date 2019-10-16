/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package util

import (
	"fmt"
	"strings"
)

// PartNo returns the partition number.
func PartNo(partname string) (partno string, err error) {
	partname = strings.TrimPrefix(partname, "/dev/")

	switch p := partname; {
	case strings.HasPrefix(p, "nvme"):
		fallthrough
	case strings.HasPrefix(p, "loop"):
		idx := strings.LastIndex(partname, "p")
		return partname[idx+1:], nil
	case strings.HasPrefix(p, "sd"):
		fallthrough
	case strings.HasPrefix(p, "hd"):
		fallthrough
	case strings.HasPrefix(p, "vd"):
		fallthrough
	case strings.HasPrefix(p, "xvd"):
		return strings.TrimLeft(partname, "/abcdefghijklmnopqrstuvwxyz"), nil
	default:
		return "", fmt.Errorf("could not determine partition number from partition name: %s", partname)
	}
}

// DevnameFromPartname returns the device name from a partition name.
func DevnameFromPartname(partname string) (devname string, err error) {
	partname = strings.TrimPrefix(partname, "/dev/")

	var partno string

	if partno, err = PartNo(partname); err != nil {
		return "", err
	}

	switch p := partname; {
	case strings.HasPrefix(p, "nvme"):
		fallthrough
	case strings.HasPrefix(p, "loop"):
		return strings.TrimRight(p, "p"+partno), nil
	case strings.HasPrefix(p, "sd"):
		fallthrough
	case strings.HasPrefix(p, "hd"):
		fallthrough
	case strings.HasPrefix(p, "vd"):
		fallthrough
	case strings.HasPrefix(p, "xvd"):
		return strings.TrimRight(partname, partno), nil
	default:
		return "", fmt.Errorf("could not determine dev name from partition name: %s", partname)
	}
}
