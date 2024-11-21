// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/mount"
	"github.com/containerd/containerd/v2/defaults"
	"github.com/opencontainers/image-spec/identity"

	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Puller pulls, unpacks and mounts extensions images.
type Puller struct {
	client    *containerd.Client
	snapshots []string
	mounts    []string
}

// NewPuller creates a new instance of system extensions puller helper.
func NewPuller(client *containerd.Client) (*Puller, error) {
	// prepare by ensuring empty extension directory
	if _, err := os.Stat(constants.SystemExtensionsPath); err == nil {
		if err = os.RemoveAll(constants.SystemExtensionsPath); err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(constants.SystemExtensionsPath, 0o700); err != nil {
		return nil, err
	}

	return &Puller{
		client: client,
	}, nil
}

// PullAndMount pulls the system extension images, unpacks them and mounts under well known path (constants.SystemExtensionsPath).
func (puller *Puller) PullAndMount(ctx context.Context, registriesBuilder image.RegistriesBuilder, extensions []config.Extension) error {
	snapshotService := puller.client.SnapshotService(defaults.DefaultSnapshotter)

	for i, ext := range extensions {
		extensionImage := ext.Image()

		// use numeric prefix to keep extensions sorted in a proper way
		path := fmt.Sprintf("%03d.%s", i, strings.ReplaceAll(strings.ReplaceAll(extensionImage, ":", "-"), "/", "-"))

		log.Printf("pulling extension %q", extensionImage)

		var extImg containerd.Image

		extImg, err := image.Pull(ctx, registriesBuilder, puller.client, extensionImage, image.WithSkipIfAlreadyPulled())
		if err != nil {
			return err
		}

		diffs, err := extImg.RootFS(ctx)
		if err != nil {
			return err
		}

		chainID := identity.ChainID(diffs)

		_, err = snapshotService.Stat(ctx, chainID.String())
		if err != nil {
			return err
		}

		mounts, err := snapshotService.Prepare(ctx, path, chainID.String())
		if err != nil {
			return err
		}

		puller.snapshots = append(puller.snapshots, path)

		mountTarget := filepath.Join(constants.SystemExtensionsPath, path)

		if err = os.Mkdir(mountTarget, 0o700); err != nil {
			return err
		}

		if err = mount.All(mounts, mountTarget); err != nil {
			return err
		}

		puller.mounts = append(puller.mounts, mountTarget)
	}

	return nil
}

// Cleanup the temporary stuff created by the puller process.
func (puller *Puller) Cleanup(ctx context.Context) error {
	for _, target := range puller.mounts {
		if err := mount.UnmountAll(target, 0); err != nil {
			return err
		}
	}

	snapshotService := puller.client.SnapshotService(defaults.DefaultSnapshotter)

	for _, key := range puller.snapshots {
		if err := snapshotService.Remove(ctx, key); err != nil {
			return err
		}
	}

	return os.RemoveAll(constants.SystemExtensionsPath)
}
