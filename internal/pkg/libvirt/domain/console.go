// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain

import (
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	libvirt "github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"libvirt.org/go/libvirtxml"
)

const (
	consoleProgram          = 0x20008086
	consoleConnectProcedure = 1
	consoleProcedure        = 201
	consoleSerial           = 0x7fffffff
	consoleMaxFrame         = 1 << 20
	consoleChunk            = 64 << 10
)

// ConsolePreconditionError reports live domain state which prevents a safe
// console attachment. Transport and peer-decoding failures use their original
// error types so callers can classify them separately.
type ConsolePreconditionError struct {
	cause error
}

// Error implements error.
func (err *ConsolePreconditionError) Error() string {
	return err.cause.Error()
}

// Unwrap exposes the descriptive cause.
func (err *ConsolePreconditionError) Unwrap() error {
	return err.cause
}

func consolePreconditionError(message string) error {
	return &ConsolePreconditionError{cause: errors.New(message)}
}

// OpenConsole attaches exclusively to the first serial device of an active,
// owned domain. It uses a dedicated connection; Close or context cancellation
// releases the attachment. Slow readers are disconnected when the bounded
// output queue fills, rather than retaining unbounded guest output.
func (c *Connector) OpenConsole(ctx context.Context, d Domain) (io.ReadWriteCloser, error) {
	if err := validateDomain(d); err != nil {
		return nil, err
	}

	startup, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()

	raw, err := (&net.Dialer{}).DialContext(startup, "unix", c.socket)
	if err != nil {
		return nil, err
	}

	return c.openConsoleConn(ctx, startup, raw, d)
}

func (c *Connector) openConsoleConn(ctx, startup context.Context, raw net.Conn, d Domain) (io.ReadWriteCloser, error) {
	transport := &consoleTransport{
		Conn:          raw,
		frames:        make(chan []byte, 16),
		done:          make(chan struct{}),
		setupComplete: make(chan struct{}),
	}

	stopStartup := context.AfterFunc(startup, func() { transport.finish(startup.Err()) })
	defer stopStartup()

	stop := context.AfterFunc(ctx, func() { transport.finish(ctx.Err()) })

	rpc := libvirt.NewWithDialer(dialers.NewAlreadyConnected(transport))
	if err := rpc.ConnectToURI(libvirt.ConnectURI(c.uri)); err != nil {
		close(transport.setupComplete)
		stop()
		transport.finish(err)

		return nil, err
	}

	disconnected := rpc.Disconnected()

	close(transport.setupComplete)

	stream := &consoleStream{transport: transport, disconnected: disconnected, stop: stop}

	success := false
	defer func() {
		if !success {
			stream.close()
		}
	}()

	found, err := consoleDomain(rpc, d)
	if err != nil {
		return nil, err
	}

	payload := consoleString(nil, found.Name)
	payload = append(payload, found.UUID[:]...)
	payload = binary.BigEndian.AppendUint32(payload, uint32(found.ID))
	payload = binary.BigEndian.AppendUint32(payload, 1) // optional device name present
	payload = consoleString(payload, "serial0")

	payload = binary.BigEndian.AppendUint32(payload, uint32(libvirt.DomainConsoleSafe)) // never FORCE
	if err = transport.send(0, 0, payload); err != nil {
		return nil, err
	}

	frame, err := transport.next()
	if err != nil {
		return nil, err
	}

	if binary.BigEndian.Uint32(frame[12:16]) != 1 || binary.BigEndian.Uint32(frame[20:24]) != 0 {
		return nil, consoleFrameError(frame)
	}

	if err = startup.Err(); err != nil {
		return nil, err
	}

	success = true

	return stream, nil
}

