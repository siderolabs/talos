// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// Install performs an installation via the installer container.
//
// nolint: gocyclo
func Install(r runtime.Runtime) error {
	ctx := namespaces.WithNamespace(context.Background(), constants.SystemContainerdNamespace)

	client, err := containerd.New(constants.SystemContainerdAddress)
	if err != nil {
		return err
	}

	image, err := client.Pull(ctx, r.Config().Machine().Install().Image(), []containerd.RemoteOpt{containerd.WithPullUnpack}...)
	if err != nil {
		return err
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
	}

	// TODO(andrewrynhard): To handle cases when the newer version changes the
	// platform name, this should be determined in the installer container.
	var config *string
	if config = kernel.ProcCmdline().Get(constants.KernelParamConfig).First(); config == nil {
		return fmt.Errorf("no config option was found")
	}

	upgrade := "false"
	if r.Sequence() == runtime.Upgrade {
		upgrade = "true"
	}

	args := []string{
		"/bin/osctl",
		"install",
		"--disk=" + r.Config().Machine().Install().Disk(),
		"--platform=" + r.Platform().Name(),
		"--config=" + *config,
		"--upgrade=" + upgrade,
	}

	for _, arg := range r.Config().Machine().Install().ExtraKernelArgs() {
		args = append(args, []string{"--extra-kernel-arg", arg}...)
	}

	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithProcessArgs(args...),
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithHostNamespace(specs.PIDNamespace),
		oci.WithMounts(mounts),
		oci.WithHostHostsFile,
		oci.WithHostResolvconf,
		oci.WithParentCgroupDevices,
		oci.WithPrivileged,
	}
	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(image),
		containerd.WithNewSnapshot("upgrade", image),
		containerd.WithNewSpec(specOpts...),
	}

	container, err := client.NewContainer(ctx, "upgrade", containerOpts...)
	if err != nil {
		return err
	}

	t, err := container.NewTask(ctx, cio.LogFile("/dev/kmsg"))
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
