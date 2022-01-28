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
	"github.com/talos-systems/go-kmsg"
	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	containerdrunner "github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/pkg/capability"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/extensions"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// RunInstallerContainer performs an installation via the installer container.
//
//nolint:gocyclo,cyclop
func RunInstallerContainer(disk, platform, ref string, cfg config.Provider, opts ...Option) error {
	const containerID = "upgrade"

	options := DefaultInstallOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	configBytes, err := cfg.Bytes()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	client, err := containerd.New(constants.SystemContainerdAddress)
	if err != nil {
		return err
	}

	defer client.Close() //nolint:errcheck

	var done func(context.Context) error

	ctx, done, err = client.WithLease(ctx)
	defer done(ctx) //nolint:errcheck

	var img containerd.Image

	if !options.Pull {
		img, err = client.GetImage(ctx, ref)
	}

	if img == nil || err != nil && errdefs.IsNotFound(err) {
		log.Printf("pulling %q", ref)

		img, err = image.Pull(ctx, cfg.Machine().Registries(), client, ref)
	}

	if err != nil {
		return err
	}

	puller, err := extensions.NewPuller(client)
	if err != nil {
		return err
	}

	if err = puller.PullAndMount(ctx, cfg.Machine().Registries(), cfg.Machine().Install().Extensions()); err != nil {
		return err
	}

	defer func() {
		if err = puller.Cleanup(ctx); err != nil {
			log.Printf("error cleaning up pulled system extensions: %s", err)
		}
	}()

	// See if there's previous container/snapshot to clean up
	var oldcontainer containerd.Container

	if oldcontainer, err = client.LoadContainer(ctx, containerID); err == nil {
		if err = oldcontainer.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
			return fmt.Errorf("error deleting old container instance: %w", err)
		}
	}

	if err = client.SnapshotService("").Remove(ctx, containerID); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("error cleaning up stale snapshot: %w", err)
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: constants.SystemExtensionsPath, Source: constants.SystemExtensionsPath, Options: []string{"rbind", "rshared", "ro"}},
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
		args = append(args, "--extra-kernel-arg", arg)
	}

	if c := procfs.ProcCmdline().Get(constants.KernelParamSideroLink).First(); c != nil {
		args = append(args, "--extra-kernel-arg", fmt.Sprintf("%s=%s", constants.KernelParamSideroLink, *c))
	}

	if c := procfs.ProcCmdline().Get(constants.KernelParamEventsSink).First(); c != nil {
		args = append(args, "--extra-kernel-arg", fmt.Sprintf("%s=%s", constants.KernelParamEventsSink, *c))
	}

	if c := procfs.ProcCmdline().Get(constants.KernelParamLoggingKernel).First(); c != nil {
		args = append(args, "--extra-kernel-arg", fmt.Sprintf("%s=%s", constants.KernelParamLoggingKernel, *c))
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
		oci.WithCapabilities(capability.AllGrantableCapabilities()),
		oci.WithMaskedPaths(nil),
		oci.WithReadonlyPaths(nil),
		oci.WithWriteableSysfs,
		oci.WithWriteableCgroupfs,
		oci.WithSelinuxLabel(""),
		oci.WithApparmorProfile(""),
		oci.WithSeccompUnconfined,
		oci.WithAllDevicesAllowed,
	}

	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(img),
		containerd.WithNewSnapshot(containerID, img),
		containerd.WithNewSpec(specOpts...),
	}

	container, err := client.NewContainer(ctx, containerID, containerOpts...)
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
