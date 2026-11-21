// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"fmt"
	goruntime "runtime"
	"slices"
	"time"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cap"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/internal/pkg/containers/image/console"
	"github.com/siderolabs/talos/internal/pkg/environment"
	"github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	etcdresource "github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	timeresource "github.com/siderolabs/talos/pkg/machinery/resources/time"
)

var _ system.HealthcheckedService = (*Etcd)(nil)

// Etcd implements the Service interface. It serves as the concrete type with
// the required methods.
//
// The service is fully managed by the etcd controllers: it is started and stopped by the etcd.ServiceController,
// which also prepares the etcd.ServiceSpec resource the service is running with. The cluster membership
// (bootstrapping, joining, leaving the cluster) is handled by the etcd.MembershipController before the service is started.
type Etcd struct {
	client *etcd.Client

	imgRef string
}

// ID implements the Service interface.
func (e *Etcd) ID(runtime.Runtime) string {
	return "etcd"
}

// PreFunc implements the Service interface.
func (e *Etcd) PreFunc(ctx context.Context, r runtime.Runtime) error {
	spec, err := safe.ReaderGetByID[*etcdresource.ServiceSpec](ctx, r.State().V1Alpha2().Resources(), etcdresource.ServiceSpecID)
	if err != nil {
		// spec should be ready
		return fmt.Errorf("failed to get etcd service spec: %w", err)
	}

	client, err := containerdapi.New(constants.CRIContainerdAddress)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer client.Close()

	// Pull the image and unpack it.
	containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	img, err := image.PullWithRetriesAndTimeout(
		containerdctx,
		cri.RegistryBuilder(r.State().V1Alpha2().Resources()),
		r.State().V1Alpha2().Resources(),
		client, spec.TypedSpec().Image,
		image.WithSkipIfAlreadyPulled(),
		image.WithProgressReporter(console.NewProgressReporter),
	)
	if err != nil {
		return fmt.Errorf("failed to pull image %q: %w", spec.TypedSpec().Image, err)
	}

	e.imgRef = img.Target().Digest.String()

	return nil
}

// PostFunc implements the Service interface.
func (e *Etcd) PostFunc(runtime.Runtime, events.ServiceState) (err error) {
	if e.client != nil {
		e.client.Close() //nolint:errcheck
	}

	e.client = nil

	return nil
}

// Condition implements the Service interface.
func (e *Etcd) Condition(r runtime.Runtime) conditions.Condition {
	return conditions.WaitForAll(
		timeresource.NewSyncCondition(r.State().V1Alpha2().Resources()),
		network.NewReadyCondition(r.State().V1Alpha2().Resources(), network.AddressReady, network.HostnameReady, network.EtcFilesReady),
	)
}

// DependsOn implements the Service interface.
func (e *Etcd) DependsOn(runtime.Runtime) []string {
	return []string{"cri"}
}

// Volumes implements the Service interface.
func (e *Etcd) Volumes(runtime.Runtime) []string {
	return []string{
		"/var/lib",
		constants.EtcdDataVolumeID,
	}
}

// Runner implements the Service interface.
func (e *Etcd) Runner(r runtime.Runtime) (runner.Runner, error) {
	spec, err := safe.ReaderGetByID[*etcdresource.ServiceSpec](context.Background(), r.State().V1Alpha2().Resources(), etcdresource.ServiceSpecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get etcd service spec: %w", err)
	}

	// Set the process arguments.
	args := runner.Args{
		ID:          e.ID(r),
		ProcessArgs: append([]string{"/usr/local/bin/etcd"}, spec.TypedSpec().Args...),
	}

	mounts := []specs.Mount{
		{Type: "bind", Destination: constants.EtcdPKIPath, Source: constants.EtcdPKIPath, Options: []string{"rbind", "ro"}},
		{Type: "bind", Destination: constants.EtcdDataPath, Source: constants.EtcdDataPath, Options: []string{"rbind", "rw"}},
	}

	env := environment.Get(r.Config())

	// NOTE: leave it here for future unsupported architectures, so we can know where to add them
	if slices.Contains([]string{}, goruntime.GOARCH) {
		env = append(env, "ETCD_UNSUPPORTED_ARCH="+goruntime.GOARCH)
	}

	return restart.New(
		containerd.NewRunner(
			r.Config().Debug(),
			&args,
			runner.WithLoggingManager(r.Logging()),
			runner.WithNamespace(constants.SystemContainerdNamespace),
			runner.WithContainerImage(e.imgRef),
			runner.WithEnv(env),
			runner.WithCgroupPath(constants.CgroupEtcd),
			runner.WithSelinuxLabel(constants.SELinuxLabelEtcd),
			runner.WithOCISpecOpts(
				oci.WithDroppedCapabilities(cap.Known()),
				oci.WithHostNamespace(specs.NetworkNamespace),
				oci.WithMounts(mounts),
				oci.WithUIDGID(constants.EtcdUserID, constants.EtcdUserID),
				oci.WithRlimit(&specs.POSIXRlimit{
					Type: "RLIMIT_NOFILE",
					Hard: uint64(10240),
					Soft: uint64(10240),
				}),
			),
			runner.WithOOMScoreAdj(-998),
		),
		restart.WithType(restart.Forever),
	), nil
}

// healthCheckConnectParams are the gRPC connection parameters for the health check etcd client.
//
// The client is reused across health checks, and etcd is probed early and often during startup,
// so the default gRPC reconnect backoff (1s base, capped at 2 minutes) would grow with every refused
// connection before etcd opens its client listener, delaying the detection of a healthy etcd.
//
// MinConnectTimeout is set explicitly, as otherwise the connection attempt deadline is limited to
// the (short) backoff delay, while etcd holds the connection until it is ready to serve (e.g. has quorum).
var healthCheckConnectParams = grpc.ConnectParams{
	Backoff: backoff.Config{
		BaseDelay:  100 * time.Millisecond,
		Multiplier: 1.6,
		Jitter:     0.2,
		MaxDelay:   2 * time.Second,
	},
	MinConnectTimeout: 20 * time.Second,
}

// HealthFunc implements the HealthcheckedService interface.
func (e *Etcd) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		if e.client == nil {
			var err error

			e.client, err = etcd.NewLocalClient(ctx, grpc.WithConnectParams(healthCheckConnectParams))
			if err != nil {
				return err
			}
		}

		return e.client.ValidateQuorum(ctx)
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (e *Etcd) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.Settings{
		InitialDelay: 100 * time.Millisecond,
		Period:       20 * time.Second,
		Timeout:      15 * time.Second,

		// etcd might take a while to reach quorum (e.g. when the whole cluster is restarting),
		// so probe silently and report the first success right away
		StartupWindow:  30 * time.Second,
		StartupPeriod:  time.Second,
		StartupTimeout: time.Second,
	}
}
