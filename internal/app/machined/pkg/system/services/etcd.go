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
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/siderolabs/talos/internal/pkg/cgroup"
	"github.com/siderolabs/talos/internal/pkg/containers/image"
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

	img, err := image.Pull(containerdctx, cri.RegistryBuilder(r.State().V1Alpha2().Resources()), client, spec.TypedSpec().Image, image.WithSkipIfAlreadyPulled())
	if err != nil {
		return fmt.Errorf("failed to pull image %q: %w", spec.TypedSpec().Image, err)
	}

	e.imgRef = img.Target().Digest.String()

	// Clear any previously set learner member ID
	e.learnerMemberID = 0

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

	if goruntime.GOARCH == "arm64" {
		env = append(env, "ETCD_UNSUPPORTED_ARCH=arm64")
	}

	env = append(env, "ETCD_CIPHER_SUITES=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305") //nolint:lll

	if e.learnerMemberID != 0 {
		var promoteCtx context.Context

		promoteCtx, e.promoteCtxCancel = context.WithCancel(context.Background())

		go func() {
			if err := promoteMember(promoteCtx, r, e.learnerMemberID); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("failed promoting member: %s", err)
			} else if err == nil {
				log.Printf("successfully promoted etcd member")
			}
		}()
	}

	return restart.New(containerd.NewRunner(
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
			oci.WithUser(fmt.Sprintf("%d:%d", constants.EtcdUserID, constants.EtcdUserID)),
			runner.WithMemoryReservation(constants.CgroupEtcdReservedMemory),
			oci.WithCPUShares(uint64(cgroup.MilliCoresToShares(constants.CgroupEtcdMillicores))),
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
	_, err := r.State().V1Alpha2().Resources().WatchFor(ctx,
		resource.NewMetadata(etcdresource.NamespaceName, etcdresource.PKIStatusType, etcdresource.PKIID, resource.VersionUndefined),
		state.WithEventTypes(state.Created, state.Updated),
	)

	return err
}

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
		return nil, 0, fmt.Errorf("error adding member: %w", err)
	}

	list, err = client.MemberList(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("error getting second etcd member list: %w", err)
	}

	return list, add.Member.ID, nil
}

func buildInitialCluster(ctx context.Context, r runtime.Runtime, name string, peerAddrs []string) (initial string, learnerMemberID uint64, err error) {
	var (
		id      uint64
		lastNag time.Time
	)

	err = retry.Constant(constants.EtcdJoinTimeout,
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

		var conf []string

		for _, memb := range resp.Members {
			for _, u := range memb.PeerURLs {
				n := memb.Name
				if memb.ID == id {
					n = name
				}

				conf = append(conf, fmt.Sprintf("%s=%s", n, u))
			}
		}

		initial = strings.Join(conf, ",")

		return nil
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to build cluster arguments: %w", err)
	}

	return initial, id, nil
}

