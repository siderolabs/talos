// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/netip"
	"os"
	goruntime "runtime"
	"slices"
	"strings"
	"time"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cap"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"

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
	"github.com/siderolabs/talos/pkg/argsbuilder"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/filetree"
	"github.com/siderolabs/talos/pkg/logging"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	etcdresource "github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	timeresource "github.com/siderolabs/talos/pkg/machinery/resources/time"
)

var _ system.HealthcheckedService = (*Etcd)(nil)

// Etcd implements the Service interface. It serves as the concrete type with
// the required methods.
type Etcd struct {
	Bootstrap            bool
	RecoverFromSnapshot  bool
	RecoverSkipHashCheck bool

	args   []string
	client *etcd.Client

	imgRef string

	// if the new member was added as a learner during the service start, its ID is kept here
	learnerMemberID uint64

	// etcd client URLs advertised by this node, so that they can be excluded from the promotion endpoints:
	// a learner can never promote itself
	selfEndpoints []string

	// etcd client URLs of the voting members as seen at the moment this node joined the cluster
	//
	// these are the URLs etcd itself advertises, so they are preferred over the discovered control plane
	// endpoints, which are node addresses that might not be running etcd at all
	votingMemberEndpoints []string

	promoteCtxCancel context.CancelFunc
}

// ID implements the Service interface.
func (e *Etcd) ID(runtime.Runtime) string {
	return "etcd"
}

// PreFunc implements the Service interface.
//
//nolint:gocyclo
func (e *Etcd) PreFunc(ctx context.Context, r runtime.Runtime) error {
	client, err := containerdapi.New(constants.CRIContainerdAddress)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer client.Close()

	// Pull the image and unpack it.
	containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	spec, err := safe.ReaderGet[*etcdresource.Spec](ctx, r.State().V1Alpha2().Resources(), etcdresource.NewSpec(etcdresource.NamespaceName, etcdresource.SpecID).Metadata())
	if err != nil {
		// spec should be ready
		return fmt.Errorf("failed to get etcd spec: %w", err)
	}

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

	// Clear any state left over from a previous service start.
	e.learnerMemberID = 0
	e.votingMemberEndpoints = nil
	e.selfEndpoints = getEtcdURLs(spec.TypedSpec().AdvertisedAddresses, constants.EtcdClientPort)

	switch t := r.Config().Machine().Type(); t {
	case machine.TypeInit:
		if err = e.argsForInit(ctx, r, spec.TypedSpec()); err != nil {
			return err
		}
	case machine.TypeControlPlane:
		if err = e.argsForControlPlane(ctx, r, spec.TypedSpec()); err != nil {
			return err
		}
	case machine.TypeWorker:
		return fmt.Errorf("unexpected machine type: %v", t)
	case machine.TypeUnknown:
		fallthrough
	default:
		panic(fmt.Sprintf("unexpected machine type %v", t))
	}

	if err = waitPKI(ctx, r); err != nil {
		return fmt.Errorf("failed to generate etcd PKI: %w", err)
	}

	return nil
}

