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
	"syscall"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/provision"
)

const (
	imageCachePid = "image-cache.pid"
	imageCacheLog = "image-cache.log"
)

// CreateImageCache creates the Image Cache server.
func (p *Provisioner) CreateImageCache(state *State, clusterReq provision.ClusterRequest) error {
	pidPath := state.GetRelativePath(imageCachePid)

	logFile, err := os.OpenFile(state.GetRelativePath(imageCacheLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint:errcheck

	gatewayAddrs := xslices.Map(clusterReq.Network.GatewayAddrs, netip.Addr.String)
	gatewayAddrs = xslices.Map(gatewayAddrs, func(s string) string { return net.JoinHostPort(s, fmt.Sprint(clusterReq.Network.ImageCachePort)) })

	args := []string{
		"image", "cache-serve",
		"--address", gatewayAddrs[0],
		"--image-cache-path", clusterReq.Network.ImageCachePath,
		"--tls-cert-file", clusterReq.Network.ImageCacheTLSCertFile,
		"--tls-key-file", clusterReq.Network.ImageCacheTLSKeyFile,
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
		return fmt.Errorf("error writing imageCache PID file: %w", err)
	}

	return nil
}

// DestroyImageCache destoys Image Cache server.
func (p *Provisioner) DestroyImageCache(state *State) error {
	pidPath := state.GetRelativePath(imageCachePid)

	return StopProcessByPidfile(pidPath)
}
