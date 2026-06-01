// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package ethtool

// Origin: https://github.com/weaveworks/weave/blob/master/net/ethtool.go

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Linux constants.
//
//nolint:revive
const (
	SIOCETHTOOL     = 0x8946     // linux/sockios.h
	ETHTOOL_GTXCSUM = 0x00000016 // linux/ethtool.h
	ETHTOOL_STXCSUM = 0x00000017 // linux/ethtool.h
	IFNAMSIZ        = 16         // linux/if.h
)

// linux/if.h 'struct ifreq'.
type ifReqData struct {
	Name [IFNAMSIZ]byte
	Data uintptr
}

// linux/ethtool.h 'struct ethtool_value'.
type ethtoolValue struct {
	Cmd  uint32
	Data uint32
}

func ioctlEthtool(fd int, argp uintptr) error {
	_, _, errno := syscall.RawSyscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(SIOCETHTOOL), argp)
	if errno != 0 {
		return errno
	}

	return nil
}

// TXOff disables TX checksum offload on specified interface.
func TXOff(name string) error {
	if len(name)+1 > IFNAMSIZ {
		return fmt.Errorf("name too long")
	}

	socket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer syscall.Close(socket) //nolint:errcheck

	// Request current value
	value := ethtoolValue{Cmd: ETHTOOL_GTXCSUM}
	request := ifReqData{Data: uintptr(unsafe.Pointer(&value))}
	copy(request.Name[:], name)

	if err := ioctlEthtool(socket, uintptr(unsafe.Pointer(&request))); err != nil {
		return err
	}

	if value.Data == 0 { // if already off, don't try to change
		return nil
	}

	value = ethtoolValue{ETHTOOL_STXCSUM, 0}

	return ioctlEthtool(socket, uintptr(unsafe.Pointer(&request)))
}
