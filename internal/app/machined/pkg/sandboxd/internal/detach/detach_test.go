// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package detach_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/sandboxd/internal/detach"
)

// onThread runs f on a locked thread which is never handed back to the Go runtime, so the
// mount namespace f moves the thread to dies with it.
func onThread(f func() error) error {
	errCh := make(chan error, 1)

	go func() {
		runtime.LockOSThread()

		errCh <- f()
	}()

	return <-errCh
}

func isMountRoot(path string) bool {
	var stx unix.Statx_t

	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, 0, &stx); err != nil {
		panic(err)
	}

	return stx.Attributes&unix.STATX_ATTR_MOUNT_ROOT != 0
}

type mountStep struct {
	source, target, fstype string
	flags                  uintptr
}

func mountAll(steps []mountStep) error {
	for _, step := range steps {
		if err := unix.Mount(step.source, step.target, step.fstype, step.flags, ""); err != nil {
			return fmt.Errorf("mount %+v: %w", step, err)
		}
	}

	return nil
}

// setupHost stands for machined's namespace: a shared tmpfs (/system) with a tmpfs on a directory below
// a plain one (the system containerd state in /system/run) holding a socket, and maybe a mount under it
// (a container rootfs).
func setupHost(dir string, privateState, submount bool) error {
	state := filepath.Join(dir, "run", "state")
	sub := filepath.Join(state, "sub")

	if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
		return fmt.Errorf("unshare host: %w", err)
	}

	if err := mountAll([]mountStep{
		{target: "/", flags: unix.MS_REC | unix.MS_PRIVATE},
		{source: "tmpfs", target: dir, fstype: "tmpfs"},
		{target: dir, flags: unix.MS_SHARED},
	}); err != nil {
		return err
	}

	if err := os.MkdirAll(state, 0o700); err != nil {
		return err
	}

	propagation := uintptr(unix.MS_SHARED)

	if privateState {
		propagation = unix.MS_PRIVATE
	}

	if err := mountAll([]mountStep{
		{source: "tmpfs", target: state, fstype: "tmpfs"},
		{target: state, flags: propagation},
	}); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(state, "containerd.sock"), nil, 0o600); err != nil {
		return err
	}

	if !submount {
		return nil
	}

	if err := os.Mkdir(sub, 0o700); err != nil {
		return err
	}

	return mountAll([]mountStep{{source: "tmpfs", target: sub, fstype: "tmpfs"}})
}

// enterCopy moves the calling thread to a copy of the host namespace, as sandboxd is started.
func enterCopy(hostNS string) error {
	if err := unix.Unshare(unix.CLONE_FS); err != nil {
		return fmt.Errorf("unshare fs: %w", err)
	}

	fd, err := unix.Open(hostNS, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}

	defer unix.Close(fd) //nolint:errcheck

	if err = unix.Setns(fd, unix.CLONE_NEWNS); err != nil {
		return fmt.Errorf("enter host namespace: %w", err)
	}

	if err = unix.Unshare(unix.CLONE_NEWNS); err != nil {
		return fmt.Errorf("unshare sandbox: %w", err)
	}

	return nil
}

// detachInCopy detaches the state in a copy of the host namespace.
func detachInCopy(hostNS, dir string) error {
	state := filepath.Join(dir, "run", "state")

	if err := enterCopy(hostNS); err != nil {
		return err
	}

	detached, err := detach.Mount(state)
	if err != nil {
		return err
	}

	if !detached || isMountRoot(state) {
		return fmt.Errorf("%s was not detached in the sandbox namespace", state)
	}

	if _, err = os.Stat(filepath.Join(state, "containerd.sock")); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("the socket is still in the sandbox namespace: %v", err)
	}

	// the directory left behind is on the tmpfs, which stays
	if detached, err = detach.Mount(state); err != nil || detached {
		return fmt.Errorf("detaching %s again: detached %v, error %v", state, detached, err)
	}

	if !isMountRoot(dir) {
		return fmt.Errorf("%s is gone from the sandbox namespace", dir)
	}

	return nil
}

func TestMountNotMounted(t *testing.T) {
	dir := t.TempDir()

	detached, err := detach.Mount(dir)
	require.NoError(t, err)
	require.False(t, detached)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "containerd.sock"), nil, 0o600))

	_, err = detach.Mount(dir)
	require.ErrorContains(t, err, "is not empty after detaching it")
}

func TestMount(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}

	for _, test := range []struct {
		name         string
		privateState bool
		submount     bool
	}{
		{
			name:         "private state",
			privateState: true,
		},
		{
			name:         "private state with submount",
			privateState: true,
			submount:     true,
		},
		{
			name: "shared state",
		},
		{
			name:     "shared state with submount",
			submount: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()

			hostReady := make(chan string)
			hostCheck := make(chan struct{})
			hostResult := make(chan error, 1)

			go func() {
				hostResult <- onThread(func() error {
					if err := setupHost(dir, test.privateState, test.submount); err != nil {
						return err
					}

					hostReady <- fmt.Sprintf("/proc/%d/task/%d/ns/mnt", os.Getpid(), unix.Gettid())

					<-hostCheck

					paths := []string{filepath.Join(dir, "run", "state")}

					if test.submount {
						paths = append(paths, filepath.Join(dir, "run", "state", "sub"))
					}

					for _, path := range paths {
						if !isMountRoot(path) {
							return fmt.Errorf("detaching in the sandbox namespace unmounted %s in the host namespace", path)
						}
					}

					return nil
				})
			}()

			var hostNS string

			select {
			case hostNS = <-hostReady:
			case err := <-hostResult:
				t.Fatalf("host namespace: %v", err)
			}

			sandboxErr := onThread(func() error { return detachInCopy(hostNS, dir) })

			close(hostCheck)

			require.NoError(t, sandboxErr)
			require.NoError(t, <-hostResult)
		})
	}
}
