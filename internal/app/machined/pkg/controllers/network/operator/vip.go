// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operator

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/operator/vip"
	"github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

const campaignRetryInterval = time.Second

// VIP implements the Virtual (Shared) IP network operator.
type VIP struct {
	logger *zap.Logger

	linkName      string
	sharedIP      netip.Addr
	gratuitousARP bool

	state state.State

	mu     sync.Mutex
	leader bool

	handler vip.Handler
}

// NewVIP creates Virtual IP operator.
func NewVIP(logger *zap.Logger, linkName string, spec network.VIPOperatorSpec, state state.State) *VIP {
	var handler vip.Handler

	switch {
	case spec.EquinixMetal != network.VIPEquinixMetalSpec{}:
		handler = vip.NewEquinixMetalHandler(logger, spec.IP.String(), spec.EquinixMetal)
	case spec.HCloud != network.VIPHCloudSpec{}:
		handler = vip.NewHCloudHandler(logger, spec.IP.String(), spec.HCloud)
	default:
		handler = vip.NopHandler{}
	}

	return &VIP{
		logger:        logger,
		linkName:      linkName,
		sharedIP:      spec.IP,
		gratuitousARP: spec.GratuitousARP,
		state:         state,
		handler:       handler,
	}
}

// Prefix returns unique operator prefix which gets prepended to each spec.
func (vip *VIP) Prefix() string {
	return fmt.Sprintf("vip/%s", vip.linkName)
}

// Run the operator loop.
func (vip *VIP) Run(ctx context.Context, notifyCh chan<- struct{}) {
	for {
		err := vip.campaign(ctx, notifyCh)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				vip.logger.Warn("campaign failure", zap.Error(err), zap.String("link", vip.linkName), zap.Stringer("ip", vip.sharedIP))
			}

			select {
			case <-time.After(campaignRetryInterval):
			case <-ctx.Done():
				return
			}
		}
	}
}

// AddressSpecs implements Operator interface.
func (vip *VIP) AddressSpecs() []network.AddressSpecSpec {
	vip.mu.Lock()
	defer vip.mu.Unlock()

	if !vip.leader {
		return nil
	}

	family := nethelpers.FamilyInet6
	gratuitousARP := false

	if vip.sharedIP.Is4() {
		family = nethelpers.FamilyInet4
		gratuitousARP = vip.gratuitousARP
	}

	return []network.AddressSpecSpec{
		{
			Address:         netip.PrefixFrom(vip.sharedIP, vip.sharedIP.BitLen()),
			LinkName:        vip.linkName,
			Family:          family,
			Scope:           nethelpers.ScopeGlobal,
			Flags:           nethelpers.AddressFlags(nethelpers.AddressPermanent),
			AnnounceWithARP: gratuitousARP,
			ConfigLayer:     network.ConfigOperator,
		},
	}
}

// LinkSpecs implements Operator interface.
func (vip *VIP) LinkSpecs() []network.LinkSpecSpec {
	return nil
}

// RouteSpecs implements Operator interface.
func (vip *VIP) RouteSpecs() []network.RouteSpecSpec {
	return nil
}

// HostnameSpecs implements Operator interface.
func (vip *VIP) HostnameSpecs() []network.HostnameSpecSpec {
	return nil
}

// ResolverSpecs implements Operator interface.
func (vip *VIP) ResolverSpecs() []network.ResolverSpecSpec {
	return nil
}

// TimeServerSpecs implements Operator interface.
func (vip *VIP) TimeServerSpecs() []network.TimeServerSpecSpec {
	return nil
}

func (vip *VIP) etcdElectionKey() string {
	return fmt.Sprintf("%s:vip:election:%s", constants.EtcdRootTalosKey, vip.sharedIP.String())
}

