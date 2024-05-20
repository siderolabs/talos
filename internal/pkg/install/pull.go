// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"context"
	"errors"
	"fmt"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"

	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// PullAndValidateInstallerImage pulls down the installer and validates that it can run.
//
//nolint:gocyclo
func PullAndValidateInstallerImage(ctx context.Context, reg config.Registries, ref string) error {
	// Pull down specified installer image early so we can bail if it doesn't exist in the upstream registry
	containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	const containerID = "validate"

	client, err := containerd.New(constants.SystemContainerdAddress)
	if err != nil {
		return err
	}

	defer client.Close() //nolint:errcheck

	img, err := image.Pull(containerdctx, reg, client, ref, image.WithSkipIfAlreadyPulled())
	if err != nil {
		return err
	}

	// See if there's previous container/snapshot to clean up
	var oldcontainer containerd.Container

	if oldcontainer, err = client.LoadContainer(containerdctx, containerID); err == nil {
		if err = oldcontainer.Delete(containerdctx, containerd.WithSnapshotCleanup); err != nil {
			return fmt.Errorf("error deleting old container instance: %w", err)
		}
	}

	if err = client.SnapshotService("").Remove(containerdctx, containerID); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("error cleaning up stale snapshot: %w", err)
	}

	// Launch the container with a known help command for a simple check to make sure the image is valid
	args := []string{
		"/bin/installer",
		"--help",
	}

	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(img),
		oci.WithProcessArgs(args...),
	}

	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(img),
		containerd.WithNewSnapshot(containerID, img),
		containerd.WithNewSpec(specOpts...),
	}

	container, err := client.NewContainer(containerdctx, containerID, containerOpts...)
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer container.Delete(containerdctx, containerd.WithSnapshotCleanup)

	task, err := container.NewTask(containerdctx, cio.NullIO)
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer task.Delete(containerdctx)

	exitStatusC, err := task.Wait(containerdctx)
	if err != nil {
		return err
	}

	if err = task.Start(containerdctx); err != nil {
		return err
	}

	status := <-exitStatusC

	code, _, err := status.Result()
	if err != nil {
		return err
	}

	if code != 0 {
		return errors.New("installer help returned non-zero exit. assuming invalid installer")
	}

	return nil
}