func (e *Etcd) argsForInit(ctx context.Context, r runtime.Runtime, spec *etcdresource.SpecSpec) error {
	var upgraded bool

	_, upgraded = r.State().Machine().Meta().ReadTag(meta.Upgrade)

	denyListArgs := argsbuilder.Args{
		"name":                               spec.Name,
		"auto-tls":                           "false",
		"peer-auto-tls":                      "false",
		"data-dir":                           constants.EtcdDataPath,
		"listen-peer-urls":                   formatEtcdURLs(spec.ListenPeerAddresses, constants.EtcdPeerPort),
		"listen-client-urls":                 formatEtcdURLs(spec.ListenClientAddresses, constants.EtcdClientPort),
		"client-cert-auth":                   "true",
		"cert-file":                          constants.EtcdCert,
		"key-file":                           constants.EtcdKey,
		"trusted-ca-file":                    constants.EtcdCACert,
		"peer-client-cert-auth":              "true",
		"peer-cert-file":                     constants.EtcdPeerCert,
		"peer-key-file":                      constants.EtcdPeerKey,
		"peer-trusted-ca-file":               constants.EtcdCACert,
		"experimental-initial-corrupt-check": "true",
		"experimental-watch-progress-notify-interval": "5s",
		"experimental-compact-hash-check-enabled":     "true",
	}

	extraArgs := argsbuilder.Args(spec.ExtraArgs)

	denyList := argsbuilder.WithDenyList(denyListArgs)

	if !extraArgs.Contains("initial-cluster-state") {
		denyListArgs.Set("initial-cluster-state", "new")
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
				denyListArgs.Set("initial-cluster-state", "existing")

				initialCluster, e.learnerMemberID, err = buildInitialCluster(ctx, r, spec.Name, getEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort))
				if err != nil {
					return err
				}
			}

			denyListArgs.Set("initial-cluster", initialCluster)
		} else {
			denyListArgs.Set("initial-cluster-state", "existing")
		}
	}

	if !extraArgs.Contains("initial-advertise-peer-urls") {
		denyListArgs.Set("initial-advertise-peer-urls",
			formatEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort),
		)
	}

	if !extraArgs.Contains("advertise-client-urls") {
		denyListArgs.Set("advertise-client-urls",
			formatEtcdURLs(spec.AdvertisedAddresses, constants.EtcdClientPort),
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
		"name":                               spec.Name,
		"auto-tls":                           "false",
		"peer-auto-tls":                      "false",
		"data-dir":                           constants.EtcdDataPath,
		"listen-peer-urls":                   formatEtcdURLs(spec.ListenPeerAddresses, constants.EtcdPeerPort),
		"listen-client-urls":                 formatEtcdURLs(spec.ListenClientAddresses, constants.EtcdClientPort),
		"client-cert-auth":                   "true",
		"cert-file":                          constants.EtcdCert,
		"key-file":                           constants.EtcdKey,
		"trusted-ca-file":                    constants.EtcdCACert,
		"peer-client-cert-auth":              "true",
		"peer-cert-file":                     constants.EtcdPeerCert,
		"peer-key-file":                      constants.EtcdPeerKey,
		"peer-trusted-ca-file":               constants.EtcdCACert,
		"experimental-initial-corrupt-check": "true",
		"experimental-watch-progress-notify-interval": "5s",
		"experimental-compact-hash-check-enabled":     "true",
	}

	extraArgs := argsbuilder.Args(spec.ExtraArgs)

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
				denyListArgs.Set("initial-cluster-state", "new")
			} else {
				denyListArgs.Set("initial-cluster-state", "existing")
			}
		}

		if !extraArgs.Contains("initial-cluster") {
			var initialCluster string

			if e.Bootstrap {
				initialCluster = formatClusterURLs(spec.Name, getEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort))
			} else {
				initialCluster, e.learnerMemberID, err = buildInitialCluster(ctx, r, spec.Name, getEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort))
				if err != nil {
					return fmt.Errorf("failed to build initial etcd cluster: %w", err)
				}
			}

			denyListArgs.Set("initial-cluster", initialCluster)
		}

		if !extraArgs.Contains("initial-advertise-peer-urls") {
			denyListArgs.Set("initial-advertise-peer-urls",
				formatEtcdURLs(spec.AdvertisedAddresses, constants.EtcdPeerPort),
			)
		}
	}

	if !extraArgs.Contains("advertise-client-urls") {
		denyListArgs.Set("advertise-client-urls",
			formatEtcdURLs(spec.AdvertisedAddresses, constants.EtcdClientPort),
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

func promoteMember(ctx context.Context, r runtime.Runtime, memberID uint64) error {
	// try to promote a member until it succeeds (call might fail until the member catches up with the leader)
	// promote member call will fail until member catches up with the master
	//
	// iterate over all endpoints until we find the one which works
	// if we stick with the default behavior, we might hit the member being promoted, and that will never
	// promote itself.
	idx := 0

	return retry.Constant(10*time.Minute,
		retry.WithUnits(15*time.Second),
		retry.WithAttemptTimeout(30*time.Second),
		retry.WithJitter(time.Second),
		retry.WithErrorLogging(true),
	).RetryWithContext(ctx, func(ctx context.Context) error {
		endpoints, err := etcd.GetEndpoints(ctx, r.State().V1Alpha2().Resources())
		if err != nil {
			return retry.ExpectedError(err)
		}

		if len(endpoints) == 0 {
			return retry.ExpectedErrorf("no endpoints")
		}

		// try to iterate all available endpoints in the time available for an attempt
		for range endpoints {
			select {
			case <-ctx.Done():
				return retry.ExpectedError(ctx.Err())
			default:
			}

			endpoint := endpoints[idx%len(endpoints)]
			idx++

			err = attemptPromote(ctx, endpoint, memberID)
			if err == nil {
				return nil
			}
		}

		return retry.ExpectedError(err)
	})
}

func attemptPromote(ctx context.Context, endpoint string, memberID uint64) error {
	client, err := etcd.NewClient(ctx, []string{endpoint})
	if err != nil {
		return err
	}

	defer client.Close() //nolint:errcheck

	_, err = client.MemberPromote(ctx, memberID)

	return err
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
