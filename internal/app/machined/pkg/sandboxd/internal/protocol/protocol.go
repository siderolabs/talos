// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package protocol implements the control protocol between machined and sandboxd.
//
// machined sends a Request over the control socket as one SEQPACKET datagram.
//
// For a launch, SCM_RIGHTS carries [stdin, stdout, stderr, serviceSocket].
// sandboxd - which is PID 1 inside the namespace - forks the service
// directly (no setns needed) and uses serviceSocket to (1) return a
// LaunchResponse plus the service pidfd via SCM_RIGHTS, and (2) after machined
// asks (Wait), return a WaitResponse carrying the exit status.
//
// For a setattr, SCM_RIGHTS carries [replySocket], which returns a SetattrResponse.
package protocol

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

// Request is one control datagram; exactly one field is set.
type Request struct {
	Launch  *LaunchRequest
	Setattr *SetattrRequest
}

// LaunchRequest is sent by machined for each service launch.
type LaunchRequest struct {
	Args                []string
	Env                 []string
	DroppedCapabilities []string
	SelinuxLabel        string
}

// LaunchResponse is returned once the service is started; the service pidfd is
// passed via SCM_RIGHTS alongside it. On error (Err != ""), no pidfd is sent.
type LaunchResponse struct {
	Err string
}

// WaitResponse is returned after the service exits.
type WaitResponse struct {
	ExitCode int
	ExitErr  string
}

// SetattrRequest changes the attributes of the mount at Target in the sandbox
// namespace: machined can't, mount_setattr only accepts mounts of the caller's
// namespace.
type SetattrRequest struct {
	Target string
	Set    uint64
	Clr    uint64
}

// SetattrResponse is returned once the attributes are set, Errno is set on failure.
type SetattrResponse struct {
	Errno syscall.Errno
}

// DecodeRequest decodes a control datagram that arrived with nfds descriptors.
func DecodeRequest(data []byte, nfds int) (Request, error) {
	var req Request

	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&req); err != nil {
		return req, fmt.Errorf("decoding request: %w", err)
	}

	var expected int

	switch {
	case req.Launch != nil && req.Setattr == nil:
		expected = 4 // stdin, stdout, stderr, service socket
	case req.Setattr != nil && req.Launch == nil:
		expected = 1 // reply socket
	default:
		return req, errors.New("request must set exactly one operation")
	}

	if nfds != expected {
		return req, fmt.Errorf("expected %d fds, got %d", expected, nfds)
	}

	return req, nil
}

// encode gob-encodes a message.
func encode(msg any) ([]byte, error) {
	var buf bytes.Buffer

	err := gob.NewEncoder(&buf).Encode(msg)

	return buf.Bytes(), err
}

// Setattr sends req through send with a fresh reply socket and returns sandboxd's
// answer, waiting for it up to timeout. send writes the datagram and its SCM_RIGHTS
// to the control socket.
func Setattr(send func(data, oob []byte) error, req SetattrRequest, timeout time.Duration) error {
	replyFds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("creating reply socketpair: %w", err)
	}

	replyInit := os.NewFile(uintptr(replyFds[1]), "reply-init")

	defer replyInit.Close() //nolint:errcheck

	replyConn, err := UnixConn(os.NewFile(uintptr(replyFds[0]), "reply-machined"))
	if err != nil {
		return fmt.Errorf("wrapping reply socket: %w", err)
	}

	defer replyConn.Close() //nolint:errcheck

	data, err := encode(Request{Setattr: &req})
	if err != nil {
		return fmt.Errorf("encoding setattr request: %w", err)
	}

	if err = send(data, syscall.UnixRights(int(replyInit.Fd()))); err != nil {
		return fmt.Errorf("sending setattr request: %w", err)
	}

	// Drop our copy, so a sandboxd exit reads as EOF instead of blocking.
	replyInit.Close() //nolint:errcheck

	if err = replyConn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("setting setattr response deadline: %w", err)
	}

	respBuf := make([]byte, 4096)

	n, err := replyConn.Read(respBuf)
	if err != nil {
		return fmt.Errorf("reading setattr response: %w", err)
	}

	var resp SetattrResponse

	if err = gob.NewDecoder(bytes.NewReader(respBuf[:n])).Decode(&resp); err != nil {
		return fmt.Errorf("decoding setattr response: %w", err)
	}

	if resp.Errno != 0 {
		return fmt.Errorf("sandbox setattr %q: %w", req.Target, resp.Errno)
	}

	return nil
}

// ServeSetattr applies req and answers on reply, which it closes.
func ServeSetattr(reply *os.File, req SetattrRequest, apply func(SetattrRequest) error) error {
	replyConn, err := UnixConn(reply)
	if err != nil {
		return fmt.Errorf("wrapping reply socket: %w", err)
	}

	defer replyConn.Close() //nolint:errcheck

	var resp SetattrResponse

	if err = apply(req); err != nil && !errors.As(err, &resp.Errno) {
		resp.Errno = syscall.EIO
	}

	data, err := encode(resp)
	if err != nil {
		return fmt.Errorf("encoding setattr response: %w", err)
	}

	_, err = replyConn.Write(data)

	return err
}

// UnixConn wraps an *os.File socket as a *net.UnixConn. FileConn duplicates the
// fd, so the original file is closed.
func UnixConn(f *os.File) (*net.UnixConn, error) {
	conn, err := net.FileConn(f)
	f.Close() //nolint:errcheck

	if err != nil {
		return nil, err
	}

	uc, ok := conn.(*net.UnixConn)
	if !ok {
		conn.Close() //nolint:errcheck

		return nil, errors.New("not a unix socket")
	}

	return uc, nil
}
