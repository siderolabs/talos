// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	efiles "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	mountv3 "github.com/siderolabs/talos/internal/pkg/mount/v3"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/fsopen"
)

// etcRootPath is the host /etc directory.
const etcRootPath = "/etc"

// setupEtcOverlay composes /etc as a writable overlay that is bind-mounted read-only.
//
// A managed tmpfs (the overlay UPPER) is layered over the existing rootfs /etc (the squashfs
// static defaults, the lower) into a single WRITABLE overlay. machined keeps that overlay's
// detached mount (the returned xfs.Root) and controllers write managed files THROUGH it (into the
// upper).
func setupEtcOverlay(etcPath string, inContainer bool, upperFSOpts []fsopen.Option, logger *zap.Logger) (xfs.Root, func() error, error) {
	printer := logger.Sugar().Infof

	// Clone the static rootfs /etc as the detached lower layer (squashfs, read-only).
	lowerFd, err := mountv3.OpenTreeClone(etcPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to clone lower /etc: %w", err)
	}

	defer lowerFd.Close() //nolint:errcheck

	etcRoot, err := mountv3.NewSecureWritableOverlay([]int{lowerFd.Fd()}, upperFSOpts, printer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to compose writable /etc overlay: %w", err)
	}

	if err := seedBindMountFiles(etcRoot, etcPath, inContainer); err != nil {
		return nil, nil, err
	}

	overlayFd, err := etcRoot.Fd()
	if err != nil {
		return nil, nil, err
	}

	if err := mountv3.BindReadonlyFd(overlayFd, etcPath); err != nil {
		return nil, nil, fmt.Errorf("failed to bind /etc overlay read-only at %q: %w", etcPath, err)
	}

	// /etc/cni and /etc/kubernetes must be writable by the CNI and kubelet, but /etc is now the
	// read-only overlay bind. Mount plain writable tmpfs on top of those mountpoints.
	cniTmpfs := mountv3.NewSecureTmpfs(filepath.Join(etcPath, "cni"), "0755", constants.CNISELinuxLabel, printer)
	if _, err := cniTmpfs.Mount(); err != nil {
		return nil, nil, fmt.Errorf("failed to mount /etc/cni tmpfs: %w", err)
	}

	kubeTmpfs := mountv3.NewSecureTmpfs(filepath.Join(etcPath, "kubernetes"), "0755", constants.KubernetesConfigSELinuxLabel, printer)
	if _, err := kubeTmpfs.Mount(); err != nil {
		return nil, nil, fmt.Errorf("failed to mount /etc/kubernetes tmpfs: %w", err)
	}

	printer("composed writable /etc overlay, bound read-only at %s", etcPath)

	unmount := func() error {
		return errors.Join(
			kubeTmpfs.Unmount(),
			cniTmpfs.Unmount(),
			unix.Unmount(etcPath, unix.MNT_DETACH),
			etcRoot.Close(),
		)
	}

	return etcRoot, unmount, nil
}

// seedBindMountFiles seeds the files that are bind-mounted into /etc from outside the rootfs.
//
// Such a file is a submount of /etc, and the overlay lower does not carry it: the lower is cloned
// with OPEN_TREE_CLONE (no AT_RECURSIVE) and overlayfs does not cross submounts inside a lower
// either. Without seeding, the overlay would expose the empty placeholder shipped in the rootfs
// instead of the bind-mounted content.
func seedBindMountFiles(etcRoot xfs.Root, etcPath string, inContainer bool) error {
	// extensions.yaml is bound in from the initramfs, so extension detection needs it.
	names := []string{"extensions.yaml"}

	// In container mode the runtime binds in resolv.conf (the resolvers to use) and hostname (the
	// container name). No controller manages either one then, and the container platform reads
	// hostname back off the file to publish the platform hostname, so both must survive the
	// compose or the machine comes up on a generated hostname.
	if inContainer {
		names = append(names, "resolv.conf", "hostname")
	}

	for _, name := range names {
		if err := seedBindMountFile(etcRoot, etcPath, name); err != nil {
			return fmt.Errorf("failed to seed /etc/%s: %w", name, err)
		}
	}

	return nil
}

// seedBindMountFile copies /<name> from etcPath into the writable xfs.Root
// this is needed for bind mounted files since an overlay composed of lower
// from a directory containing bind mount files will not be present in final overlay.
func seedBindMountFile(etcRoot xfs.Root, etcPath, name string) error {
	src := filepath.Join(etcPath, name)

	contents, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read %q: %w", src, err)
	}

	return efiles.UpdateFile(etcRoot, name, contents, 0o644, constants.EtcSelinuxLabel)
}
