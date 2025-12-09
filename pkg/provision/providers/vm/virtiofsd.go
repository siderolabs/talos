// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/siderolabs/talos/pkg/provision"
)

const (
	virtiofsdPid = "virtiofsd.pid"
	virtiofsdLog = "virtiofsd.log"
)

// FindVirtiofsd tries to find the virtiofsd binary in common locations.
func (p *Provisioner) FindVirtiofsd() (string, error) {
	return p.findVirtiofsd()
}

// Virtiofsd starts virtiofsd and restarts it if it exits.
// The restart is needed, because the virtiofsd exits when client disconnects.
func Virtiofsd(ctx context.Context, virtiofsdBin, share, socket string) error {
	if virtiofsdBin == "" {
		return errors.New("virtiofsd binary path is empty")
	}

	if err := os.MkdirAll(share, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create shared dir %s: %v\n", share, err)
	}

	args := []string{
		"--shared-dir", share,
		"--socket-path", socket,
		"--announce-submounts",
		"--inode-file-handles=mandatory",
	}

	fmt.Printf("Starting virtiofsd with restart loop: %s %s\n",
		virtiofsdBin, strings.Join(args, " "))

	for {
		err := runVirtiofsd(ctx, share, virtiofsdBin, args)
		switch {
		case err == nil:
			fmt.Printf("virtiofsd exited - restarting...\n")

		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return nil

		default:
			fmt.Printf("virtiofsd exited: %v - restarting...\n", err)
		}
	}
}

func runVirtiofsd(ctx context.Context, share, virtiofsdBin string, args []string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	cmd := exec.CommandContext(ctx, virtiofsdBin, args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdoutPipe.Close() //nolint:errcheck

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	defer stderrPipe.Close() //nolint:errcheck

	if err := cmd.Start(); err != nil {
		return err
	}

	fmt.Printf("virtiofsd started with PID %d\n", cmd.Process.Pid)

	go printPipe(share, os.Stdout, stdoutPipe)
	go printPipe(share, os.Stderr, stderrPipe)

	err = cmd.Wait()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	return err
}

func printPipe(prefix string, wr io.Writer, r io.Reader) {
	if len(prefix) > 32 {
		// print only last 32 characters
		prefix = prefix[len(prefix)-32:]
	}

	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		fmt.Fprintf(wr, "[%s] %s\n", prefix, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(wr, "error reading from pipe: %v\n", err)
	}
}

// CreateVirtiofsd creates the Virtiofsd server.
func (p *Provisioner) CreateVirtiofsd(state *State, clusterReq provision.ClusterRequest, virtiofdPath string) error {
	return p.startVirtiofsd(state, clusterReq, virtiofdPath)
}

// DestroyVirtiofsd destoys Virtiofsd server.
func (p *Provisioner) DestroyVirtiofsd(state *State) error {
	return p.stopVirtiofsd(state)
}
