// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/talos-systems/talos/internal/pkg/provision/providers/vm"
)

// LaunchConfig is passed in to the Launch function over stdin.
type LaunchConfig struct {
	DiskPath       string
	VCPUCount      int64
	MemSize        int64
	QemuExecutable string
}

// Launch a control process around qemu VM manager.
//
// This function is invoked from 'talosctl qemu-launch' hidden command
// and wraps starting, controlling 'qemu' VM process.
//
// Launch restarts VM forever until control process is stopped itself with a signal.
//
// Process is expected to receive configuration on stdin. Current working directory
// should be cluster state directory, process output should be redirected to the
// logfile in state directory.
//
// When signals SIGINT, SIGTERM are received, control process stops qemu and exits.
//
//nolint: gocyclo
func Launch() error {
	var config LaunchConfig

	if err := vm.ReadConfig(&config); err != nil {
		return err
	}

	c := vm.ConfigureSignals()

	cmd := exec.Command(
		config.QemuExecutable,
		"-m", strconv.FormatInt(config.MemSize, 10),
		"-drive", fmt.Sprintf("format=raw,file=%s", config.DiskPath),
		"-smp", fmt.Sprintf("cpus=%d", config.VCPUCount),
		"-accel",
		"kvm",
		"-nographic",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error)

	go func() {
		done <- cmd.Wait()
	}()

	select {
	case sig := <-c:
		fmt.Fprintf(os.Stderr, "exiting VM as signal %s was received\n", sig)

		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process %w", err)
		}

		return fmt.Errorf("process stopped")
	case err := <-done:
		if err != nil {
			return fmt.Errorf("process exited with error %s", err)
		}

		return fmt.Errorf("process stopped")
	}
}
