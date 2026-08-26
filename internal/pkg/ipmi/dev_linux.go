// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package ipmi

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// DevicePathGlob matches the OpenIPMI character devices.
const DevicePathGlob = "/dev/ipmi*"

const (
	// responseTimeout is how long to wait for the BMC to answer a single command.
	//
	// The KCS interface is slow (tens of milliseconds per command), and firmware
	// which doesn't implement a command may take even longer to NAK it.
	responseTimeout = 5 * time.Second

	// pollSlice caps a single poll(2) call: poll(2) can't be interrupted by context
	// cancellation, so the wait for a reply is sliced to keep the caller responsive
	// to shutdown while a wedged BMC runs down the response timeout.
	pollSlice = 250 * time.Millisecond

	// recvTypeResponse is IPMI_RESPONSE_RECV_TYPE: a response to a command sent from
	// this handle, as opposed to an asynchronous event or a received command.
	recvTypeResponse = 1

	// responseBufSize is the receive buffer size: an IPMI response over the system
	// interface is bounded well below this.
	responseBufSize = 256
)

type sysIfaceAddr struct {
	addrType int32
	channel  int16
	lun      uint8
	_        uint8
}

type ipmiMsg struct {
	netfn   uint8
	cmd     uint8
	dataLen uint16
	_       [4]byte
	data    unsafe.Pointer
}

type ipmiReq struct {
	addr    unsafe.Pointer
	addrLen uint32
	_       [4]byte
	msgid   int64
	msg     ipmiMsg
}

type ipmiRecv struct {
	recvType int32
	_        [4]byte
	addr     unsafe.Pointer
	addrLen  uint32
	_        [4]byte
	msgid    int64
	msg      ipmiMsg
}

// OpenIPMI ioctl request numbers (linux/ipmi.h), computed from the struct sizes
// so they are correct on any LP64 arch (amd64, arm64), not a hardcoded constant:
//
//	IPMICTL_SEND_COMMAND      = _IOR('i', 13, struct ipmi_req)
//	IPMICTL_RECEIVE_MSG_TRUNC = _IOWR('i', 11, struct ipmi_recv)
var (
	ioctlSendCommand     = ioc(2, 'i', 13, unsafe.Sizeof(ipmiReq{}))  // _IOR
	ioctlReceiveMsgTrunc = ioc(3, 'i', 11, unsafe.Sizeof(ipmiRecv{})) // _IOWR
)

func ioc(dir, typ, nr, size uintptr) uintptr {
	const (
		nrShift   = 0
		typeShift = 8
		sizeShift = 16
		dirShift  = 30
	)

	return dir<<dirShift | size<<sizeShift | typ<<typeShift | nr<<nrShift
}

// Dev is an open handle to a local BMC.
type Dev struct {
	fd  int
	seq atomic.Int64
}

// Open the OpenIPMI character device at path.
func Open(path string) (*Dev, error) {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	return &Dev{fd: fd}, nil
}

// Close the device.
func (d *Dev) Close() error {
	return unix.Close(d.fd)
}

func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(arg)); errno != 0 {
		return errno
	}

	return nil
}

// SendRecv issues one request to the local BMC and returns the completion code
// and the response payload (the bytes after the completion code).
//
// It implements [Transport]. It is not safe for concurrent use on a single [Dev].
//
// #nosec G103 -- the OpenIPMI ioctl ABI takes raw pointers to request/response
// structs; unsafe.Pointer is inherent to the interface, and KeepAlive pins the
// referents across the syscalls.
func (d *Dev) SendRecv(ctx context.Context, netfn, cmd byte, data []byte) (byte, []byte, error) {
	addr := sysIfaceAddr{addrType: systemInterfaceAddrType, channel: bmcChannel}

	var reqData unsafe.Pointer

	if len(data) > 0 {
		reqData = unsafe.Pointer(&data[0])
	}

	msgid := d.seq.Add(1)

	req := ipmiReq{
		addr:    unsafe.Pointer(&addr),
		addrLen: uint32(unsafe.Sizeof(addr)),
		msgid:   msgid,
		msg:     ipmiMsg{netfn: netfn, cmd: cmd, dataLen: uint16(len(data)), data: reqData},
	}

	err := ioctl(d.fd, ioctlSendCommand, unsafe.Pointer(&req))

	runtime.KeepAlive(data)
	runtime.KeepAlive(&addr)

	if err != nil {
		return 0, nil, fmt.Errorf("IPMICTL_SEND_COMMAND: %w", err)
	}

	// The BMC reply lands on the device asynchronously, and the reply to a command
	// which already timed out may still be queued ahead of it, so messages are
	// drained until the one matching this request shows up: the driver queues them
	// per open handle, and this one is reused across commands.
	deadline := time.Now().Add(responseTimeout)

	for {
		if err := d.wait(ctx, deadline); err != nil {
			return 0, nil, err
		}

		recv, buf, err := d.receive()
		if err != nil {
			return 0, nil, err
		}

		switch {
		case recv.msgid == msgid && recv.recvType == recvTypeResponse:
		case recv.msgid < msgid:
			// the reply to a command which already timed out, or an asynchronous event:
			// drop it and keep waiting for the one matching this request
			continue
		default:
			// msgid is assigned here and only ever increases, so a reply from the future
			// means the driver or the firmware echoed back an id which was never sent
			return 0, nil, fmt.Errorf("unexpected IPMI message: recv type %d, msgid %d, want %d", recv.recvType, recv.msgid, msgid)
		}

		n := int(recv.msg.dataLen)
		if n < 1 {
			return 0, nil, errors.New("empty IPMI response")
		}

		return buf[0], append([]byte(nil), buf[1:n]...), nil
	}
}

// wait blocks until the device has a message to dequeue, ctx is canceled or the
// deadline expires.
func (d *Dev) wait(ctx context.Context, deadline time.Time) error {
	pfd := []unix.PollFd{{Fd: int32(d.fd), Events: unix.POLLIN}}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return errors.New("IPMI response timeout")
		}

		n, err := unix.Poll(pfd, int(min(remaining, pollSlice).Milliseconds()))
		if err != nil && !errors.Is(err, unix.EINTR) {
			return fmt.Errorf("poll: %w", err)
		}

		if n > 0 {
			return nil
		}
	}
}

// receive dequeues a single message from the device.
//
// #nosec G103 -- see [Dev.SendRecv].
func (d *Dev) receive() (ipmiRecv, []byte, error) {
	var rAddr sysIfaceAddr

	buf := make([]byte, responseBufSize)

	recv := ipmiRecv{
		addr:    unsafe.Pointer(&rAddr),
		addrLen: uint32(unsafe.Sizeof(rAddr)),
		msg:     ipmiMsg{dataLen: uint16(len(buf)), data: unsafe.Pointer(&buf[0])},
	}

	err := ioctl(d.fd, ioctlReceiveMsgTrunc, unsafe.Pointer(&recv))

	runtime.KeepAlive(&rAddr)
	runtime.KeepAlive(buf)

	if err != nil {
		return recv, nil, fmt.Errorf("IPMICTL_RECEIVE_MSG_TRUNC: %w", err)
	}

	return recv, buf, nil
}