// PostFunc implements the Service interface.
func (e *Etcd) PostFunc(runtime.Runtime, events.ServiceState) (err error) {
	if e.promoteCtxCancel != nil {
		e.promoteCtxCancel()
	}

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
		etcdresource.NewSpecReadyCondition(r.State().V1Alpha2().Resources()),
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
	// Set the process arguments.
	args := runner.Args{
		ID:          e.ID(r),
		ProcessArgs: append([]string{"/usr/local/bin/etcd"}, e.args...),
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

	if e.learnerMemberID != 0 {
		var promoteCtx context.Context

		promoteCtx, e.promoteCtxCancel = context.WithCancel(context.Background())

		go func() {
			if err := promoteMember(promoteCtx, r, e.selfEndpoints, e.votingMemberEndpoints, e.learnerMemberID); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("failed promoting member: %s", err)
			} else if err == nil {
				log.Printf("successfully promoted etcd member")
			}
		}()
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

// HealthFunc implements the HealthcheckedService interface.
func (e *Etcd) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		if e.client == nil {
			var err error

			e.client, err = etcd.NewLocalClient(ctx)
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
		InitialDelay: 5 * time.Second,
		Period:       20 * time.Second,
		Timeout:      15 * time.Second,
	}
}

func waitPKI(ctx context.Context, r runtime.Runtime) error {
	_, err := r.State().V1Alpha2().Resources().WatchFor(
		ctx,
		resource.NewMetadata(etcdresource.NamespaceName, etcdresource.PKIStatusType, etcdresource.PKIID, resource.VersionUndefined),
		state.WithEventTypes(state.Created, state.Updated),
	)

	return err
}

//nolint:gocyclo
func addMember(ctx context.Context, r runtime.Runtime, addrs []string, name string) (*clientv3.MemberListResponse, uint64, error) {
	client, err := etcd.NewClientFromControlPlaneIPs(ctx, r.State().V1Alpha2().Resources())
	if err != nil {
		return nil, 0, err
	}

	//nolint:errcheck
	defer client.Close()

	ctx = clientv3.WithRequireLeader(ctx)

	list, err := client.MemberList(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("error getting etcd member list: %w", err)
	}

	for _, member := range list.Members {
		// addMember only gets called when the etcd data directory is empty, so the node is about to join the etcd cluster
		// if there's already a member with same hostname, it should be removed, as there will be a conflict between the existing
		// member and a new joining member.
		// here we assume that control plane nodes have unique hostnames (if that's not the case, it will be a problem anyways)
		if member.Name == name {
			if _, err = client.MemberRemove(ctx, member.ID); err != nil {
				return nil, 0, fmt.Errorf("error removing self from the member list: %w", err)
			}
		}
	}

	add, err := client.MemberAddAsLearner(ctx, addrs)
	if err != nil {
		if errors.Is(err, rpctypes.ErrPeerURLExist) {
			// member already exists with the same peer URLs, see if it's ourselves as a learner
			// we can't really say for sure, but we try to match the peer URLs, and name should
			// be still empty at this point
			for _, member := range list.Members {
				if slices.Equal(member.PeerURLs, addrs) {
					if member.IsLearner && member.Name == "" {
						return list, member.ID, nil
					}
				}
			}

			return nil, 0, fmt.Errorf("member already exists with the same peer URLs %q, but is not a learner: %w", addrs, err)
		}

		return nil, 0, fmt.Errorf("error adding member: %w", err)
	}

	list, err = client.MemberList(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("error getting second etcd member list: %w", err)
	}

	return list, add.Member.ID, nil
}

// joinResult describes the outcome of joining the etcd cluster as a learner.
type joinResult struct {
	// initialCluster is the value for the etcd --initial-cluster flag.
	initialCluster string
	// learnerMemberID is the ID this node was assigned as a learner.
	learnerMemberID uint64
	// votingMemberEndpoints are the etcd client URLs advertised by the voting members of the cluster.
	votingMemberEndpoints []string
}

//nolint:gocyclo
func buildInitialCluster(ctx context.Context, r runtime.Runtime, name string, peerAddrs []string) (result joinResult, err error) {
	var (
		id      uint64
		lastNag time.Time
	)

	err = retry.Constant(
		constants.EtcdJoinTimeout,
		retry.WithUnits(3*time.Second),
		retry.WithJitter(time.Second),
		retry.WithErrorLogging(true),
	).RetryWithContext(ctx, func(ctx context.Context) error {
		var resp *clientv3.MemberListResponse

		if time.Since(lastNag) > 30*time.Second {
			lastNag = time.Now()

			log.Printf("etcd is waiting to join the cluster, if this node is the first node in the cluster, please run `talosctl bootstrap` against one of the following IPs:")

			// we "allow" a failure here since we want to fallthrough and attempt to add the etcd member regardless of
			// whether we can print our IPs
			currentAddresses, addrErr := safe.ReaderGet[*network.NodeAddress](
				ctx,
				r.State().V1Alpha2().Resources(),
				resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s), resource.VersionUndefined),
			)
			if addrErr != nil {
				log.Printf("error getting node addresses: %s", addrErr.Error())
			} else {
				ips := currentAddresses.TypedSpec().IPs()
				log.Printf("%s", ips)
			}
		}

		attemptCtx, attemptCtxCancel := context.WithTimeout(ctx, 30*time.Second)
		defer attemptCtxCancel()

		resp, id, err = addMember(attemptCtx, r, peerAddrs, name)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}

			// TODO(andrewrynhard): We should check the error type here and
			// handle the specific error accordingly.
			return retry.ExpectedError(err)
		}

		var (
			conf                  []string
			votingMemberEndpoints []string
		)

		for _, memb := range resp.Members {
			for _, u := range memb.PeerURLs {
				n := memb.Name
				if memb.ID == id {
					n = name
				}

				conf = append(conf, fmt.Sprintf("%s=%s", n, u))
			}

			// a learner (including this node) can't serve the promotion call, so only voting members are of interest
			if memb.ID != id && !memb.IsLearner {
				votingMemberEndpoints = append(votingMemberEndpoints, memb.ClientURLs...)
			}
		}

		result = joinResult{
			initialCluster:        strings.Join(conf, ","),
			learnerMemberID:       id,
			votingMemberEndpoints: votingMemberEndpoints,
		}

		return nil
	})
	if err != nil {
		return joinResult{}, fmt.Errorf("failed to build cluster arguments: %w", err)
	}

	return result, nil
}

