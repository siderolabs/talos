// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package unix

import (
	"context"
	"errors"
	"net"
	"syscall"

	"google.golang.org/grpc/credentials"

	"github.com/siderolabs/talos/internal/pkg/miniprocfs"
)

type unixCreds struct{}

// NewServerCredentials creates gRPC TransportCredentials that extract Unix socket
// peer credentials (PID, UID, GID) via SO_PEERCRED from the connecting process.
// Must be used with a server listening on a Unix domain socket.
func NewServerCredentials() credentials.TransportCredentials {
	return &unixCreds{}
}

func (unixCreds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{SecurityProtocol: "unix-peer-creds"}
}

func (unixCreds) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return rawConn, nil, nil
}

func (unixCreds) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	unixConn, ok := rawConn.(*net.UnixConn)
	if !ok {
		return nil, nil, errors.New("not a Unix connection")
	}

	rawSysConn, err := unixConn.SyscallConn()
	if err != nil {
		return nil, nil, err
	}

	var ucred *syscall.Ucred

	var credErr error

	if err = rawSysConn.Control(func(fd uintptr) {
		ucred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return nil, nil, err
	}

	if credErr != nil {
		return nil, nil, credErr
	}

	mountNamespace, _ := miniprocfs.ReadMountNamespace(ucred.Pid)

	return rawConn, PeerCredentials{
		PID:            ucred.Pid,
		UID:            ucred.Uid,
		GID:            ucred.Gid,
		MountNamespace: mountNamespace,
	}, nil
}

func (unixCreds) Clone() credentials.TransportCredentials {
	return &unixCreds{}
}

func (unixCreds) OverrideServerName(_ string) error {
	return nil
}