func (vip *VIP) waitForPreconditions(ctx context.Context) error {
	//  wait for the etcd to be up
	_, err := vip.state.WatchFor(ctx, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "etcd", resource.VersionUndefined),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			if resource.IsTombstone(r) {
				return false, nil
			}

			svc := r.(*v1alpha1.Service) //nolint:forcetypeassert

			return svc.TypedSpec().Running && svc.TypedSpec().Healthy, nil
		}))
	if err != nil {
		return fmt.Errorf("etcd health wait failure: %w", err)
	}

	// wait for the kubelet lifecycle to be up, and not being torn down
	_, err = vip.state.WatchFor(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.KubeletLifecycleType, k8s.KubeletLifecycleID, resource.VersionUndefined),
		state.WithCondition(func(r resource.Resource) (bool, error) {
			if resource.IsTombstone(r) {
				return false, nil
			}

			if r.Metadata().Phase() == resource.PhaseTearingDown {
				return false, nil
			}

			return true, nil
		}))
	if err != nil {
		return fmt.Errorf("kubelet lifecycle wait failure: %w", err)
	}

	return nil
}

//nolint:gocyclo,cyclop
func (vip *VIP) campaign(ctx context.Context, notifyCh chan<- struct{}) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if err := vip.waitForPreconditions(ctx); err != nil {
		return fmt.Errorf("error waiting for preconditions: %w", err)
	}

	// put a finalizer on the kubelet lifecycle and remove once the campaign is done
	kubeletLifecycle := resource.NewMetadata(k8s.NamespaceName, k8s.KubeletLifecycleType, k8s.KubeletLifecycleID, resource.VersionUndefined)
	if err := vip.state.AddFinalizer(ctx, kubeletLifecycle, vip.Prefix()); err != nil {
		return fmt.Errorf("error adding kubelet lifecycle finalizer: %w", err)
	}

	defer func() {
		vip.state.RemoveFinalizer(ctx, kubeletLifecycle, vip.Prefix()) //nolint:errcheck
	}()

	hostname, err := os.Hostname() // TODO: this should be etcd nodename
	if err != nil {
		return errors.New("refusing to join election without a hostname")
	}

	ec, err := etcd.NewLocalClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create local etcd client: %w", err)
	}

	defer ec.Close() //nolint:errcheck

	sess, err := concurrency.NewSession(ec.Client)
	if err != nil {
		return fmt.Errorf("failed to create concurrency session: %w", err)
	}
	defer sess.Close() //nolint:errcheck

	election := concurrency.NewElection(sess, vip.etcdElectionKey())

	node, err := election.Leader(ctx)
	if err != nil {
		if err != concurrency.ErrElectionNoLeader {
			return fmt.Errorf("failed getting current leader: %w", err)
		}
	} else if string(node.Kvs[0].Value) == hostname {
		vip.logger.Info("resigning from previous election")

		// we are still leader from the previous election, attempt to resign to force new election
		resumedElection := concurrency.ResumeElection(sess, vip.etcdElectionKey(), string(node.Kvs[0].Key), node.Kvs[0].CreateRevision)

		if err = resumedElection.Resign(ctx); err != nil {
			return fmt.Errorf("failed resigning from previous elections: %w", err)
		}
	}

	campaignErrCh := make(chan error)

	go func() {
		campaignErrCh <- election.Campaign(ctx, hostname)
	}()

	watchCh := make(chan state.Event)

	if err = vip.state.Watch(ctx, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "etcd", resource.VersionUndefined), watchCh); err != nil {
		return fmt.Errorf("error setting up etcd watch: %w", err)
	}

	if err = vip.state.Watch(ctx, kubeletLifecycle, watchCh); err != nil {
		return fmt.Errorf("error setting up etcd watch: %w", err)
	}

	err = vip.state.WatchKind(ctx, resource.NewMetadata(k8s.NamespaceName, k8s.StaticPodStatusType, "", resource.VersionUndefined), watchCh)
	if err != nil {
		return fmt.Errorf("kube-apiserver health wait failure: %w", err)
	}

	// wait for the etcd election campaign to be complete
	// while waiting, also observe the kubelet lifecycle object (if the node is shutting down) and etcd status