//nolint:gocyclo
func (e *Etcd) argsForInit(ctx context.Context, r runtime.Runtime, spec *etcdresource.SpecSpec) error {
	var upgraded bool

	_, upgraded = r.State().Machine().Meta().ReadTag(meta.Upgrade)

	denyListArgs := argsbuilder.Args{
		"name":                           {spec.Name},
		"auto-tls":                       {"false"},
		"peer-auto-tls":                  {"false"},
		"data-dir":                       {constants.EtcdDataPath},
		"listen-peer-urls":               {formatEtcdURLs(spec.ListenPeerAddresses, constants.EtcdPeerPort)},
		"listen-client-urls":             {formatEtcdURLs(spec.ListenClientAddresses, constants.EtcdClientPort)},
		"listen-client-http-urls":        {formatEtcdURLs(spec.ListenClientAddresses, constants.EtcdClientHTTPPort)},
		"client-cert-auth":               {"true"},
		"cert-file":                      {constants.EtcdCert},
		"key-file":                       {constants.EtcdKey},
		"trusted-ca-file":                {constants.EtcdCACert},
		"peer-client-cert-auth":          {"true"},
		"peer-cert-file":                 {constants.EtcdPeerCert},
		"peer-key-file":                  {constants.EtcdPeerKey},
		"peer-trusted-ca-file":           {constants.EtcdCACert},
		"feature-gates":                  {"InitialCorruptCheck=true", "CompactHashCheck=true"},
		"watch-progress-notify-interval": {"5s"},
		"tls-min-version":                {"TLS1.3"},
	}

	extraArgs := make(argsbuilder.Args, len(spec.ExtraArgs))
	for k, v := range spec.ExtraArgs {
		extraArgs[k] = v.Values
	}

	denyList := argsbuilder.WithDenyList(denyListArgs)

	if !extraArgs.Contains("initial-cluster-state") {
		denyListArgs.Set("initial-cluster-state", argsbuilder.Value{"new"})
	}

	// If the initial cluster isn't explicitly defined, we need to discover any
	// existing members.
	if !extraArgs.Contains("initial-cluster") {
		ok, err := IsDirEmpty(constants.EtcdDataPath)
		if err != nil {
			return err
		}

		if ok {
			initialCluster := formatClusterURLs(spec.Name, getEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort))

			if upgraded {
				denyListArgs.Set("initial-cluster-state", argsbuilder.Value{"existing"})

				join, joinErr := buildInitialCluster(ctx, r, spec.Name, getEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort))
				if joinErr != nil {
					return joinErr
				}

				initialCluster, e.learnerMemberID, e.votingMemberEndpoints = join.initialCluster, join.learnerMemberID, join.votingMemberEndpoints
			}

			denyListArgs.Set("initial-cluster", argsbuilder.Value{initialCluster})
		} else {
			denyListArgs.Set("initial-cluster-state", argsbuilder.Value{"existing"})
		}
	}

	if !extraArgs.Contains("initial-advertise-peer-urls") {
		denyListArgs.Set(
			"initial-advertise-peer-urls",
			argsbuilder.Value{formatEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort)},
		)
	}

	if !extraArgs.Contains("advertise-client-urls") {
		denyListArgs.Set(
			"advertise-client-urls",
			argsbuilder.Value{formatEtcdURLs(spec.AdvertisedAddresses, constants.EtcdClientPort)},
		)
	}

	if err := denyListArgs.Merge(extraArgs, denyList); err != nil {
		return err
	}

	e.args = denyListArgs.Args()

	return nil
}

