// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	goruntime "runtime"
	"strings"
	"time"

	containerdapi "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/containerd/pkg/cap"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/go-retry/retry"
	"github.com/talos-systems/net"
	clientv3 "go.etcd.io/etcd/client/v3"
	snapshot "go.etcd.io/etcd/etcdutl/v3/snapshot"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/adv"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/pkg/argsbuilder"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
	timeresource "github.com/talos-systems/talos/pkg/machinery/resources/time"
)

// Etcd implements the Service interface. It serves as the concrete type with
// the required methods.
type Etcd struct {
	Bootstrap            bool
	RecoverFromSnapshot  bool
	RecoverSkipHashCheck bool

	args   []string
	client *etcd.Client

	// if the new member was added as a learner during the service start, its ID is kept here
	learnerMemberID uint64

	promoteCtxCancel context.CancelFunc
}

// ID implements the Service interface.
func (e *Etcd) ID(r runtime.Runtime) string {
	return "etcd"
}

// PreFunc implements the Service interface.
//
//nolint:gocyclo
func (e *Etcd) PreFunc(ctx context.Context, r runtime.Runtime) (err error) {
	if err = os.MkdirAll(constants.EtcdDataPath, 0o700); err != nil {
		return err
	}

	// Data path might exist after upgrade from previous version of Talos.
	if err = os.Chmod(constants.EtcdDataPath, 0o700); err != nil {
		return err
	}

	// Make sure etcd user can access files in the data directory.
	if err = chownRecursive(constants.EtcdDataPath, constants.EtcdUserID, constants.EtcdUserID); err != nil {
		return err
	}

	if err = generatePKI(ctx, r); err != nil {
		return fmt.Errorf("failed to generate etcd PKI: %w", err)
	}

	client, err := containerdapi.New(constants.CRIContainerdAddress)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer client.Close()

	// Pull the image and unpack it.
	containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	_, err = image.Pull(containerdctx, r.Config().Machine().Registries(), client, r.Config().Cluster().Etcd().Image(), image.WithSkipIfAlreadyPulled())
	if err != nil {
		return fmt.Errorf("failed to pull image %q: %w", r.Config().Cluster().Etcd().Image(), err)
	}

	// Clear any previously set learner member ID
	e.learnerMemberID = 0

	switch t := r.Config().Machine().Type(); t {
	case machine.TypeInit:
		return e.argsForInit(ctx, r)
	case machine.TypeControlPlane:
		return e.argsForControlPlane(ctx, r)
	case machine.TypeWorker:
		return fmt.Errorf("unexpected machine type: %v", t)
	case machine.TypeUnknown:
		fallthrough
	default:
		panic(fmt.Sprintf("unexpected machine type %v", t))
	}
}

// PostFunc implements the Service interface.
func (e *Etcd) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
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
	)
}

