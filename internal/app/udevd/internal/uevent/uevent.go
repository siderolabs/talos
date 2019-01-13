/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package uevent is a library for working the the kernel userspace events.
package uevent

import (
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"golang.org/x/sys/unix"
)

const (
	// KernelGroup is the broadcast group for the kernel.
	KernelGroup = 0x1
)

// KObject represents a Linux Kobject.
type KObject struct {
	fd int
}

// UEvent represents a userspace event.
type UEvent struct {
	*Message

	Error error
}

// Message represents a uevent message.
type Message struct {
	Action    Action
	Devpath   string
	Subsystem Subsystem
	Seqnum    int
	Values    map[string]string
}

// Dial returns a netlink socket using the address family AF_NETLINK and netlink
// family NETLINK_KOBJECT_UEVENT.
// 		Domain: AF_NETLINK
// 		Type: SOCK_RAW (the netlink protocol does not distinguish between datagram and raw sockets)
// 		Protocol: NETLINK_KOBJECT_UEVENT
// See http://man7.org/linux/man-pages/man7/netlink.7.html.
func Dial() (kobject *KObject, err error) {
	var fd int
	if fd, err = unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW, unix.NETLINK_KOBJECT_UEVENT); err != nil {
		return nil, err
	}

	err = unix.Bind(fd, &unix.SockaddrNetlink{
		Family: unix.AF_NETLINK,
		Groups: KernelGroup,
		Pid:    uint32(os.Getpid()),
	})

	return &KObject{fd: fd}, err
}

// Close closes the socket.
func (obj *KObject) Close() error {
	return unix.Close(obj.fd)
}

// Watch watches for kernel uevents.
func (obj *KObject) Watch() (uevents chan *UEvent) {
	uevents = make(chan *UEvent)
	go func() {
		buf := make([]byte, os.Getpagesize())
		for {
			var n int
			var err error
			e := &UEvent{
				Message: &Message{
					Values: map[string]string{},
				},
			}
			for {
				if n, _, err = unix.Recvfrom(obj.fd, buf, unix.MSG_PEEK); err != nil {
					e.Error = err
					uevents <- e
				}
				if n < len(buf) {
					break
				}
				buf = make([]byte, len(buf)*2)
			}

			if n, _, err = unix.Recvfrom(obj.fd, buf, 0); err != nil {
				e.Error = err
				uevents <- e
			}

			if err = e.parse(buf[:n]); err != nil {
				e.Error = err
				uevents <- e
			}

			uevents <- e

			// Clear the buffer.
			for i := 0; i < n; i++ {
				buf[i] = 0
			}
		}
	}()

	return uevents
}

// Action represents a uevent action.
type Action int

const (
	// ActionAdd represents a uevent add action.
	ActionAdd = iota
	// ActionRemove represents a uevent remove action.
	ActionRemove
	// ActionChange represents a uevent change action.
	ActionChange
	// ActionMove represents a uevent move action.
	ActionMove
	// ActionOnline represents a uevent online action.
	ActionOnline
	// ActionOffline represents a uevent offline action.
	ActionOffline
	// ActionUnknown represents a uevent unknown action.
	ActionUnknown
)

// NewAction returns an Action from a string.
func NewAction(s string) Action {
	switch s {
	case "add":
		return ActionAdd
	case "remove":
		return ActionRemove
	case "change":
		return ActionChange
	case "move":
		return ActionMove
	case "online":
		return ActionOnline
	case "offline":
		return ActionOffline
	default:
		return ActionUnknown
	}
}

// String returns the string representation of an Action.
func (actn Action) String() string {
	switch actn {
	case ActionAdd:
		return "add"
	case ActionRemove:
		return "remove"
	case ActionChange:
		return "change"
	case ActionMove:
		return "move"
	case ActionOnline:
		return "online"
	case ActionOffline:
		return "offline"
	default:
		return "unknown"
	}
}

// Subsystem represents a uevent subsystem.
type Subsystem int

const (
	// SubsystemBlock represents the block subsystem.
	SubsystemBlock = iota
	// SubsystemUSB represents the usdb subsystem.
	SubsystemUSB
	// SubsystemUnknown represents an unknown subsystem.
	SubsystemUnknown
)

// NewSubsystem returns a Subsystem from a string.
func NewSubsystem(s string) Subsystem {
	switch s {
	case "block":
		return SubsystemBlock
	case "usd":
		return SubsystemUSB
	default:
		return SubsystemUnknown
	}
}

// String returns the string representation of Subsystem.
func (sys Subsystem) String() string {
	switch sys {
	case SubsystemBlock:
		return "block"
	case SubsystemUSB:
		return "usb"
	default:
		return "unknown"
	}
}

func (evt *UEvent) parse(buf []byte) (err error) {
	fields := strings.Split(string(buf), "\x00")
	for _, field := range fields {
		parts := strings.Split(field, "=")
		if len(parts) != 2 {
			continue
		}

		switch parts[0] {
		case "ACTION":
			evt.Action = NewAction(parts[1])
		case "DEVPATH":
			evt.Devpath = parts[1]
		case "SUBSYSTEM":
			evt.Subsystem = NewSubsystem(parts[1])
		case "SEQNUM":
			n, err := strconv.Atoi(parts[1])
			if err != nil {
				return errors.Errorf("error converting SEQNUM: %v", err)
			}
			evt.Seqnum = n
		default:
			evt.Values[parts[0]] = parts[1]
		}
	}

	return nil
}
