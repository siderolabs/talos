// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"sync"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/go-kmsg"
	"github.com/siderolabs/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	containerdrunner "github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/internal/pkg/capability"
	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/internal/pkg/extensions"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	configcore "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// RunInstallerContainer performs an installation via the installer container.
//
//nolint:gocyclo,cyclop
func RunInstallerContainer(
	disk, platform, ref string,
	cfg configcore.Config,
	cfgContainer configcore.Container,
	registryBuilder image.RegistriesBuilder,
	opts ...Option,
) error {
	const containerID = "upgrade"

	options := DefaultInstallOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return err
		}
	}

	var extensionsConfig []config.Extension

	if cfg != nil && cfg.Machine() != nil {
		extensionsConfig = cfg.Machine().Install().Extensions()
	}

	ctx, cancel := context.WithCancel(namespaces.WithNamespace(context.Background(), constants.SystemContainerdNamespace))
	defer cancel()

	client, err := containerd.New(constants.SystemContainerdAddress)
	if err != nil {
		return err
	}

	clientClose := wrapOnErr(client.Close, "failed to close containerd client")

	defer logError(clientClose)

	ctx, done, err := client.WithLease(ctx)
	clientDone := wrapOnErr(func() error { return done(ctx) }, "failed to release containerd lease")

	defer logError(clientDone)

	var img containerd.Image

	if !options.Pull {
		img, err = client.GetImage(ctx, ref)
	}

	if img == nil || err != nil && errdefs.IsNotFound(err) {
		log.Printf("pulling %q", ref)

		img, err = image.Pull(ctx, registryBuilder, client, ref)
		if err == nil {
			log.Printf("pulled %q", ref)
		}
	}

	if err != nil {
		return fmt.Errorf("error pulling %q: %w", ref, err)
	}

	puller, err := extensions.NewPuller(client)
	if err != nil {
		return fmt.Errorf("error creating system extensions puller: %w", err)
	}

	if extensionsConfig != nil {
		if err = puller.PullAndMount(ctx, registryBuilder, extensionsConfig); err != nil {
			return err
		}
	}

	defer func() {
		if err = puller.Cleanup(ctx); err != nil {
			log.Printf("error cleaning up pulled system extensions: %s", err)
		}
	}()

	// See if there's previous container/snapshot to clean up
	oldcontainer, err := client.LoadContainer(ctx, containerID)
	if err == nil {
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

	// mount the machined socket into the container for upgrade pre-checks if the socket exists
	if _, err = os.Stat(constants.MachineSocketPath); err == nil {
		mounts = append(mounts,
			specs.Mount{Type: "bind", Destination: constants.MachineSocketPath, Source: constants.MachineSocketPath, Options: []string{"rbind", "rshared", "ro"}},
		)
	}

	// mount the efivars into the container if the efivars directory exists
	if _, err = os.Stat(constants.EFIVarsMountPoint); err == nil {
		mounts = append(mounts,
			specs.Mount{Type: "efivarfs", Source: "efivarfs", Destination: constants.EFIVarsMountPoint, Options: []string{"rw", "nosuid", "nodev", "noexec", "relatime"}},
		)
	}

	// mount the /.extra directory into the container if the directory exists
	if _, err = os.Stat(constants.SDStubDynamicInitrdPath); err == nil {
		mounts = append(mounts,
			specs.Mount{Type: "bind", Destination: constants.SDStubDynamicInitrdPath, Source: constants.SDStubDynamicInitrdPath, Options: []string{"rbind", "rshared", "ro"}},
		)
	}

	// TODO(andrewrynhard): To handle cases when the newer version changes the
	// platform name, this should be determined in the installer container.
	config := constants.ConfigNone
	if c := procfs.ProcCmdline().Get(constants.KernelParamConfig).First(); c != nil {
		config = *c
	}

	args := []string{
		"/bin/installer",
		"install",
		"--disk=" + disk,
		"--platform=" + platform,
		"--config=" + config,
		"--upgrade=" + strconv.FormatBool(options.Upgrade),
		"--force=" + strconv.FormatBool(options.Force),
		"--zero=" + strconv.FormatBool(options.Zero),
	}

	for _, arg := range options.ExtraKernelArgs {
		args = append(args, "--extra-kernel-arg", arg)
	}

	for _, preservedArg := range []string{
		constants.KernelParamSideroLink,
		constants.KernelParamEventsSink,
		constants.KernelParamLoggingKernel,
		constants.KernelParamEquinixMetalEvents,
		constants.KernelParamAuditdDisabled,
		constants.KernelParamDashboardDisabled,
		constants.KernelParamNetIfnames,
	} {
		if c := procfs.ProcCmdline().Get(preservedArg).First(); c != nil {
			args = append(args, "--extra-kernel-arg", fmt.Sprintf("%s=%s", preservedArg, *c))
		}
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
		oci.WithApparmorProfile(""),
		oci.WithSeccompUnconfined,
		oci.WithAllDevicesAllowed,
		oci.WithEnv(environment.Get(cfg)),
	}

	if selinux.IsEnabled() {
		specOpts = append(specOpts, oci.WithSelinuxLabel(constants.SelinuxLabelInstaller))
	}

	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(img),
		containerd.WithNewSnapshot(containerID, img),
		containerd.WithNewSpec(specOpts...),
	}

	container, err := client.NewContainer(ctx, containerID, containerOpts...)
	if err != nil {
		return fmt.Errorf("failed to create %q container: %w", containerID, err)
	}

	containerClose := wrapOnErr(
		func() error { return container.Delete(ctx, containerd.WithSnapshotCleanup) },
		"failed to delete container",
	)

	defer logError(containerClose)

	f, err := os.OpenFile("/dev/kmsg", os.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOCTTY, 0o666)
	if err != nil {
		return fmt.Errorf("failed to open /dev/kmsg: %w", err)
	}

	fClose := wrapOnErr(f.Close, "failed to close /dev/kmsg")

	defer logError(fClose)

	w := &kmsg.Writer{KmsgWriter: f}

	var r interface {
		io.Reader
		WaitAndClose(context.Context, containerd.Task)
	}

	if cfgContainer != nil {
		var configBytes []byte

		configBytes, err = cfgContainer.Bytes()
		if err != nil {
			return err
		}

		r = &containerdrunner.StdinCloser{
			Stdin:  bytes.NewReader(configBytes),
			Closer: make(chan struct{}),
		}
	}

	creator := cio.NewCreator(cio.WithStreams(r, w, w))

	t, err := container.NewTask(ctx, creator)
	if err != nil {
		return fmt.Errorf("failed to create %q task: %w", containerID, err)
	}

	if r != nil {
		go r.WaitAndClose(ctx, t)
	}

	tDelete := wrapOnErr(
		func() error { return takeErr(t.Delete(ctx)) },
		"failed to delete task",
	)

	defer logError(tDelete)

	if err = t.Start(ctx); err != nil {
		return fmt.Errorf("failed to start %q task: %w", "upgrade", err)
	}

	statusC, err := t.Wait(ctx)
	if err != nil {
		return fmt.Errorf("failed waiting for %q task: %w", "upgrade", err)
	}

	if code := (<-statusC).ExitCode(); code != 0 {
		return fmt.Errorf("task %q failed: exit code %d", "upgrade", code)
	}

	return cmp.Or(tDelete(), fClose(), containerClose(), clientDone(), clientClose())
}

// OptionsFromUpgradeRequest builds installer options from upgrade request.
func OptionsFromUpgradeRequest(r runtime.Runtime, in *machineapi.UpgradeRequest) []Option {
	opts := []Option{
		WithPull(false),
		WithUpgrade(true),
		WithForce(!in.GetPreserve()),
	}

	if r.Config() != nil && r.Config().Machine() != nil {
		opts = append(opts, WithExtraKernelArgs(r.Config().Machine().Install().ExtraKernelArgs()))
	}

	return opts
}

func wrapOnErr(fn func() error, msg string) func() error {
	var once sync.Once

	return func() error {
		var err error

		// Return error only once
		once.Do(func() {
			err = fn()
			if err != nil {
				err = fmt.Errorf("%s: %w", msg, err)
			}
		})

		return err
	}
}

func logError(clientClose func() error) {
	if err := clientClose(); err != nil {
		log.Output(2, err.Error()) //nolint:errcheck
	}
}

func takeErr[T any](_ T, e error) error { return e }