// DependsOn implements the Service interface.
func (e *Etcd) DependsOn(r runtime.Runtime) []string {
	return []string{"cri"}
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

	env := []string{}
	for key, val := range r.Config().Machine().Env() {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

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
		runner.WithContainerImage(r.Config().Cluster().Etcd().Image()),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			oci.WithDroppedCapabilities(cap.Known()),
			oci.WithHostNamespace(specs.NetworkNamespace),
			oci.WithMounts(mounts),
			oci.WithUser(fmt.Sprintf("%d:%d", constants.EtcdUserID, constants.EtcdUserID)),
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

			e.client, err = etcd.NewLocalClient()
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

//nolint:gocyclo
func generatePKI(ctx context.Context, r runtime.Runtime) (err error) {
	// remove legacy etcd PKI directory to handle upgrades with `--preserve` to Talos 0.12
	// TODO: remove me in Talos 0.13
	if err = os.RemoveAll("/etc/kubernetes/pki/etcd"); err != nil {
		return err
	}

	if err = os.MkdirAll(constants.EtcdPKIPath, 0o700); err != nil {
		return err
	}

	if err = ioutil.WriteFile(constants.KubernetesEtcdCACert, r.Config().Cluster().Etcd().CA().Crt, 0o400); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	if err = ioutil.WriteFile(constants.KubernetesEtcdCAKey, r.Config().Cluster().Etcd().CA().Key, 0o400); err != nil {
		return fmt.Errorf("failed to write CA key: %w", err)
	}

	// wait for etcd certificates to be generated in the controller
	watchCh := make(chan state.Event)

	if err = r.State().V1Alpha2().Resources().Watch(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.EtcdType, secrets.EtcdID, resource.VersionUndefined), watchCh); err != nil {
		return err
	}

	var event state.Event

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event = <-watchCh:
		}

		if event.Type == state.Created || event.Type == state.Updated {
			break
		}
	}

	etcdCerts := event.Resource.(*secrets.Etcd).Certs()

	for _, keypair := range []struct {
		getter   func() *x509.PEMEncodedCertificateAndKey
		keyPath  string
		certPath string
	}{
		{
			getter:   func() *x509.PEMEncodedCertificateAndKey { return etcdCerts.Etcd },
			keyPath:  constants.KubernetesEtcdKey,
			certPath: constants.KubernetesEtcdCert,
		},
		{
			getter:   func() *x509.PEMEncodedCertificateAndKey { return etcdCerts.EtcdPeer },
			keyPath:  constants.KubernetesEtcdPeerKey,
			certPath: constants.KubernetesEtcdPeerCert,
		},
		{
			getter:   func() *x509.PEMEncodedCertificateAndKey { return etcdCerts.EtcdAdmin },
			keyPath:  constants.KubernetesEtcdAdminKey,
			certPath: constants.KubernetesEtcdAdminCert,
		},
	} {
		if err = ioutil.WriteFile(keypair.keyPath, keypair.getter().Key, 0o400); err != nil {
			return err
		}

		if err = ioutil.WriteFile(keypair.certPath, keypair.getter().Crt, 0o400); err != nil {
			return err
		}
	}

	return chownRecursive(constants.EtcdPKIPath, constants.EtcdUserID, constants.EtcdUserID)
}