//nolint:gocyclo
func (e *Etcd) argsForControlPlane(ctx context.Context, r runtime.Runtime, spec *etcdresource.SpecSpec) error {
	denyListArgs := argsbuilder.Args{
		"name":                           {spec.Name},
		"auto-tls":                       {"false"},
		"peer-auto-tls":                  {"false"},
		"data-dir":                       {constants.EtcdDataPath},
		"listen-peer-urls":               {formatEtcdURLs(spec.ListenPeerAddresses, constants.EtcdPeerPort)},
		"listen-client-urls":             {formatEtcdURLs(spec.ListenClientAddresses, constants.EtcdClientPort)},
		"listen-client-http-urls":        {formatEtcdURLs(spec.ListenClientAddresses, constants.EtcdClientHTTPPort)},
		"client-cert-auth":               {"true"},
		"cert-file":                      {constants.EtcdCert},
		"key-file":                       {constants.EtcdKey},
		"trusted-ca-file":                {constants.EtcdCACert},
		"peer-client-cert-auth":          {"true"},
		"peer-cert-file":                 {constants.EtcdPeerCert},
		"peer-key-file":                  {constants.EtcdPeerKey},
		"peer-trusted-ca-file":           {constants.EtcdCACert},
		"feature-gates":                  {"InitialCorruptCheck=true", "CompactHashCheck=true"},
		"watch-progress-notify-interval": {"5s"},
		"tls-min-version":                {"TLS1.3"},
	}

	extraArgs := make(argsbuilder.Args, len(spec.ExtraArgs))
	for k, v := range spec.ExtraArgs {
		extraArgs[k] = v.Values
	}

	denyList := argsbuilder.WithDenyList(denyListArgs)

	if e.RecoverFromSnapshot {
		if err := e.recoverFromSnapshot(spec); err != nil {
			return err
		}
	}

	ok, err := IsDirEmpty(constants.EtcdDataPath)
	if err != nil {
		return err
	}

	// The only time that we need to build the initial cluster args, is when we
	// don't have any data.
	if ok {
		if !extraArgs.Contains("initial-cluster-state") {
			if e.Bootstrap {
				denyListArgs.Set("initial-cluster-state", argsbuilder.Value{"new"})
			} else {
				denyListArgs.Set("initial-cluster-state", argsbuilder.Value{"existing"})
			}
		}

		if !extraArgs.Contains("initial-cluster") {
			var initialCluster string

			if e.Bootstrap {
				initialCluster = formatClusterURLs(spec.Name, getEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort))
			} else {
				join, joinErr := buildInitialCluster(ctx, r, spec.Name, getEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort))
				if joinErr != nil {
					return fmt.Errorf("failed to build initial etcd cluster: %w", joinErr)
				}

				initialCluster, e.learnerMemberID, e.votingMemberEndpoints = join.initialCluster, join.learnerMemberID, join.votingMemberEndpoints
			}

			denyListArgs.Set("initial-cluster", argsbuilder.Value{initialCluster})
		}
	}

	if !extraArgs.Contains("advertise-client-urls") {
		denyListArgs.Set(
			"advertise-client-urls",
			argsbuilder.Value{formatEtcdURLs(spec.AdvertisedAddresses, constants.EtcdClientPort)},
		)
	}

	if !extraArgs.Contains("initial-advertise-peer-urls") {
		denyListArgs.Set(
			"initial-advertise-peer-urls",
			argsbuilder.Value{formatEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort)},
		)
	}

	if err = denyListArgs.Merge(extraArgs, denyList); err != nil {
		return err
	}

	e.args = denyListArgs.Args()

	return nil
}

