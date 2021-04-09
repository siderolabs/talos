// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"bytes"
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
	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	containerdrunner "github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/kmsg"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// RunInstallerContainer performs an installation via the installer container.
//
//nolint:gocyclo
func RunInstallerContainer(disk, platform, ref string, configBytes []byte, reg config.Registries, opts ...Option) error {
	options := DefaultInstallOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	client, err := containerd.New(constants.SystemContainerdAddress)
	if err != nil {
		return err
	}

	defer client.Close() //nolint:errcheck

	var img containerd.Image

	if !options.Pull {
		img, err = client.GetImage(ctx, ref)
	}

	if img == nil || err != nil && errdefs.IsNotFound(err) {
		log.Printf("pulling %q", ref)

		img, err = image.Pull(ctx, reg, client, ref)
	}

	if err != nil {
		return err
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
	}

	// TODO(andrewrynhard): To handle cases when the newer version changes the
	// platform name, this should be determined in the installer container.
	config := constants.ConfigNone
	if c := procfs.ProcCmdline().Get(constants.KernelParamConfig).First(); c != nil {
		config = *c
	}

	upgrade := strconv.FormatBool(options.Upgrade)
	force := strconv.FormatBool(options.Force)
	zero := strconv.FormatBool(options.Zero)

	args := []string{
		"/bin/installer",
		"install",
		"--disk=" + disk,
		"--platform=" + platform,
		"--config=" + config,
		"--upgrade=" + upgrade,
		"--force=" + force,
		"--zero=" + zero,
	}

	if c := procfs.ProcCmdline().Get(constants.KernelParamBoard).First(); c != nil {
		args = append(args, "--board="+*c)
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

	defer container.Delete(ctx, containerd.WithSnapshotCleanup) //nolint:errcheck

	f, err := os.OpenFile("/dev/kmsg", os.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOCTTY, 0o666)
	if err != nil {
		return fmt.Errorf("failed to open /dev/kmsg: %w", err)
	}
	//nolint:errcheck
	defer f.Close()

	w := &kmsg.Writer{KmsgWriter: f}

	configR := &containerdrunner.StdinCloser{
		Stdin:  bytes.NewReader(configBytes),
		Closer: make(chan struct{}),
	}

	creator := cio.NewCreator(cio.WithStreams(configR, w, w))

	t, err := container.NewTask(ctx, creator)
	if err != nil {
		return err
	}

	go configR.WaitAndClose(ctx, t)

	defer t.Delete(ctx) //nolint:errcheck

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

// OptionsFromUpgradeRequest builds installer options from upgrade request.
func OptionsFromUpgradeRequest(r runtime.Runtime, in *machineapi.UpgradeRequest) []Option {
	return []Option{
		WithPull(false),
		WithUpgrade(true),
		WithForce(!in.GetPreserve()),
		WithExtraKernelArgs(r.Config().Machine().Install().ExtraKernelArgs()),
	}
}
