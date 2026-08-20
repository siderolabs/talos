// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"go.uber.org/zap"

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
	logger  *zap.Logger
}

func newContainerdRunner(logging runtime.LoggingManager, logger *zap.Logger) (TaskRunner, error) {
	client, err := containerdapi.New(constants.CRIContainerdAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to containerd: %w", err)
	}

	return &containerdRunner{
		client:  client,
		logging: logging,
		logger:  logger,
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
		runner.WithCgroupResources(spec.Resources.CgroupResources()),
		runner.WithOOMScoreAdj(oomScoreAdj),
		runner.WithGracefulShutdownTimeout(gracefulShutdownTimeout),
		runner.WithHostNetworkFiles(spec.Network.HostNetwork),
		runner.WithSelinuxLabel(constants.SelinuxLabelTalosContainer),
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
		r.withImageUser(image),
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

	opts = append(opts, spec.Security.OCISpecOpts(capability.AllGrantableCapabilities())...)

	return opts
}

// withImageUser applies the image's USER, as long as it is expressed numerically.
func (r *containerdRunner) withImageUser(image containerdapi.Image) oci.SpecOpts {
	return func(ctx context.Context, _ oci.Client, _ *ctrdcontainers.Container, s *specs.Spec) error {
		config, err := ImageConfig(ctx, image)
		if err != nil {
			return err
		}

		user := config.User
		if user == "" {
			return nil
		}

		uid, gid, numeric := ParseNumericUser(user)
		if !numeric {
			r.logger.Warn(
				"image USER is not numeric and cannot be resolved without the image's /etc/passwd, running as root",
				zap.String("image", image.Name()),
				zap.String("user", user),
			)

			return nil
		}

		if s.Process == nil {
			s.Process = &specs.Process{}
		}

		s.Process.User.UID = uid
		s.Process.User.GID = gid

		return nil
	}
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

// ParseNumericUser parses an image USER of the form "uid" or "uid:gid".
//
// A bare uid leaves the gid at 0.
func ParseNumericUser(user string) (uid, gid uint32, numeric bool) {
	uidPart, gidPart, hasGID := strings.Cut(user, ":")

	parsedUID, err := strconv.ParseUint(uidPart, 10, 32)
	if err != nil {
		return 0, 0, false
	}

	if !hasGID {
		return uint32(parsedUID), 0, true
	}

	parsedGID, err := strconv.ParseUint(gidPart, 10, 32)
	if err != nil {
		return 0, 0, false
	}

	return uint32(parsedUID), uint32(parsedGID), true
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
		case containersres.MountKindHostPath:
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
