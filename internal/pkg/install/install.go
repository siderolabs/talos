/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package install

import (
	"context"
	"log"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/constants"
)

// Install performs an installation via the installer container.
func Install(ref string, disk string, platform string) error {
	ctx := namespaces.WithNamespace(context.Background(), constants.SystemContainerdNamespace)
	client, err := containerd.New(constants.SystemContainerdAddress)
	if err != nil {
		return err
	}
	log.Printf("running install via %q", ref)
	image, err := client.Pull(ctx, ref, []containerd.RemoteOpt{containerd.WithPullUnpack}...)
	if err != nil {
		return err
	}
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/dev", Source: "/dev", Options: []string{"rbind", "rshared", "rw"}},
	}

	// TODO(andrewrynhard): To handle cases when the newer version changes the
	// platform name, this should be determined in the installer container.
	var userdata *string
	if userdata = kernel.ProcCmdline().Get(constants.KernelParamUserData).First(); userdata == nil {
		return errors.Errorf("no user data option was found")
	}

	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithProcessArgs([]string{"/bin/entrypoint.sh", "install", "-d", disk, "-p", platform, "-u", *userdata}...),
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
		return errors.Wrapf(err, "failed to start task: %q", "upgrade")
	}
	statusC, err := t.Wait(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed waiting for task: %q", "upgrade")
	}

	status := <-statusC
	code := status.ExitCode()
	if code != 0 {
		return errors.Errorf("task %q failed: exit code %d", "upgrade", code)
	}

	return nil
}