func consoleDomain(rpc *libvirt.Libvirt, d Domain) (libvirt.Domain, error) {
	found, err := rpc.DomainLookupByName(d.Name)
	if err != nil {
		return found, err
	}

	if found.Name != d.Name || found.UUID != libvirt.UUID(d.UUID) {
		return found, consolePreconditionError(fmt.Sprintf("domain %q is not owned by this machine", d.Name))
	}

	text, err := rpc.DomainGetXMLDesc(found, 0)
	if err != nil {
		return found, err
	}

	if err = validateConsoleXML(text, d); err != nil {
		return found, err
	}

	active, err := rpc.DomainIsActive(found)
	if err != nil {
		return found, err
	}

	if active != 1 {
		return found, consolePreconditionError("domain is not active")
	}

	return found, nil
}

func validateConsoleXML(text string, d Domain) error {
	var desc libvirtxml.Domain
	if err := desc.Unmarshal(text); err != nil {
		return err
	}

	if desc.Name != d.Name || desc.UUID != d.UUID.String() {
		return consolePreconditionError("console domain XML identity mismatch")
	}

	var metadata struct {
		Digest string `xml:"https://talos.dev/libvirt/domain definition"`
	}

	if desc.Metadata == nil {
		return consolePreconditionError("domain has no Talos ownership metadata")
	}

	if err := xml.Unmarshal([]byte("<metadata>"+desc.Metadata.XML+"</metadata>"), &metadata); err != nil {
		return err
	}

	if metadata.Digest == "" {
		return consolePreconditionError("domain has no Talos ownership metadata")
	}

	if desc.Devices == nil || len(desc.Devices.Serials) == 0 {
		return consolePreconditionError("domain has no serial device")
	}

	return nil
}

// consoleTransport leaves ordinary RPC framing to the SDK and diverts only our
// reserved console serial. go-libvirt requestStream uses unbuffered abort/outErr
// sends which can deadlock on guest EOF or cancellation. Do not replace this
// bounded demultiplexer with DomainOpenConsoleBidirectional until that is fixed.
// The SDK owns the sole socket reader; no additional stream pump is started.
type consoleTransport struct {
	net.Conn
	pending          []byte // accessed only by the SDK reader
	frames           chan []byte
	done             chan struct{}
	setupComplete    chan struct{}
	connectReplySeen bool // accessed only by the SDK reader
	connectDelivered bool // accessed only by the SDK reader
	connectSerial    uint32
	once             sync.Once
	mu               sync.Mutex
	err              error
	writeMu          sync.Mutex
	writeHeader      []byte
	writeRemaining   uint32
}

func (c *consoleTransport) finish(err error) {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		closeTransport(c.Conn)
		close(c.done)
	})
}

func (c *consoleTransport) Close() error {
	c.finish(io.EOF)

	return nil
}

func (c *consoleTransport) Write(p []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	n, err := c.Conn.Write(p)
	c.observeWrite(p[:n])

	return n, err
}

func (c *consoleTransport) observeWrite(p []byte) {
	for len(p) > 0 {
		if c.writeRemaining > 0 {
			n := min(uint32(len(p)), c.writeRemaining)
			p = p[n:]
			c.writeRemaining -= n

			continue
		}

		n := min(len(p), 28-len(c.writeHeader))
		c.writeHeader = append(c.writeHeader, p[:n]...)
		p = p[n:]

		if len(c.writeHeader) < 28 {
			return
		}

		size := binary.BigEndian.Uint32(c.writeHeader[:4])
		if size < 28 {
			c.writeHeader = c.writeHeader[:0]

			continue
		}

		if binary.BigEndian.Uint32(c.writeHeader[4:8]) == consoleProgram &&
			binary.BigEndian.Uint32(c.writeHeader[8:12]) == 1 &&
			binary.BigEndian.Uint32(c.writeHeader[12:16]) == consoleConnectProcedure &&
			binary.BigEndian.Uint32(c.writeHeader[16:20]) == 0 &&
			binary.BigEndian.Uint32(c.writeHeader[24:28]) == 0 {
			c.connectSerial = binary.BigEndian.Uint32(c.writeHeader[20:24])
		}

		c.writeRemaining = size - 28
		c.writeHeader = c.writeHeader[:0]
	}
}

