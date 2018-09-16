package containerd

import (
	"context"
	"fmt"

	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/system/conditions"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/system/runner"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/runtime/restart"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

// Containerd represents a service to be run in a container.
type Containerd struct{}

// WithMemoryLimit sets the linux resource memory limit field.
func WithMemoryLimit(limit int64) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Linux.Resources.Memory = &specs.LinuxMemory{
			Limit: &limit,
			// DisableOOMKiller: &disable,
		}
		return nil
	}
}

// WithRootfsPropagation sets the root filesystem propagation.
func WithRootfsPropagation(rp string) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *containers.Container, s *specs.Spec) error {
		s.Linux.RootfsPropagation = rp
		return nil
	}
}

// Run implements the Runner interface.
// nolint: gocyclo
func (c *Containerd) Run(data *userdata.UserData, args runner.Args, setters ...runner.Option) error {
	//  Wait for the containerd socket.

	_, err := conditions.WaitForFileToExist(constants.ContainerdSocket)()
	if err != nil {
		return err
	}

	// Create the default runner options.

	opts := runner.DefaultOptions()
	for _, setter := range setters {
		setter(opts)
	}

	// Create the containerd client.

	ctx := namespaces.WithNamespace(context.Background(), "system")
	client, err := containerd.New(constants.ContainerdSocket)
	if err != nil {
		return err
	}
	defer client.Close()

	// Pull the image and unpack it.

	image, err := client.Pull(ctx, opts.ContainerImage, containerd.WithPullUnpack)
	if err != nil {
		return fmt.Errorf("failed to pull image %q: %v", opts.ContainerImage, err)
	}

	// Create the container.

	specOpts := newOCISpecOpts(image, args, opts)
	containerOpts := newContainerOpts(image, args, opts, specOpts)
	container, err := client.NewContainer(
		ctx,
		args.ID,
		containerOpts...,
	)
	if err != nil {
		return fmt.Errorf("failed to create container %q: %v", args.ID, err)
	}

	// Create the task and start it.

	task, err := container.NewTask(ctx, cio.LogFile(logPath(args)))
	if err != nil {
		return fmt.Errorf("failed to create task: %q: %v", args.ID, err)
	}
	if err := task.Start(ctx); err != nil {
		return fmt.Errorf("failed to start task: %q: %v", args.ID, err)
	}

	// Wait for the task exit code.

	if opts.Type == runner.Once {
		defer container.Delete(ctx, containerd.WithSnapshotCleanup) // nolint: errcheck
		defer task.Delete(ctx)                                      // nolint: errcheck
		statusC, err := task.Wait(ctx)
		if err != nil {
			return fmt.Errorf("failed waiting for task: %q: %v", args.ID, err)
		}
		status := <-statusC
		code := status.ExitCode()
		if code != 0 {
			return fmt.Errorf("task %q failed: exit code %d", args.ID, code)
		}
	}

	return nil
}

func newContainerOpts(image containerd.Image, args runner.Args, opts *runner.Options, specOpts []oci.SpecOpts) []containerd.NewContainerOpts {
	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(image),
		containerd.WithNewSnapshot(args.ID, image),
		containerd.WithNewSpec(specOpts...),
	}
	switch opts.Type {
	case runner.Forever:
		containerOpts = append(containerOpts, restart.WithStatus(containerd.Running), restart.WithLogPath(logPath(args)))
	}
	containerOpts = append(containerOpts, opts.ContainerOpts...)

	return containerOpts
}

func newOCISpecOpts(image containerd.Image, args runner.Args, opts *runner.Options) []oci.SpecOpts {
	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithProcessArgs(args.ProcessArgs...),
		oci.WithEnv(opts.Env),
		oci.WithHostNamespace(specs.NetworkNamespace),
		oci.WithHostNamespace(specs.PIDNamespace),
		oci.WithHostHostsFile,
		oci.WithHostResolvconf,
		oci.WithPrivileged,
	}
	specOpts = append(specOpts, opts.OCISpecOpts...)

	return specOpts
}

func logPath(args runner.Args) string {
	return "/var/log/" + args.ID + ".log"
}