campaignLoop:
	for {
		select {
		case err = <-campaignErrCh:
			if err != nil {
				return fmt.Errorf("failed to conduct campaign: %w", err)
			}

			// node won the election campaign!
			break campaignLoop
		case <-sess.Done():
			vip.logger.Info("etcd session closed")

			return nil
		case <-ctx.Done():
			return nil
		case event := <-watchCh:
			// note: here we don't wait for kube-apiserver, as it might not be up on cluster bootstrap, but VIP should be still assigned
			// break the loop when etcd is stopped
			if event.Type == state.Destroyed && event.Resource.Metadata().ID() == "etcd" {
				return nil
			}

			// break the loop if the kubelet lifecycle is entering teardown phase
			if event.Resource != nil {
				if event.Resource.Metadata().Type() == kubeletLifecycle.Type() && event.Resource.Metadata().ID() == kubeletLifecycle.ID() && event.Resource.Metadata().Phase() == resource.PhaseTearingDown {
					return nil
				}
			}
		}
	}

	defer func() {
		// use a new context to resign, as `ctx` might be canceled
		resignCtx, resignCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer resignCancel()

		election.Resign(resignCtx) //nolint:errcheck
	}()

	if err = vip.markAsLeader(ctx, notifyCh, true); err != nil {
		return err
	}

	defer func() {
		if err = vip.markAsLeader(ctx, notifyCh, false); err != nil && !errors.Is(err, context.Canceled) {
			vip.logger.Info("failed disabling shared IP", zap.String("link", vip.linkName), zap.Stringer("ip", vip.sharedIP), zap.Error(err))
		}

		vip.logger.Info("removing shared IP", zap.String("link", vip.linkName), zap.Stringer("ip", vip.sharedIP))
	}()

	vip.logger.Info("enabled shared IP", zap.String("link", vip.linkName), zap.Stringer("ip", vip.sharedIP))

	observe := election.Observe(ctx)

observeLoop:
	for {
		select {
		case <-sess.Done():
			vip.logger.Info("etcd session closed")

			break observeLoop
		case <-ctx.Done():
			break observeLoop
		case resp, ok := <-observe:
			if !ok {
				break observeLoop
			}

			if string(resp.Kvs[0].Value) != hostname {
				vip.logger.Info("detected new leader", zap.ByteString("leader", resp.Kvs[0].Value))

				break observeLoop
			}
		case event := <-watchCh:
			// break the loop when etcd is stopped or kube-apiserver is stopped
			if event.Type == state.Destroyed {
				if event.Resource.Metadata().ID() == "etcd" || strings.HasPrefix(event.Resource.Metadata().ID(), "kube-system/kube-apiserver-") {
					break observeLoop
				}
			}

			// break the loop if the kubelet lifecycle is entering teardown phase
			if event.Resource != nil {
				if event.Resource.Metadata().Type() == kubeletLifecycle.Type() && event.Resource.Metadata().ID() == kubeletLifecycle.ID() && event.Resource.Metadata().Phase() == resource.PhaseTearingDown {
					break observeLoop
				}
			}
		}
	}

	return nil
}

func (vip *VIP) markAsLeader(ctx context.Context, notifyCh chan<- struct{}, leader bool) error {
	var handlerErr error

	if leader {
		handlerErr = vip.handler.Acquire(ctx)

		if handlerErr != nil {
			// if failed to acquire, we are not a leader, we will resign from the election
			// so don't mark as leader, so that Talos doesn't announce IPs on the host
			leader = false
		}
	} else {
		handlerErr = vip.handler.Release(ctx)
	}

	func() {
		vip.mu.Lock()
		defer vip.mu.Unlock()

		vip.leader = leader
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case notifyCh <- struct{}{}:
		return handlerErr
	}
}