func (c *consoleTransport) Read(p []byte) (int, error) {
	for len(c.pending) == 0 {
		frame, err := c.readFrame()
		if err != nil {
			c.finish(err)
			c.waitForSetupPublication()

			return 0, err
		}

		if binary.BigEndian.Uint32(frame[16:20]) != consoleSerial {
			connectReply, success, validateErr := c.classifyConnectReply(frame)
			if validateErr != nil {
				c.finish(validateErr)
				c.waitForSetupPublication()

				return 0, validateErr
			}

			if connectReply {
				if c.connectDelivered {
					continue
				}

				c.connectDelivered = true
				c.connectReplySeen = success
			}

			c.pending = binary.BigEndian.AppendUint32(nil, uint32(len(frame)+4))
			c.pending = append(c.pending, frame...)

			break
		}

		if err = c.route(frame); err != nil {
			c.finish(err)
			c.waitForSetupPublication()

			return 0, err
		}
	}

	n := copy(p, c.pending)
	c.pending = c.pending[n:]

	return n, nil
}

func (c *consoleTransport) waitForSetupPublication() {
	if c.connectReplySeen {
		// The pinned SDK publishes a replacement disconnected channel only
		// after CONNECT_OPEN returns. Every terminal reader path after the
		// matching success reply must wait for that publication.
		<-c.setupComplete
	}
}

func (c *consoleTransport) classifyConnectReply(frame []byte) (connectReply, success bool, err error) {
	// Write holds writeMu while forwarding and then observing bytes. Taking the
	// same lock here prevents a peer reply from being classified before the
	// corresponding CONNECT_OPEN serial has been recorded. Observing only the
	// reported written prefix preserves split and failed-write framing.
	c.writeMu.Lock()
	serial := c.connectSerial
	c.writeMu.Unlock()

	if serial == 0 || binary.BigEndian.Uint32(frame[16:20]) != serial {
		return false, false, nil
	}

	if binary.BigEndian.Uint32(frame[:4]) != consoleProgram ||
		binary.BigEndian.Uint32(frame[4:8]) != 1 ||
		binary.BigEndian.Uint32(frame[8:12]) != consoleConnectProcedure ||
		binary.BigEndian.Uint32(frame[12:16]) != 1 {
		return false, false, errors.New("invalid CONNECT_OPEN reply header")
	}

	status := binary.BigEndian.Uint32(frame[20:24])
	if status != 0 && status != 1 {
		return false, false, errors.New("invalid CONNECT_OPEN reply status")
	}

	return true, status == 0, nil
}

func consoleFrameStatus(frame []byte) error {
	if binary.BigEndian.Uint32(frame[:4]) != consoleProgram || binary.BigEndian.Uint32(frame[4:8]) != 1 || binary.BigEndian.Uint32(frame[8:12]) != consoleProcedure {
		return errors.New("invalid console RPC header")
	}

	kind, status := binary.BigEndian.Uint32(frame[12:16]), binary.BigEndian.Uint32(frame[20:24])
	if status == 1 {
		return consoleFrameError(frame)
	}

	if kind == 3 && (status == 0 || status == 2 && len(frame) == 24) {
		return io.EOF
	}

	return nil
}

func (c *consoleTransport) route(frame []byte) error {
	if err := consoleFrameStatus(frame); err != nil {
		return err
	}

	switch binary.BigEndian.Uint32(frame[12:16]) {
	case 1:
		if binary.BigEndian.Uint32(frame[20:24]) != 0 {
			return errors.New("invalid console reply status")
		}
	case 3:
		if binary.BigEndian.Uint32(frame[20:24]) != 2 {
			return errors.New("invalid console stream status")
		}
	default:
		return errors.New("unexpected console message type")
	}

	select {
	case c.frames <- frame:
		return nil
	case <-c.done:
		return net.ErrClosed
	default:
		return errors.New("console output queue overflow")
	}
}

