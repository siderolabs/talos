// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// ReExecWithUserNamespace re-executes the current process in a new user namespace.
// This allows non-root users to perform operations that require root privileges
// within the isolated user namespace.
func ReExecWithUserNamespace(ctx context.Context) error {
	// Check if we're already in a user namespace to avoid infinite recursion
	if inUserNamespace() {
		return nil
	}

	// Check if user namespaces are available
	if !userNamespacesAvailable() {
		return fmt.Errorf("user namespaces are not available on this system")
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	uid := os.Getuid()
	gid := os.Getgid()

	cmd := exec.CommandContext(ctx, exe, os.Args[1:]...)
	// Preserve the original command name (os.Args[0]) for symlink support
	cmd.Args[0] = os.Args[0]
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      uid,
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      gid,
				Size:        1,
			},
		},
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}

		return fmt.Errorf("failed to re-exec with user namespace: %w", err)
	}

	os.Exit(0)

	return nil
}

// inUserNamespace checks if the current process is already running in a user namespace.
func inUserNamespace() bool {
	// If we're root (uid 0), check if we started as non-root
	// by comparing our uid_map with the init namespace
	if os.Getuid() == 0 {
		data, err := os.ReadFile("/proc/self/uid_map")
		if err != nil {
			return false
		}

		// In init namespace: "0 0 4294967295"
		// In user namespace: "0 <real-uid> 1"
		// If the mapping is not the full range, we're in a user namespace
		return string(data) != "         0          0 4294967295\n"
	}

	return false
}

// userNamespacesAvailable checks if user namespaces can be created.
func userNamespacesAvailable() bool {
	// Try to read /proc/sys/user/max_user_namespaces
	// If it's 0, user namespaces are disabled
	data, err := os.ReadFile("/proc/sys/user/max_user_namespaces")
	if err != nil {
		// If the file doesn't exist, user namespaces might not be supported
		return false
	}

	// Check if it's not "0"
	return string(data) != "0\n"
}
