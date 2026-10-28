// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	containerdapi "github.com/containerd/containerd/v2/client"
	ctrdcontainers "github.com/containerd/containerd/v2/core/containers"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	containersadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/containers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	containerdrunner "github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/internal/pkg/capability"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	containersres "github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

const (
	// gracefulShutdownTimeout is how long a container gets after SIGTERM before SIGKILL.
	//
	// Internal and not configurable, matching the containerd service runner's existing default.
	gracefulShutdownTimeout = 10 * time.Second

	// oomScoreAdj matches what extension services get. Containers must be killed before apid and
	// trustd, which sit at -998, so that the API stays reachable on a node under memory pressure.
	oomScoreAdj = -600
)

// containerdRunner runs containers against the CRI containerd instance.
type containerdRunner struct {
	client  *containerdapi.Client
	logging runtime.LoggingManager
}

func newContainerdRunner(logging runtime.LoggingManager) (TaskRunner, error) {
	client, err := containerdapi.New(constants.CRIContainerdAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}

	return &containerdRunner{
		client:  client,
		logging: logging,
	}, nil
}

// withNamespace scopes a context to the dedicated namespace.
func withNamespace(ctx context.Context) context.Context {
	return namespaces.WithNamespace(ctx, constants.TalosContainersContainerdNamespace)
}

// List implements TaskRunner interface.
func (r *containerdRunner) List(ctx context.Context) ([]string, error) {
	list, err := r.client.Containers(withNamespace(ctx))
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(list))

	for _, container := range list {
		ids = append(ids, container.ID())
	}

	return ids, nil
}

// Remove implements TaskRunner interface.
func (r *containerdRunner) Remove(ctx context.Context, id string) error {
	ctx = withNamespace(ctx)

	container, err := r.client.LoadContainer(ctx, id)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return r.removeSnapshot(ctx, id)
		}

		return fmt.Errorf("failed to load container: %w", err)
	}

	// Kill any task first: a container with a live task cannot be deleted.
	if task, taskErr := container.Task(ctx, nil); taskErr == nil {
		if _, delErr := task.Delete(ctx, containerdapi.WithProcessKill); delErr != nil && !errdefs.IsNotFound(delErr) {
			return fmt.Errorf("failed to delete task: %w", delErr)
		}
	}

	if err := container.Delete(ctx, containerdapi.WithSnapshotCleanup); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("failed to delete container: %w", err)
	}

	return r.removeSnapshot(ctx, id)
}

// Run implements TaskRunner interface.
func (r *containerdRunner) Run(
	ctx context.Context,
	id string,
	spec containersres.ContainerInstanceSpecSpec,
	started func(pid uint32),
) (int32, error) {
	image, err := r.client.GetImage(withNamespace(ctx), spec.Image)
	if err != nil {
		return 0, fmt.Errorf("failed to get image %q: %w", spec.Image, err)
	}

	cgroupPath := filepath.Join(constants.CgroupTalosContainersRoot, spec.ContainerID)

	// The mount alone does not make the socket usable: only the machined-access domain is permitted to
	// connect to it, so a container that asked for the socket has to be labeled for it as well.
	selinuxLabel := constants.SelinuxLabelTalosContainer
	if spec.Security.MachinedAccess {
		selinuxLabel = constants.SelinuxLabelTalosContainerMachined
	}

	svc := containerdrunner.NewRunner(false,
		&runner.Args{ID: id},
		runner.WithNamespace(constants.TalosContainersContainerdNamespace),
		runner.WithContainerdAddress(constants.CRIContainerdAddress),
		runner.WithContainerImage(spec.Image),
		runner.WithEnv(spec.Environment),
		runner.WithLoggingManager(r.logging),
		// Keyed by container, not by instance: successive generations append to one buffer, so restart
		// history reads as a single continuous log.
		runner.WithLogID(constants.TalosContainersLogPrefix+spec.ContainerID),
		runner.WithCgroupPath(cgroupPath),
		runner.WithCgroupResources(containersadapter.ContainerResourcesSpec(&spec.Resources).CgroupResources()),
		runner.WithOOMScoreAdj(oomScoreAdj),
		runner.WithGracefulShutdownTimeout(gracefulShutdownTimeout),
		runner.WithHostNetworkFiles(spec.Network.HostNetwork),
		runner.WithSelinuxLabel(selinuxLabel),
		runner.WithOCISpecOpts(r.ociSpecOpts(spec, image)...),
	)

	if err := svc.Open(); err != nil {
		return 0, fmt.Errorf("failed to create container %q: %w", id, err)
	}

	defer svc.Close() //nolint:errcheck

	status, err := svc.Run(ctx, events.NullRecorder, func(pid int32) {
		started(uint32(pid)) //nolint:gosec
	})

	return int32(status.ExitCode), err //nolint:gosec
}

