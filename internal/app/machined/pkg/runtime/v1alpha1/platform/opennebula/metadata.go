// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package opennebula provides the OpenNebula platform implementation.
package opennebula

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/siderolabs/go-blockdevice/blockdevice/filesystem"
	"github.com/siderolabs/go-blockdevice/blockdevice/probe"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
)

const (
	configISOLabel = "context"
	oneContextPath = "context.sh"
	mnt            = "/mnt"
)

func (o *OpenNebula) contextFromCD() (oneContext []byte, err error) {
	var dev *probe.ProbedBlockDevice

	dev, err = probe.GetDevWithFileSystemLabel(strings.ToLower(configISOLabel))
	if err != nil {
		dev, err = probe.GetDevWithFileSystemLabel(strings.ToUpper(configISOLabel))
		if err != nil {
			return nil, fmt.Errorf("failed to find %s iso: %w", configISOLabel, err)
		}
	}

	//nolint:errcheck
	defer dev.Close()

	sb, err := filesystem.Probe(dev.Path)
	if err != nil || sb == nil {
		return nil, errors.ErrNoConfigSource
	}

	log.Printf("found config disk (context) at %s", dev.Path)

	if err = unix.Mount(dev.Path, mnt, sb.Type(), unix.MS_RDONLY, ""); err != nil {
		return nil, fmt.Errorf("failed to mount iso: %w", err)
	}

	log.Printf("fetching context from: %s/", oneContextPath)

	oneContext, err = os.ReadFile(filepath.Join(mnt, oneContextPath))
	if err != nil {
		return nil, fmt.Errorf("read config: %s", err.Error())
	}

	if err = unix.Unmount(mnt, 0); err != nil {
		return nil, fmt.Errorf("failed to unmount: %w", err)
	}

	if oneContext == nil {
		return nil, errors.ErrNoConfigSource
	}

	return oneContext, nil
}