// recoverFromSnapshot recovers etcd data directory from the snapshot uploaded previously.
func (e *Etcd) recoverFromSnapshot(spec *etcdresource.SpecSpec) error {
	manager := snapshot.NewV3(logging.Wrap(log.Writer()))

	status, err := manager.Status(constants.EtcdRecoverySnapshotPath)
	if err != nil {
		return fmt.Errorf("error verifying snapshot: %w", err)
	}

	log.Printf("recovering etcd from snapshot: hash %08x, revision %d, total keys %d, total size %d\n",
		status.Hash, status.Revision, status.TotalKey, status.TotalSize)

	if err = manager.Restore(snapshot.RestoreConfig{
		SnapshotPath: constants.EtcdRecoverySnapshotPath,

		Name:          spec.Name,
		OutputDataDir: constants.EtcdDataPath,

		PeerURLs: getEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort),

		InitialCluster: formatClusterURLs(spec.Name, getEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort)),

		SkipHashCheck: e.RecoverSkipHashCheck,
	}); err != nil {
		return fmt.Errorf("error recovering from the snapshot: %w", err)
	}

	if err = os.Remove(constants.EtcdRecoverySnapshotPath); err != nil {
		return fmt.Errorf("error deleting snapshot: %w", err)
	}

	return filetree.ChownRecursive(constants.EtcdDataPath, constants.EtcdUserID, constants.EtcdUserID)
}

// promoteEndpointTimeout bounds a single promotion call against a single endpoint.
//
// Client.PromoteMember reports an endpoint which refuses connections almost immediately, so this
// timeout only bounds endpoints which accept the connection but never answer (e.g. a blackholed
// address or a wedged member).
const promoteEndpointTimeout = 5 * time.Second

// promoteMember promotes this node from a learner to a voting member of the etcd cluster.
//
// The call is retried until it succeeds, as it fails while the learner is still catching up with
// the leader. Each attempt walks the full list of candidate endpoints: the promotion has to be
// served by a voting member, so the endpoint of the member being promoted (which is always part of
// the discovered control plane endpoints) can never be the one which works.
//
// votingMemberEndpoints are the client URLs etcd itself advertises for the voting members, captured
// when this node joined; they are tried first, as the discovered control plane endpoints are node
// addresses which might not be running etcd at all (e.g. the Kubernetes control plane endpoint, or a
// node address outside of the configured etcd listen subnets).
func promoteMember(ctx context.Context, r runtime.Runtime, selfEndpoints, votingMemberEndpoints []string, memberID uint64) error {
	self := xslices.ToSetFunc(selfEndpoints, normalizeEtcdEndpoint)

	// The attempt timeout is only a backstop: an attempt normally ends once every endpoint has been
	// tried, each bounded by promoteEndpointTimeout. It is deliberately large enough to walk a
	// realistic endpoint list, so that the endpoints at the tail are not starved.
	return retry.Constant(
		10*time.Minute,
		retry.WithUnits(2*time.Second),
		retry.WithAttemptTimeout(time.Minute),
		retry.WithJitter(time.Second),
		retry.WithErrorLogging(true),
	).RetryWithContext(ctx, func(ctx context.Context) error {
		discoveredEndpoints, err := etcd.GetEndpoints(ctx, r.State().V1Alpha2().Resources())
		if err != nil {
			return retry.ExpectedError(err)
		}

		endpoints := promotionEndpoints(self, votingMemberEndpoints, discoveredEndpoints)

		if len(endpoints) == 0 {
			return retry.ExpectedErrorf("no endpoints")
		}

		errs := make([]error, 0, len(endpoints))

		for _, endpoint := range endpoints {
			select {
			case <-ctx.Done():
				return retry.ExpectedError(ctx.Err())
			default:
			}

			if err = attemptPromote(ctx, endpoint, memberID); err == nil {
				return nil
			}

			errs = append(errs, fmt.Errorf("%s: %w", endpoint, err))
		}

		return retry.ExpectedError(errors.Join(errs...))
	})
}

func attemptPromote(ctx context.Context, endpoint string, memberID uint64) error {
	ctx, cancel := context.WithTimeout(ctx, promoteEndpointTimeout)
	defer cancel()

	client, err := etcd.NewClient(ctx, []string{endpoint})
	if err != nil {
		return err
	}

	defer client.Close() //nolint:errcheck

	return client.PromoteMember(ctx, memberID)
}

