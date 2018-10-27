package util

import "C"
import (
	"strings"
)

func PartNo(partname string) string {
	if strings.HasPrefix(partname, "/dev/nvme") {
		idx := strings.Index(partname, "p")
		return partname[idx+1:]
	} else if strings.HasPrefix(partname, "/dev/sd") || strings.HasPrefix(partname, "/dev/hd") {
		return strings.TrimLeft(partname, "/abcdefghijklmnopqrstuvwxyz")
	}

	return ""
}

func DevnameFromPartname(partname, partno string) string {
	if strings.HasPrefix(partname, "/dev/nvme") {
		return strings.TrimRight(partname, "p"+partno)
	} else if strings.HasPrefix(partname, "/dev/sd") || strings.HasPrefix(partname, "/dev/hd") {
		return strings.TrimRight(partname, partno)
	}

	return ""
}