// Close implements TaskRunner interface.
func (r *containerdRunner) Close() error {
	return r.client.Close()
}

// removeSnapshot clears a snapshot left behind without its container.
func (r *containerdRunner) removeSnapshot(ctx context.Context, id string) error {
	if err := r.client.SnapshotService("").Remove(ctx, id); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("failed to remove snapshot: %w", err)
	}

	return nil
}

// ociSpecOpts builds the OCI spec for a container.
func (r *containerdRunner) ociSpecOpts(spec containersres.ContainerInstanceSpecSpec, image containerdapi.Image) []oci.SpecOpts {
	// No-new-privileges, the cgroup, the OOM score, seccomp and the SELinux label are the shared
	// runner's, applied around these; what is left here is the translation of the container's own
	// declared spec.
	opts := []oci.SpecOpts{
		containerdrunner.WithImageConfigStripped(image),
		WithImageUser(image, spec.RunAs),
		// Always applied: it is what resolves each half of the argv against the image, so leaving it
		// out when neither is overridden would run a container with no argv at all.
		WithProcessArgs(spec, image),
	}

	if spec.WorkingDir != "" {
		opts = append(opts, oci.WithProcessCwd(spec.WorkingDir))
	}

	if spec.RunAs.UID != nil || spec.RunAs.GID != nil {
		opts = append(opts, WithRunAs(spec.RunAs))
	}

	if spec.Network.HostNetwork {
		opts = append(opts, oci.WithHostNamespace(specs.NetworkNamespace))
	}

	if mounts := MountsResolvedToOCI(spec.Mounts); len(mounts) > 0 {
		opts = append(opts, oci.WithMounts(mounts))
	}

	if len(spec.Devices) > 0 {
		opts = append(opts, WithResolvedDevices(spec.Devices))
	}

	// An rshared bind can only join the host peer group when the container's
	// rootfs propagation is shared: with the runc default (recursive slave),
	// the bind lands in a new peer group slaved to the host, and mounts created
	// inside the container never propagate back out.
	if MountsRequestSharedPropagation(spec.Mounts) {
		opts = append(opts, containerdrunner.WithRootfsPropagation("shared"))
	}

	if spec.Security.MachinedAccess {
		opts = append(opts, func(_ context.Context, _ oci.Client, _ *ctrdcontainers.Container, s *specs.Spec) error {
			if _, err := os.Stat(constants.MachineSocketPath); err != nil {
				return fmt.Errorf("machined socket %q not available for machinedAccess: %w", constants.MachineSocketPath, err)
			}

			s.Mounts = append(s.Mounts, specs.Mount{
				Type:        "bind",
				Destination: constants.MachineSocketPath,
				Source:      constants.MachineSocketPath,
				Options:     []string{"rbind", "ro"},
			})

			return nil
		})
	}

	opts = append(opts, containersadapter.ContainerSecuritySpec(&spec.Security).OCISpecOpts(capability.AllGrantableCapabilities())...)

	return opts
}

// WithImageUser applies the numeric halves of the image's USER.
//
// A half expressed as a name cannot be resolved without the image's /etc/passwd, so it has to be
// covered by runAs (applied later by WithRunAs); otherwise the container is rejected rather than
// silently run as root. The image controller rejects the same images right after the pull; this is
// the backstop for the runner.
func WithImageUser(image containerdapi.Image, runAs containersres.ContainerRunAsSpec) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, _ *ctrdcontainers.Container, s *specs.Spec) error {
		config, err := ImageConfig(ctx, image)
		if err != nil {
			return err
		}

		if err := CheckImageUser(config.User, runAs); err != nil {
			return err
		}

		if config.User == "" {
			return nil
		}

		if s.Process == nil {
			s.Process = &specs.Process{}
		}

		uidPart, gidPart, _ := strings.Cut(config.User, ":")

		if uid, ok := parseID(uidPart); ok {
			s.Process.User.UID = uid
		}

		if gid, ok := parseID(gidPart); ok {
			s.Process.User.GID = gid
		}

		return nil
	}
}

// CheckImageUser rejects an image USER whose non-numeric halves are not overridden by runAs.
//
// A name can only be resolved through the image's /etc/passwd and /etc/group, which Talos does not
// read. A bare name ("app") also leaves the gid unknown, since it would be the user's primary group.
func CheckImageUser(user string, runAs containersres.ContainerRunAsSpec) error {
	if user == "" {
		return nil
	}

	uidPart, gidPart, hasGID := strings.Cut(user, ":")
	_, uidNumeric := parseID(uidPart)
	_, gidNumeric := parseID(gidPart)

	var missing []string

	if !uidNumeric && runAs.UID == nil {
		missing = append(missing, "runAs.uid")
	}

	gidUnresolved := (hasGID && !gidNumeric) || (!hasGID && !uidNumeric)
	if gidUnresolved && runAs.GID == nil {
		missing = append(missing, "runAs.gid")
	}

	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf(
		"image USER %q is not numeric and cannot be resolved without the image's /etc/passwd; set %s in the ContainerConfig document",
		user, strings.Join(missing, " and "),
	)
}