func (c *consoleTransport) readFrame() ([]byte, error) {
	var prefix [4]byte
	if _, err := io.ReadFull(c.Conn, prefix[:]); err != nil {
		return nil, err
	}

	size := binary.BigEndian.Uint32(prefix[:])
	if size < 28 || size > consoleMaxFrame {
		return nil, fmt.Errorf("invalid console RPC frame size %d", size)
	}

	frame := make([]byte, size-4)
	_, err := io.ReadFull(c.Conn, frame)

	return frame, err
}

func (c *consoleTransport) next() ([]byte, error) {
	// Drain bytes received before EOF before reporting the terminal error.
	select {
	case frame := <-c.frames:
		return frame, nil
	default:
	}

	select {
	case frame := <-c.frames:
		return frame, nil
	case <-c.done:
		select {
		case frame := <-c.frames:
			return frame, nil
		default:
		}

		c.mu.Lock()
		defer c.mu.Unlock()

		return nil, c.err
	}
}

func (c *consoleTransport) send(kind, status uint32, payload []byte) error {
	packet := make([]byte, 0, 28+len(payload))
	for _, value := range []uint32{uint32(28 + len(payload)), consoleProgram, 1, consoleProcedure, kind, consoleSerial, status} {
		packet = binary.BigEndian.AppendUint32(packet, value)
	}

	packet = append(packet, payload...)

	_, err := c.Write(packet)
	if err != nil {
		c.finish(err)
	}

	return err
}

func consoleString(dst []byte, value string) []byte {
	dst = binary.BigEndian.AppendUint32(dst, uint32(len(value)))
	dst = append(dst, value...)

	return append(dst, make([]byte, (4-len(value)%4)%4)...)
}

func consoleFrameError(frame []byte) error {
	// remote_error starts with code, domain, and an optional message. Retain the
	// libvirt error code even when the server omits a human-readable message.
	if len(frame) < 36 {
		return errors.New("invalid console RPC error")
	}

	code := binary.BigEndian.Uint32(frame[24:28])
	message := "libvirt console attachment failed"

	if binary.BigEndian.Uint32(frame[32:36]) == 1 && len(frame) >= 40 {
		size := binary.BigEndian.Uint32(frame[36:40])
		if uint64(size) <= uint64(len(frame)-40) {
			message = string(frame[40 : 40+size])
		}
	}

	return libvirt.Error{Code: code, Message: message}
}

type consoleStream struct {
	transport    *consoleTransport
	disconnected <-chan struct{}
	stop         func() bool
	readMu       sync.Mutex
	writeMu      sync.Mutex
	pending      []byte
}

func (s *consoleStream) Read(p []byte) (int, error) {
	s.readMu.Lock()
	defer s.readMu.Unlock()

	if len(p) == 0 {
		return 0, nil
	}

	for len(s.pending) == 0 {
		frame, err := s.transport.next()
		if err != nil {
			return 0, err
		}

		if binary.BigEndian.Uint32(frame[12:16]) != 3 || binary.BigEndian.Uint32(frame[20:24]) != 2 {
			return 0, consoleFrameError(frame)
		}

		s.pending = frame[24:]
	}

	n := copy(p, s.pending)
	s.pending = s.pending[n:]

	return n, nil
}

func (s *consoleStream) Write(p []byte) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	written := 0

	for len(p) > 0 {
		n := min(len(p), consoleChunk)
		if err := s.transport.send(3, 2, p[:n]); err != nil {
			return written, err
		}

		written += n
		p = p[n:]
	}

	return written, nil
}

func (s *consoleStream) Close() error {
	s.close()

	return nil
}

func (s *consoleStream) close() {
	s.stop()
	s.transport.finish(io.EOF)
	<-s.disconnected
}
