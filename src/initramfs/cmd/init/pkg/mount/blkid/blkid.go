// +build linux

// Package blkid provides bindings to libblkid.
package blkid

/*
#cgo CFLAGS: -I/usr/include
#cgo LDFLAGS: -L/usr/lib -lblkid
#include <stdio.h>
#include <stdlib.h>
#include <blkid/blkid.h>
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/pkg/errors"
)

const (
	BLKID_SUBLKS_LABEL        = (1 << 1)
	BLKID_SUBLKS_UUID         = (1 << 3)
	BLKID_SUBLKS_TYPE         = (1 << 5)
	BLKID_PARTS_ENTRY_DETAILS = (1 << 2)
)

// GetDevWithAttribute returns the dev name of a block device matching the ATTRIBUTE=VALUE
// pair. Supported attributes are:
//    TYPE: filesystem type
//    UUID: filesystem uuid
//    LABEL: filesystem label
func GetDevWithAttribute(attribute, value string) (string, error) {
	var cache C.blkid_cache

	ret := C.blkid_get_cache(&cache, nil)
	if ret != 0 {
		return "", fmt.Errorf("failed to get blkid cache: %d", ret)
	}

	C.blkid_probe_all(cache)

	cs_attribute := C.CString(attribute)
	cs_value := C.CString(value)
	defer C.free(unsafe.Pointer(cs_attribute))
	defer C.free(unsafe.Pointer(cs_value))

	devname := C.blkid_get_devname(cache, cs_attribute, cs_value)
	defer C.free(unsafe.Pointer(devname))

	// If you have called blkid_get_cache(), you should call blkid_put_cache()
	// when you are done using the blkid library functions.  This will save the
	// cache to the blkid.tab file, if you have write access to the file.  It
	// will also free all associated devices and tags:
	C.blkid_put_cache(cache)

	return C.GoString(devname), nil
}

// NewProbeFromFilename executes lblkid blkid_new_probe_from_filename.
func NewProbeFromFilename(s string) (C.blkid_probe, error) {
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	var pr C.blkid_probe = C.blkid_new_probe_from_filename(cs)
	if pr == nil {
		return nil, fmt.Errorf("failed to open device %s", C.GoString(cs))
	}

	return pr, nil
}

// DoProbe executes lblkid blkid_do_probe.
func DoProbe(pr C.blkid_probe) error {
	if retval := C.blkid_do_probe(pr); retval != 0 {
		return errors.Errorf("%d", retval)
	}

	return nil
}

// DoSafeProbe executes lblkid blkid_do_safeprobe.
func DoSafeProbe(pr C.blkid_probe) error {
	if retval := C.blkid_do_safeprobe(pr); retval != 0 {
		return errors.Errorf("%d", retval)
	}

	return nil
}

// ProbeLookupValue implements:
//  int blkid_probe_lookup_value (blkid_probe pr, const char *name, const char **data, size_t *len);
func ProbeLookupValue(pr C.blkid_probe, name string, size *int) (string, error) {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	var data *C.char
	defer C.free(unsafe.Pointer(data))

	C.blkid_probe_enable_superblocks(pr, 1)
	C.blkid_probe_set_superblocks_flags(pr, BLKID_SUBLKS_LABEL|BLKID_SUBLKS_UUID|BLKID_SUBLKS_TYPE)
	C.blkid_probe_enable_partitions(pr, 1)
	C.blkid_probe_set_partitions_flags(pr, BLKID_PARTS_ENTRY_DETAILS)

	if err := DoSafeProbe(pr); err != nil {
		return "", errors.Errorf("failed to do safe probe: %v", err)
	}

	retval := C.blkid_probe_lookup_value(pr, cs, &data, nil)
	if retval != 0 {
		return "", errors.Errorf("failed to lookup value %s: %d", name, retval)
	}

	return C.GoString(data), nil
}

// ProbeGetPartitions implements:
//  blkid_partlist blkid_probe_get_partitions (blkid_probe pr);
func ProbeGetPartitions(pr C.blkid_probe) C.blkid_partlist {
	return C.blkid_probe_get_partitions(pr)
}

// ProbeGetPartitionsPartlistNumOfPartitions implements:
//  int blkid_partlist_numof_partitions (blkid_partlist ls);
func ProbeGetPartitionsPartlistNumOfPartitions(ls C.blkid_partlist) int {
	return int(C.blkid_partlist_numof_partitions(ls))
}

// FreeProbe implements:
//  int blkid_partlist_numof_partitions (blkid_partlist ls);
func FreeProbe(pr C.blkid_probe) {
	C.blkid_free_probe(pr)
}
