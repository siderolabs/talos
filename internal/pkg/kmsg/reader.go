// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kmsg

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

// Packet combines Message and error.
//
// Only one of the fields is set in Reader.Scan.
type Packet struct {
	Message Message
	Err     error
}

// Reader for /dev/kmsg messages.
type Reader interface {
	// Scan and issue parsed messages.
	//
	// Scan stops when context is canceled or when EOF is reached
	// in NoFollow mode.
	Scan(ctx context.Context) <-chan Packet

	// Close releases resources associated with the Reader.
	Close() error
}

// Option configures Reader.
type Option func(*options)

type options struct {
	follow bool
	tail   bool
}

// Follow the kmsg to stream live messages.
func Follow() Option {
	return func(o *options) {
		o.follow = true
	}
}

// FromTail starts reading kmsg from the tail (after last message).
func FromTail() Option {
	return func(o *options) {
		o.tail = true
	}
}

// NewReader initializes new /dev/kmsg reader.
func NewReader(options ...Option) (Reader, error) {
	r := &reader{}

	for _, o := range options {
		o(&r.options)
	}

	var err error

	r.bootTime, err = getBootTime()
	if err != nil {
		return nil, err
	}

	r.f, err = os.OpenFile("/dev/kmsg", os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	if r.options.tail {
		_, err = r.f.Seek(0, os.SEEK_END)
		if err != nil {
			r.f.Close() //nolint:errcheck

			return nil, fmt.Errorf("error seeking to the tail of kmsg: %w", err)
		}
	}

	return r, nil
}

type reader struct {
	f        *os.File
	options  options
	bootTime time.Time
}

func (r *reader) Close() error {
	return r.f.Close()
}

func (r *reader) Scan(ctx context.Context) <-chan Packet {
	ch := make(chan Packet)

	if r.options.follow {
		go r.scanFollow(ctx, ch)
	} else {
		go r.scanNoFollow(ctx, ch)
	}

	return ch
}

//nolint:gocyclo
func (r *reader) scanNoFollow(ctx context.Context, ch chan<- Packet) {
	defer close(ch)

	fd := int(r.f.Fd())

	if err := syscall.SetNonblock(fd, true); err != nil {
		select {
		case ch <- Packet{
			Err: fmt.Errorf("error switching to nonblock mode: %w", err),
		}:
		case <-ctx.Done():
		}

		return
	}

	buf := make([]byte, 8192)

	for {
		n, err := syscall.Read(fd, buf)
		if err != nil {
			if err == io.EOF || err == syscall.EAGAIN {
				// end of file, done
				return
			}

			if err == syscall.EPIPE {
				// buffer overrun, retry
				continue
			}

			select {
			case ch <- Packet{
				Err: fmt.Errorf("error reading from kmsg: %w", err),
			}:
			case <-ctx.Done():
			}

			return
		}

		var packet Packet
		packet.Message, packet.Err = ParseMessage(string(buf[:n]), r.bootTime)

		select {
		case ch <- packet:
		case <-ctx.Done():
		}
	}
}

func (r *reader) scanFollow(ctx context.Context, ch chan<- Packet) {
	defer close(ch)

	buf := make([]byte, 8192)

	for {
		n, err := r.f.Read(buf)
		if err != nil {
			if err == io.EOF {
				// end of file, done
				return
			}

			if err == syscall.EPIPE {
				// buffer overrun, retry
				continue
			}

			select {
			case ch <- Packet{
				Err: fmt.Errorf("error reading from kmsg: %w", err),
			}:
			case <-ctx.Done():
			}

			return
		}

		var packet Packet
		packet.Message, packet.Err = ParseMessage(string(buf[:n]), r.bootTime)

		select {
		case ch <- packet:
		case <-ctx.Done():
		}
	}
}
