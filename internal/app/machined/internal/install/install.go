// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/kmsg"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// RunInstallerContainer performs an installation via the installer container.
//
//nolint: gocyclo
func RunInstallerContainer(disk, platform, ref string, reg config.Registries, opts ...Option) error {
	options := DefaultInstallOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	ctx := namespaces.WithNamespace(context.Background(), constants.SystemContainerdNamespace)

	client, err := containerd.New(constants.SystemContainerdAddress)
	if err != nil {
		return err
	}

	var img containerd.Image

	img, err = client.GetImage(ctx, ref)
	if err != nil {
		if errdefs.IsNotFound(err) && options.Pull {
			log.Printf("pulling %q", ref)

			img, err = image.Pull(ctx, reg, client, ref)
		}
	}

	if err != nil {
		return err
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
	}

	// TODO(andrewrynhard): To handle cases when the newer version changes the
	// platform name, this should be determined in the installer container.
	var config *string
	if config = procfs.ProcCmdline().Get(constants.KernelParamConfig).First(); config == nil {
		return fmt.Errorf("no config option was found")
	}

	upgrade := strconv.FormatBool(options.Upgrade)
	force := strconv.FormatBool(options.Force)
	zero := strconv.FormatBool(options.Zero)

	args := []string{
		"/bin/installer",
		"install",
		"--disk=" + disk,
		"--platform=" + platform,
		"--config=" + *config,
		"--upgrade=" + upgrade,
		"--force=" + force,
		"--zero=" + zero,
	}

	for _, arg := range options.ExtraKernelArgs {
		args = append(args, []string{"--extra-kernel-arg", arg}...)
	}

	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(img),
		oci.WithProcessArgs(args...),
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithHostNamespace(specs.PIDNamespace),
		oci.WithMounts(mounts),
		oci.WithHostHostsFile,
		oci.WithHostResolvconf,
		oci.WithParentCgroupDevices,
		oci.WithPrivileged,
		oci.WithAllDevicesAllowed,
	}

	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(img),
		containerd.WithNewSnapshot("upgrade", img),
		containerd.WithNewSpec(specOpts...),
	}

	container, err := client.NewContainer(ctx, "upgrade", containerOpts...)
	if err != nil {
		return err
	}

	f, err := os.OpenFile("/dev/kmsg", os.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOCTTY, 0o666)
	if err != nil {
		return fmt.Errorf("failed to open /dev/kmsg: %w", err)
	}
	// nolint: errcheck
	defer f.Close()

	w := &kmsg.Writer{KmsgWriter: f}

	creator := cio.NewCreator(cio.WithStreams(nil, w, w))

	t, err := container.NewTask(ctx, creator)
	if err != nil {
		return err
	}

	if err = t.Start(ctx); err != nil {
		return fmt.Errorf("failed to start %q task: %w", "upgrade", err)
	}

	statusC, err := t.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed waiting for %q task: %w", "upgrade", err)
	}

	status := <-statusC

	code := status.ExitCode()
	if code != 0 {
		return fmt.Errorf("task %q failed: exit code %d", "upgrade", code)
	}

	return nil
}
