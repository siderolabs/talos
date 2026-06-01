// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package runtime

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/xfs/fsopen"
	"github.com/siderolabs/talos/pkg/xfs/opentree"
)

// KernelCap represents kernel capabilities that we can check at runtime.
type KernelCap interface {
	// OpentreeOnAnonymousFS returns true if the kernel supports opentree on anonymous filesystems.
	OpentreeOnAnonymousFS() (bool, error)
}

// KernelCapabilities returns a singleton instance of KernelCap.
var KernelCapabilities = sync.OnceValue(func() KernelCap {
	return &kernelCap{
		opentreeOnAnonymousFSOnce: sync.OnceValues(canOpnetreeOnAnonymousFS),
	}
})

type kernelCap struct {
	opentreeOnAnonymousFSOnce func() (bool, error)
}

// opentreeOnAnonymousFSOnce implements KernelCap.
func (k *kernelCap) OpentreeOnAnonymousFS() (bool, error) {
	return k.opentreeOnAnonymousFSOnce()
}

func canOpnetreeOnAnonymousFS() (bool, error) {
	tmpfs := fsopen.New("tmpfs")

	mntfd, err := tmpfs.Open()
	if err != nil {
		return false, fmt.Errorf("unexpected error while checking for opentree on anonymous fs support: %w", err)
	}
	defer tmpfs.Close() //nolint:errcheck

	otfs := opentree.NewFromFd(mntfd)

	_, err = otfs.Open()

	defer otfs.Close() //nolint:errcheck

	if err == nil {
		return true, nil // yes, the kernel supports this
	}

	if errors.Is(err, unix.EINVAL) {
		return false, nil // no, the kernel does not supports this
	}

	return false, fmt.Errorf("unexpected error while checking for opentree on anonymous fs support: %w", err)
}
