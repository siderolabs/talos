// Package blkid provides bindings to libblkid.
package blkid

/*
#cgo CFLAGS: -I/tools/usr/local/musl/include -I/usr/include
#cgo LDFLAGS: -L/usr/lib -lblkid
#include <stdio.h>
#include <stdlib.h>
#include <blkid/blkid.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

func NewProbeFromFilename(s string) (C.blkid_probe, error) {
	cs := C.CString(s)
	defer C.free(unsafe.Pointer(cs))
	var pr C.blkid_probe = C.blkid_new_probe_from_filename(cs)
	if pr == nil {
		return nil, fmt.Errorf("failed to open %s", C.GoString(cs))
	}

	return pr, nil
}

func DoProbe(pr C.blkid_probe) {
	C.blkid_do_probe(pr)
}

// ProbeLookupValue implements:
//	int blkid_probe_lookup_value (blkid_probe pr, const char *name, const char **data, size_t *len);
func ProbeLookupValue(pr C.blkid_probe, name string, size *int) (string, error) {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))

	var data *C.char
	defer C.free(unsafe.Pointer(data))

	if size == nil {
		C.blkid_probe_lookup_value(pr, cs, &data, nil)
	} else {
		var s *C.size_t
		defer C.free(unsafe.Pointer(s))
		*s = C.size_t(*size)
		C.blkid_probe_lookup_value(pr, cs, &data, s)
	}

	return C.GoString(data), nil
}

// ProbeGetPartitions implements:
//	blkid_partlist blkid_probe_get_partitions (blkid_probe pr);
func ProbeGetPartitions(pr C.blkid_probe) C.blkid_partlist {
	return C.blkid_probe_get_partitions(pr)
}

// ProbeGetPartitionsPartlistNumOfPartitions implements:
//	int blkid_partlist_numof_partitions (blkid_partlist ls);
func ProbeGetPartitionsPartlistNumOfPartitions(ls C.blkid_partlist) int {
	return int(C.blkid_partlist_numof_partitions(ls))
}

// FreeProbe implements:
//	int blkid_partlist_numof_partitions (blkid_partlist ls);
func FreeProbe(pr C.blkid_probe) {
	C.blkid_free_probe(pr)
}
