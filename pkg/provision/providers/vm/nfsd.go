// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/siderolabs/gen/xslices"
	"github.com/smallfz/libnfs-go/auth"
	"github.com/smallfz/libnfs-go/backend"
	"github.com/smallfz/libnfs-go/fs"
	"github.com/smallfz/libnfs-go/server"
	"github.com/smallfz/libnfs-go/unixfs"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/pkg/provision"
)

const (
	nfsdPid     = "nfsd.pid"
	nfsdLog     = "nfsd.log"
	nfsdWorkdir = "nfsd.d"
)

// NFSd starts a userspace NFS server on the given IPs.
func NFSd(ips []net.IP, workdir string) error {
	var eg errgroup.Group

	if workdir == "" {
		return fmt.Errorf("workdir must be specified")
	}

	f, err := unixfs.New(workdir)
	if err != nil {
		return fmt.Errorf("failed to create unixfs: %w", err)
	}

	for _, ip := range ips {
		eg.Go(func() error {
			srv, err := server.NewServerTCP(net.JoinHostPort(ip.String(), "2049"), backend.New(
				func() fs.FS { return f },
				auth.Null,
			))
			if err != nil {
				return fmt.Errorf("failed to create TCP server: %w", err)
			}

			return srv.Serve()
		})
	}

	return eg.Wait()
}

// CreateNFSd creates the NFSd server.
func (p *Provisioner) CreateNFSd(state *State, clusterReq provision.ClusterRequest) error {
	pidPath := state.GetRelativePath(nfsdPid)

	logFile, err := os.OpenFile(state.GetRelativePath(nfsdLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint:errcheck

	nfsdPath := state.GetRelativePath(nfsdWorkdir)

	if err = os.MkdirAll(nfsdPath, 0o755); err != nil {
		return fmt.Errorf("error creating nfsd workdir: %w", err)
	}

	defer func() {
		os.RemoveAll(nfsdPath) //nolint:errcheck
	}()

	gatewayAddrs := xslices.Map(clusterReq.Network.GatewayAddrs, netip.Addr.String)

	args := []string{
		"nfsd-launch",
		"--addr", strings.Join(gatewayAddrs, ","),
		"--workdir", nfsdPath,
	}

	cmd := exec.Command(clusterReq.SelfExecutable, args...) //nolint:noctx // runs in background
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // daemonize
	}

	if err = cmd.Start(); err != nil {
		return err
	}

	if err = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), os.ModePerm); err != nil {
		return fmt.Errorf("error writing nfsd PID file: %w", err)
	}

	return nil
}

// DestroyNFSd destoys NFSd server.
func (p *Provisioner) DestroyNFSd(state *State) error {
	pidPath := state.GetRelativePath(nfsdPid)

	return StopProcessByPidfile(pidPath)
}