func addMember(ctx context.Context, r runtime.Runtime, addrs []string, name string) (*clientv3.MemberListResponse, uint64, error) {
	client, err := etcd.NewClientFromControlPlaneIPs(ctx, r.Config().Cluster().CA(), r.Config().Cluster().Endpoint())
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

func buildInitialCluster(ctx context.Context, r runtime.Runtime, name, ip string) (initial string, learnerMemberID uint64, err error) {
	var (
		id      uint64
		lastNag time.Time
	)

	err = retry.Constant(constants.EtcdJoinTimeout,
		retry.WithUnits(3*time.Second),
		retry.WithJitter(time.Second),
		retry.WithErrorLogging(true),
	).RetryWithContext(ctx, func(ctx context.Context) error {
		var (
			peerAddrs = []string{"https://" + net.FormatAddress(ip) + ":2380"}
			resp      *clientv3.MemberListResponse
		)

		if time.Since(lastNag) > 30*time.Second {
			log.Printf("etcd is waiting to join the cluster, if this node is the first node in the cluster, please run `talosctl bootstrap`")

			lastNag = time.Now()
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

		conf := []string{}

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

//nolint:gocyclo
func (e *Etcd) argsForInit(ctx context.Context, r runtime.Runtime) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	p, err := platform.CurrentPlatform()
	if err != nil {
		return err
	}

	var upgraded bool

	if p.Mode() != runtime.ModeContainer {
		var meta *bootloader.Meta

		if meta, err = bootloader.NewMeta(); err != nil {
			return err
		}
		//nolint:errcheck
		defer meta.Close()

		_, upgraded = meta.LegacyADV.ReadTag(adv.Upgrade)
	}

	primaryAddr, listenAddress, err := primaryAndListenAddresses(r.Config().Cluster().Etcd().Subnet())
	if err != nil {
		return fmt.Errorf("failed to calculate etcd addresses: %w", err)
	}

	// TODO(scm): see issue #2121 and description below in argsForControlPlane.
	denyListArgs := argsbuilder.Args{
		"name":                  hostname,
		"data-dir":              constants.EtcdDataPath,
		"listen-peer-urls":      "https://" + net.FormatAddress(listenAddress) + ":2380",
		"listen-client-urls":    "https://" + net.FormatAddress(listenAddress) + ":2379",
		"client-cert-auth":      "true",
		"cert-file":             constants.KubernetesEtcdCert,
		"key-file":              constants.KubernetesEtcdKey,
		"trusted-ca-file":       constants.KubernetesEtcdCACert,
		"peer-client-cert-auth": "true",
		"peer-cert-file":        constants.KubernetesEtcdPeerCert,
		"peer-key-file":         constants.KubernetesEtcdPeerKey,
		"peer-trusted-ca-file":  constants.KubernetesEtcdCACert,
	}

	extraArgs := argsbuilder.Args(r.Config().Cluster().Etcd().ExtraArgs())

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
			initialCluster := fmt.Sprintf("%s=https://%s:2380", hostname, net.FormatAddress(primaryAddr))

			if upgraded {
				denyListArgs.Set("initial-cluster-state", "existing")

				initialCluster, e.learnerMemberID, err = buildInitialCluster(ctx, r, hostname, primaryAddr)
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
		denyListArgs.Set("initial-advertise-peer-urls", fmt.Sprintf("https://%s:2380", net.FormatAddress(primaryAddr)))
	}

	if !extraArgs.Contains("advertise-client-urls") {
		denyListArgs.Set("advertise-client-urls", fmt.Sprintf("https://%s:2379", net.FormatAddress(primaryAddr)))
	}

	if err := denyListArgs.Merge(extraArgs, denyList); err != nil {
		return err
	}

	e.args = denyListArgs.Args()

	return nil
}

//nolint:gocyclo
func (e *Etcd) argsForControlPlane(ctx context.Context, r runtime.Runtime) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	// TODO(scm):  With the current setup, the listen (bind) address is
	// essentially hard-coded because we need to calculate it before we process
	// extraArgs (which may contain special overrides from the user.
	// This needs to be refactored to allow greater binding flexibility.
	// Issue #2121.
	primaryAddr, listenAddress, err := primaryAndListenAddresses(r.Config().Cluster().Etcd().Subnet())
	if err != nil {
		return fmt.Errorf("failed to calculate etcd addresses: %w", err)
	}

	denyListArgs := argsbuilder.Args{
		"name":                  hostname,
		"data-dir":              constants.EtcdDataPath,
		"listen-peer-urls":      "https://" + net.FormatAddress(listenAddress) + ":2380",
		"listen-client-urls":    "https://" + net.FormatAddress(listenAddress) + ":2379",
		"client-cert-auth":      "true",
		"cert-file":             constants.KubernetesEtcdPeerCert,
		"key-file":              constants.KubernetesEtcdPeerKey,
		"trusted-ca-file":       constants.KubernetesEtcdCACert,
		"peer-client-cert-auth": "true",
		"peer-cert-file":        constants.KubernetesEtcdPeerCert,
		"peer-key-file":         constants.KubernetesEtcdPeerKey,
		"peer-trusted-ca-file":  constants.KubernetesEtcdCACert,
	}

	extraArgs := argsbuilder.Args(r.Config().Cluster().Etcd().ExtraArgs())

	denyList := argsbuilder.WithDenyList(denyListArgs)

	if e.RecoverFromSnapshot {
		if err = e.recoverFromSnapshot(hostname, primaryAddr); err != nil {
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
				initialCluster = fmt.Sprintf("%s=https://%s:2380", hostname, net.FormatAddress(primaryAddr))
			} else {
				initialCluster, e.learnerMemberID, err = buildInitialCluster(ctx, r, hostname, primaryAddr)
				if err != nil {
					return fmt.Errorf("failed to build initial etcd cluster: %w", err)
				}
			}

			denyListArgs.Set("initial-cluster", initialCluster)
		}

		if !extraArgs.Contains("initial-advertise-peer-urls") {
			denyListArgs.Set("initial-advertise-peer-urls", fmt.Sprintf("https://%s:2380", net.FormatAddress(primaryAddr)))
		}
	}

	if !extraArgs.Contains("advertise-client-urls") {
		denyListArgs.Set("advertise-client-urls", fmt.Sprintf("https://%s:2379", net.FormatAddress(primaryAddr)))
	}

	if err = denyListArgs.Merge(extraArgs, denyList); err != nil {
		return err
	}

	e.args = denyListArgs.Args()

	return nil
}

// recoverFromSnapshot recovers etcd data directory from the snapshot uploaded previously.
func (e *Etcd) recoverFromSnapshot(hostname, primaryAddr string) error {
	manager := snapshot.NewV3(nil)

	status, err := manager.Status(constants.EtcdRecoverySnapshotPath)
	if err != nil {
		return fmt.Errorf("error verifying snapshot: %w", err)
	}

	log.Printf("recovering etcd from snapshot: hash %08x, revision %d, total keys %d, total size %d\n",
		status.Hash, status.Revision, status.TotalKey, status.TotalSize)

	if err = manager.Restore(snapshot.RestoreConfig{
		SnapshotPath: constants.EtcdRecoverySnapshotPath,

		Name:          hostname,
		OutputDataDir: constants.EtcdDataPath,

		PeerURLs: []string{"https://" + net.FormatAddress(primaryAddr) + ":2380"},

		InitialCluster: fmt.Sprintf("%s=https://%s:2380", hostname, net.FormatAddress(primaryAddr)),

		SkipHashCheck: e.RecoverSkipHashCheck,
	}); err != nil {
		return fmt.Errorf("error recovering from the snapshot: %w", err)
	}

	if err = os.Remove(constants.EtcdRecoverySnapshotPath); err != nil {
		return fmt.Errorf("error deleting snapshot: %w", err)
	}

	return chownRecursive(constants.EtcdDataPath, constants.EtcdUserID, constants.EtcdUserID)
}

func promoteMember(ctx context.Context, r runtime.Runtime, memberID uint64) error {
	// try to promote a member until it succeeds (call might fail until the member catches up with the leader)
	// promote member call will fail until member catches up with the master
	return retry.Constant(10*time.Minute,
		retry.WithUnits(10*time.Second),
		retry.WithJitter(time.Second),
		retry.WithErrorLogging(true),
	).RetryWithContext(ctx, func(ctx context.Context) error {
		client, err := etcd.NewClientFromControlPlaneIPs(ctx, r.Config().Cluster().CA(), r.Config().Cluster().Endpoint())
		if err != nil {
			return retry.ExpectedError(err)
		}

		defer client.Close() //nolint:errcheck

		_, err = client.MemberPromote(ctx, memberID)

		return retry.ExpectedError(err)
	})
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

// primaryAndListenAddresses calculates the primary (advertised) and listen (bind) addresses for etcd.
func primaryAndListenAddresses(subnet string) (primary, listen string, err error) {
	ips, err := net.IPAddrs()
	if err != nil {
		return "", "", fmt.Errorf("failed to discover interface IP addresses: %w", err)
	}

	ips = net.IPFilter(ips, network.NotSideroLinkStdIP)

	if len(ips) == 0 {
		return "", "", errors.New("no valid unicast IP addresses on any interface")
	}

	if subnet == "" {
		primary = ips[0].String()
	} else {
		network, err := net.ParseCIDR(subnet)
		if err != nil {
			return "", "", fmt.Errorf("failed to parse subnet: %w", err)
		}

		for _, ip := range ips {
			if network.Contains(ip) {
				primary = ip.String()

				break
			}
		}

		if primary == "" {
			return "", "", errors.New("no address matched the provided subnet")
		}
	}

	// Regardless of primary selected IP, we should be liberal with our listen
	// address, for maximum compatibility.  Again, this should probably be
	// exposed later for greater control.
	listen = "0.0.0.0"
	if net.IsIPv6(ips...) {
		listen = "::"
	}

	return primary, listen, nil
}
