package util

import (
	"strings"

	"github.com/pkg/errors"
)

// PartNo returns the partition number.
func PartNo(partname string) (partno string, err error) {
	partname = strings.TrimPrefix(partname, "/dev/")
	if strings.HasPrefix(partname, "nvme") {
		idx := strings.Index(partname, "p")
		return partname[idx+1:], nil
	} else if strings.HasPrefix(partname, "sd") || strings.HasPrefix(partname, "hd") || strings.HasPrefix(partname, "vd") {
		return strings.TrimLeft(partname, "/abcdefghijklmnopqrstuvwxyz"), nil
	}

	return "", errors.New("could not determine partition number from partition name")
}

// DevnameFromPartname returns the device name from a partition name.
func DevnameFromPartname(partname string) (devname string, err error) {
	partname = strings.TrimPrefix(partname, "/dev/")
	var partno string
	if partno, err = PartNo(partname); err != nil {
		return "", err
	}
	if strings.HasPrefix(partname, "nvme") {
		return strings.TrimRight(partname, "p"+partno), nil
	} else if strings.HasPrefix(partname, "sd") || strings.HasPrefix(partname, "hd") || strings.HasPrefix(partname, "vd") {
		return strings.TrimRight(partname, partno), nil
	}

	return "", errors.New("could not determine dev name from partition name")
}
