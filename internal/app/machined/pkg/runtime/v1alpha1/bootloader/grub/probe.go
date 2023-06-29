// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package grub provides the interface to the GRUB bootloader: config management, installation, etc.
package grub

import (
	"context"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/mount"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Probe probes a block device for GRUB bootloader.
//
// If the 'disk' is passed, search happens on that disk only, otherwise searches all partitions.
//
//nolint:gocyclo
func Probe(ctx context.Context, disk string) (*Config, error) {
	var grubConf *Config

	if err := mount.PartitionOp(ctx, disk, constants.BootPartitionLabel, func() error {
		var err error

		grubConf, err = Read(ConfigPath)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return grubConf, nil
}