// parseID parses a single numeric uid or gid.
func parseID(s string) (uint32, bool) {
	id, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, false
	}

	return uint32(id), true
}

// ImageConfig reads and decodes the image's OCI config.
func ImageConfig(ctx context.Context, image containerdapi.Image) (v1.ImageConfig, error) {
	descriptor, err := image.Config(ctx)
	if err != nil {
		return v1.ImageConfig{}, err
	}

	if !images.IsConfigType(descriptor.MediaType) {
		return v1.ImageConfig{}, fmt.Errorf("unknown image config media type %s", descriptor.MediaType)
	}

	configBytes, err := content.ReadBlob(ctx, image.ContentStore(), descriptor)
	if err != nil {
		return v1.ImageConfig{}, err
	}

	var ociImage v1.Image

	if err := json.Unmarshal(configBytes, &ociImage); err != nil {
		return v1.ImageConfig{}, err
	}

	return ociImage.Config, nil
}

// WithProcessArgs applies the entrypoint and args overrides, over what the image defines.
func WithProcessArgs(containerInstanceSpec containersres.ContainerInstanceSpecSpec, image containerdapi.Image) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, _ *ctrdcontainers.Container, s *specs.Spec) error {
		config, err := ImageConfig(ctx, image)
		if err != nil {
			return err
		}

		entrypoint := containerInstanceSpec.Entrypoint
		if len(entrypoint) == 0 {
			entrypoint = config.Entrypoint
		}

		args := containerInstanceSpec.Args
		if len(args) == 0 {
			args = config.Cmd
		}

		argv := slices.Concat(entrypoint, args)
		if len(argv) == 0 {
			return errors.New("nothing to run: no entrypoint or args configured and the image declares neither ENTRYPOINT nor CMD")
		}

		if s.Process == nil {
			s.Process = &specs.Process{}
		}

		s.Process.Args = argv

		return nil
	}
}

// WithRunAs applies the configured uid and gid, leaving whichever half is unset at what the image's
// USER resolved to.
func WithRunAs(runAs containersres.ContainerRunAsSpec) oci.SpecOpts {
	return func(_ context.Context, _ oci.Client, _ *ctrdcontainers.Container, s *specs.Spec) error {
		if s.Process == nil {
			s.Process = &specs.Process{}
		}

		if runAs.UID != nil {
			s.Process.User.UID = uint32(*runAs.UID) //nolint:gosec
		}

		if runAs.GID != nil {
			s.Process.User.GID = uint32(*runAs.GID) //nolint:gosec
		}

		return nil
	}
}

// MountsResolvedToOCI converts resolved mounts into OCI mounts.
func MountsResolvedToOCI(mounts []containersres.ResolvedMountSpec) []specs.Mount {
	if len(mounts) == 0 {
		return nil
	}

	out := make([]specs.Mount, 0, len(mounts))

	for _, mount := range mounts {
		switch mount.Kind {
		case containersres.MountKindTmpfs:
			options := append([]string{"nosuid", "nodev"}, mount.Options...)

			if mount.Size > 0 {
				options = append(options, fmt.Sprintf("size=%d", mount.Size))
			}

			out = append(out, specs.Mount{
				Type:        "tmpfs",
				Source:      "tmpfs",
				Destination: mount.Destination,
				Options:     options,
			})
		// A user volume is a bind of the path the volume is mounted at: by the time a mount reaches
		// here, MountController has resolved its source to that path, so the two kinds are the same
		// operation and differ only in who decided the source.
		case containersres.MountKindHostPath, containersres.MountKindVolume:
			out = append(out, specs.Mount{
				Type:        "bind",
				Source:      mount.Source,
				Destination: mount.Destination,
				Options:     append([]string{"rbind"}, mount.Options...),
			})
		}
	}

	return out
}

// WithResolvedDevices adds a device node and a device cgroup rule allowing it for each resolved device.
func WithResolvedDevices(devices []containersres.ResolvedDeviceSpec) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *ctrdcontainers.Container, s *specs.Spec) error {
		for _, device := range devices {
			if err := oci.WithDevices(device.Source, device.Destination, "rw")(ctx, client, c, s); err != nil {
				return fmt.Errorf("failed to add device %q: %w", device.Destination, err)
			}
		}

		return nil
	}
}

// MountsRequestSharedPropagation reports whether any declared mount asks for
// rshared propagation, which requires the rootfs itself to be shared
// for the bind to join the host peer group (see ociSpecOpts).
func MountsRequestSharedPropagation(mounts []containersres.ResolvedMountSpec) bool {
	for _, mount := range mounts {
		if slices.Contains(mount.Options, "rshared") {
			return true
		}
	}

	return false
}
