// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protocol_test

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/sandboxd/internal/protocol"
)

func encode(t *testing.T, msg any) []byte {
	t.Helper()

	var buf bytes.Buffer

	require.NoError(t, gob.NewEncoder(&buf).Encode(msg))

	return buf.Bytes()
}

func TestDecodeRequest(t *testing.T) {
	launch := protocol.Request{Launch: &protocol.LaunchRequest{Args: []string{"/bin/true"}}}
	setattr := protocol.Request{Setattr: &protocol.SetattrRequest{Target: "/var/mnt/a", Set: 1, Clr: 2}}

	for _, tc := range []struct {
		name string
		data []byte
		nfds int
		err  string
	}{
		{name: "launch", data: encode(t, launch), nfds: 4},
		{name: "launch with reply fd only", data: encode(t, launch), nfds: 1, err: "expected 4 fds, got 1"},
		{name: "setattr", data: encode(t, setattr), nfds: 1},
		{name: "setattr with launch fds", data: encode(t, setattr), nfds: 4, err: "expected 1 fds, got 4"},
		{
			name: "both operations",
			data: encode(t, protocol.Request{Launch: launch.Launch, Setattr: setattr.Setattr}),
			nfds: 4,
			err:  "request must set exactly one operation",
		},
		{name: "no operation", data: encode(t, protocol.Request{}), nfds: 0, err: "request must set exactly one operation"},
		{name: "garbage", data: []byte("not gob"), nfds: 1, err: "decoding request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, err := protocol.DecodeRequest(tc.data, tc.nfds)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)

				return
			}

			require.NoError(t, err)

			if tc.nfds == 4 {
				assert.Equal(t, launch, req)
			} else {
				assert.Equal(t, setattr, req)
			}
		})
	}
}

// controlPair returns both ends of a control socket.
func controlPair(t *testing.T) (machined, sandboxd *net.UnixConn) {
	t.Helper()

	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_SEQPACKET|syscall.SOCK_CLOEXEC, 0)
	require.NoError(t, err)

	machined, err = protocol.UnixConn(os.NewFile(uintptr(fds[0]), "machined"))
	require.NoError(t, err)

	sandboxd, err = protocol.UnixConn(os.NewFile(uintptr(fds[1]), "sandboxd"))
	require.NoError(t, err)

	t.Cleanup(func() {
		machined.Close() //nolint:errcheck
		sandboxd.Close() //nolint:errcheck
	})

	return machined, sandboxd
}

// receive reads one control datagram as sandboxd does.
func receive(control *net.UnixConn) (protocol.Request, *os.File, error) {
	buf := make([]byte, 4096)
	oob := make([]byte, syscall.CmsgSpace(4*4))

	n, oobn, _, _, err := control.ReadMsgUnix(buf, oob)
	if err != nil {
		return protocol.Request{}, nil, err
	}

	scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return protocol.Request{}, nil, err
	}

	if len(scms) != 1 {
		return protocol.Request{}, nil, fmt.Errorf("expected 1 control message, got %d", len(scms))
	}

	fds, err := syscall.ParseUnixRights(&scms[0])
	if err != nil {
		return protocol.Request{}, nil, err
	}

	reply := os.NewFile(uintptr(fds[0]), "reply")

	req, err := protocol.DecodeRequest(buf[:n], len(fds))
	if err != nil {
		reply.Close() //nolint:errcheck

		return protocol.Request{}, nil, err
	}

	return req, reply, nil
}

func TestSetattr(t *testing.T) {
	want := protocol.SetattrRequest{Target: "/var/mnt/a", Set: 0x2, Clr: 0x70}

	for _, tc := range []struct {
		name  string
		serve func(req protocol.SetattrRequest, reply *os.File) error
		err   string
		errIs error
	}{
		{
			name: "applied",
			serve: func(req protocol.SetattrRequest, reply *os.File) error {
				return protocol.ServeSetattr(reply, req, func(got protocol.SetattrRequest) error {
					if got != want {
						return syscall.EBADMSG
					}

					return nil
				})
			},
		},
		{
			name: "failed",
			serve: func(req protocol.SetattrRequest, reply *os.File) error {
				return protocol.ServeSetattr(reply, req, func(protocol.SetattrRequest) error {
					return syscall.EINVAL
				})
			},
			err:   `sandbox setattr "/var/mnt/a": invalid argument`,
			errIs: syscall.EINVAL,
		},
		{
			// sandboxd exiting before it answers must not leave machined blocked.
			name: "no answer",
			serve: func(_ protocol.SetattrRequest, reply *os.File) error {
				return reply.Close()
			},
			err: "reading setattr response",
		},
		{
			// nor a stuck one
			name: "answer timeout",
			serve: func(_ protocol.SetattrRequest, reply *os.File) error {
				t.Cleanup(func() { reply.Close() }) //nolint:errcheck

				return nil
			},
			err:   "reading setattr response",
			errIs: os.ErrDeadlineExceeded,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machined, sandboxd := controlPair(t)

			served := make(chan error, 1)

			go func() {
				req, reply, err := receive(sandboxd)
				if err == nil {
					err = tc.serve(*req.Setattr, reply)
				}

				served <- err
			}()

			err := protocol.Setattr(func(data, oob []byte) error {
				_, _, err := machined.WriteMsgUnix(data, oob, nil)

				return err
			}, want, time.Second)

			require.NoError(t, <-served)

			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)

				if tc.errIs != nil {
					require.ErrorIs(t, err, tc.errIs)
				}

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestSetattrSendFailure(t *testing.T) {
	errSend := errors.New("control socket closed")

	err := protocol.Setattr(func([]byte, []byte) error { return errSend }, protocol.SetattrRequest{Target: "/"}, time.Second)
	require.ErrorIs(t, err, errSend)
}