// promotionEndpoints builds the ordered list of endpoints to try the promotion call against.
//
// The endpoints etcd advertises for the voting members come first, followed by the discovered
// control plane endpoints; endpoints belonging to this node are dropped, as a learner can't promote
// itself, and duplicates across the two sources are collapsed.
func promotionEndpoints(self map[string]struct{}, votingMemberEndpoints, discoveredEndpoints []string) []string {
	total := len(votingMemberEndpoints) + len(discoveredEndpoints)

	endpoints := make([]string, 0, total)
	seen := make(map[string]struct{}, total)

	for _, endpoint := range slices.Concat(votingMemberEndpoints, discoveredEndpoints) {
		endpoint = normalizeEtcdEndpoint(endpoint)

		if _, ok := self[endpoint]; ok {
			continue
		}

		if _, ok := seen[endpoint]; ok {
			continue
		}

		seen[endpoint] = struct{}{}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

// normalizeEtcdEndpoint strips the scheme off an etcd endpoint, so that endpoints coming from
// different sources (etcd member client URLs vs. discovered control plane addresses) can be
// compared and de-duplicated.
func normalizeEtcdEndpoint(endpoint string) string {
	if _, hostPort, ok := strings.Cut(endpoint, "://"); ok {
		return hostPort
	}

	return endpoint
}

// IsDirEmpty checks if a directory is empty or not.
func IsDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	//nolint:errcheck
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}

	return false, err
}

// BootstrapEtcd bootstraps the etcd cluster.
//
// Current instance of etcd (not joined yet) is stopped, and new instance is started in bootstrap mode.
func BootstrapEtcd(ctx context.Context, r runtime.Runtime, req *machineapi.BootstrapRequest) error {
	// Reject bootstrap if an unattended install is in progress.
	if status, err := safe.ReaderGetByID[*runtimeres.UnattendedInstallStatus](
		ctx, r.State().V1Alpha2().Resources(), runtimeres.UnattendedInstallStatusID,
	); err == nil && status.TypedSpec().Phase != runtimeres.UnattendedInstallPhaseInstalled {
		return fmt.Errorf("bootstrap is not allowed during unattended install (phase: %s)", status.TypedSpec().Phase)
	}

	if err := system.Services(r).Stop(ctx, "etcd"); err != nil {
		return fmt.Errorf("failed to stop etcd: %w", err)
	}

	// This is hack. We need to fake a finished state so that we can get the
	// wait in the boot sequence to unblock.
	for _, svc := range system.Services(r).List() {
		if svc.AsProto().GetId() == "etcd" {
			svc.UpdateState(ctx, events.StateFinished, "Bootstrap requested")

			break
		}
	}

	if entries, _ := os.ReadDir(constants.EtcdDataPath); len(entries) > 0 { //nolint:errcheck
		return errors.New("etcd data directory is not empty")
	}

	svc := &Etcd{
		Bootstrap:            true,
		RecoverFromSnapshot:  req.RecoverEtcd,
		RecoverSkipHashCheck: req.RecoverSkipHashCheck,
	}

	if err := system.Services(r).Unload(ctx, svc.ID(r)); err != nil {
		return err
	}

	system.Services(r).Load(svc)

	if err := system.Services(r).Start(svc.ID(r)); err != nil {
		return fmt.Errorf("error starting etcd in bootstrap mode: %w", err)
	}

	return nil
}

func formatEtcdURL(addr netip.Addr, port int) string {
	return fmt.Sprintf("https://%s", nethelpers.JoinHostPort(addr.String(), port))
}

func getEtcdURLs(addrs []netip.Addr, port int) []string {
	return xslices.Map(addrs, func(addr netip.Addr) string {
		return formatEtcdURL(addr, port)
	})
}

func formatEtcdURLs(addrs []netip.Addr, port int) string {
	return strings.Join(getEtcdURLs(addrs, port), ",")
}

func formatClusterURLs(name string, urls []string) string {
	return strings.Join(xslices.Map(urls, func(url string) string {
		return fmt.Sprintf("%s=%s", name, url)
	}), ",")
}
