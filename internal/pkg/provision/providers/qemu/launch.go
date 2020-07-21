// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/talos-systems/talos/internal/pkg/provision/providers/vm"
)

// LaunchConfig is passed in to the Launch function over stdin.
type LaunchConfig struct {
	DiskPath        string
	VCPUCount       int64
	MemSize         int64
	QemuExecutable  string
	KernelImagePath string
	InitrdPath      string
	KernelArgs      string
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

	for {
		err := func() error {

			args := []string{
				"-m", strconv.FormatInt(config.MemSize, 10),
				"-drive", fmt.Sprintf("format=raw,if=virtio,file=%s", config.DiskPath),
				"-smp", fmt.Sprintf("cpus=%d", config.VCPUCount),
				"-accel",
				"kvm",
				"-nographic",
			}

			disk, err := os.Open(config.DiskPath)
			if err != nil {
				return fmt.Errorf("failed to open disk file %w", err)
			}

			// check if disk is empty
			checkSize := 512
			buf := make([]byte, checkSize)

			_, err = disk.Read(buf)
			if err != nil {
				return fmt.Errorf("failed to read disk file %w", err)
			}

			if bytes.Equal(buf, make([]byte, checkSize)) {
				args = append(args,
					"-kernel", config.KernelImagePath,
					"-initrd", config.InitrdPath,
					"-append", config.KernelArgs,
					"-no-reboot",
				)
			}

			fmt.Fprintf(os.Stderr, "starting qemu with args:\n%s\n", strings.Join(args, " "))
			cmd := exec.Command(
				config.QemuExecutable,
				args...,
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

				// graceful exit
				return nil
			}
		}()

		if err != nil {
			return err
		}
	}
}
