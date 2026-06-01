// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/siderolabs/talos/pkg/provision"
)

func (p *provisioner) preflightChecks(ctx context.Context, request provision.ClusterRequest, options provision.Options, arch Arch) error {
	checkContext := preflightCheckContext{
		request: request,
		options: options,
		arch:    arch,
	}

	for _, check := range []func(ctx context.Context) error{
		checkContext.verifyRoot,
		checkContext.qemuExecutable,
		checkContext.checkFlashImages,
	} {
		if err := check(ctx); err != nil {
			return err
		}
	}

	return checkContext.verifyPlatformSpecific(ctx)
}

type preflightCheckContext struct {
	request provision.ClusterRequest
	options provision.Options
	arch    Arch
}

func (check *preflightCheckContext) verifyRoot(context.Context) error {
	if os.Geteuid() != 0 {
		return errors.New("error: please run as root user (CNI, qemu hvf requirement), we recommend running with `sudo -E`")
	}

	return nil
}

func (check *preflightCheckContext) qemuExecutable(context.Context) error {
	if check.arch.QemuExecutable() == "" {
		return fmt.Errorf("QEMU executable (qemu-system-%s or qemu-kvm) not found, please install QEMU with package manager", check.arch.QemuArch())
	}

	return nil
}

func (check *preflightCheckContext) checkFlashImages(context.Context) error {
	for _, flashImage := range check.arch.PFlash(check.options.UEFIEnabled, check.options.ExtraUEFISearchPaths) {
		if len(flashImage.SourcePaths) == 0 {
			continue
		}

		found := false

		for _, path := range flashImage.SourcePaths {
			_, err := os.Stat(path)
			if err == nil {
				found = true

				break
			}
		}

		if !found {
			return fmt.Errorf("the required flash image was not found in any of the expected paths for (%q), "+
				"please install it with the package manager or specify --extra-uefi-search-paths", flashImage.SourcePaths)
		}
	}

	return nil
}
