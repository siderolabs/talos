// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	etcdresource "github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

// QuorumCheckTimeout is the amount of time to allow for KV operations before quorum is declared invalid.
const QuorumCheckTimeout = 15 * time.Second

// Client is a wrapper around the official etcd client.
type Client struct {
	*clientv3.Client
}

// NewClient initializes and returns an etcd client configured to talk to
// a list of endpoints.
func NewClient(ctx context.Context, endpoints []string, dialOpts ...grpc.DialOption) (client *Client, err error) {
	tlsInfo := transport.TLSInfo{
		CertFile:      constants.EtcdAdminCert,
		KeyFile:       constants.EtcdAdminKey,
		TrustedCAFile: constants.EtcdCACert,
	}

	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error building etcd client TLS config: %w", err)
	}

	c, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		Context:     ctx,
		DialOptions: append(dialOpts, grpc.WithSharedWriteBuffer(true)),
		TLS:         tlsConfig,
		Logger:      zap.NewNop(),
	})
	if err != nil {
		return nil, fmt.Errorf("error building etcd client: %w", err)
	}

	return &Client{Client: c}, nil
}

// NewLocalClient initializes and returns etcd client configured to talk to localhost endpoint.
func NewLocalClient(ctx context.Context, dialOpts ...grpc.DialOption) (client *Client, err error) {
	return NewClient(
		ctx,
		[]string{nethelpers.JoinHostPort("localhost", constants.EtcdClientPort)},
		append([]grpc.DialOption{grpc.WithBlock()}, dialOpts...)...,
	)
}

// NewClientFromControlPlaneIPs initializes and returns an etcd client
// configured to talk to all members.
func NewClientFromControlPlaneIPs(ctx context.Context, resources state.State, dialOpts ...grpc.DialOption) (client *Client, err error) {
	endpoints, err := GetEndpoints(ctx, resources)
	if err != nil {
		return nil, err
	}

	// Shuffle endpoints to establish random order on each call to avoid patterns based on sorted IP list.
	rand.Shuffle(len(endpoints), func(i, j int) { endpoints[i], endpoints[j] = endpoints[j], endpoints[i] })

	return NewClient(ctx, endpoints, dialOpts...)
}

// ValidateForUpgrade validates the etcd cluster state to ensure that performing
// an upgrade is safe.
func (c *Client) ValidateForUpgrade(ctx context.Context, config config.Config, preserve bool) error {
	if config.Machine().Type() == machine.TypeWorker {
		return nil
	}

	resp, err := c.MemberList(ctx)
	if err != nil {
		return err
	}

	if !preserve {
		if len(resp.Members) == 1 {
			return errors.New("only 1 etcd member found; assuming this is not an HA setup and refusing to upgrade; if this is a single-node cluster, use --preserve to upgrade")
		}
	}

	if len(resp.Members) == 2 {
		return fmt.Errorf("etcd member count(%d) is insufficient to maintain quorum if upgrade commences", len(resp.Members))
	}

	for _, member := range resp.Members {
		// If the member is not started, the name will be an empty string.
		if len(member.Name) == 0 {
			return fmt.Errorf("etcd member %016x is not started, all members must be running to perform an upgrade", member.ID)
		}

		if err = validateMemberHealth(ctx, member.GetClientURLs()); err != nil {
			return fmt.Errorf("etcd member %016x is not healthy; all members must be healthy to perform an upgrade: %w", member.ID, err)
		}
	}

	return nil
}

// ValidateQuorum performs a KV operation to make certain that quorum is good.
func (c *Client) ValidateQuorum(ctx context.Context) (err error) {
	// Get a random key. As long as we can get the response without an error, quorum is good.
	checkCtx, cancel := context.WithTimeout(ctx, QuorumCheckTimeout)
	defer cancel()

	_, err = c.Get(checkCtx, "health")
	if err == rpctypes.ErrPermissionDenied {
		// Permission denied is OK since proposal goes through consensus to get this error.
		err = nil
	}

	if err != nil {
		return err
	}

	return nil
}

func validateMemberHealth(ctx context.Context, memberURIs []string) (err error) {
	c, err := NewClient(ctx, memberURIs)
	if err != nil {
		return fmt.Errorf("failed to create client to member: %w", err)
	}

	return c.ValidateQuorum(ctx)
}

// LeaveCluster removes the current member from the etcd cluster and nukes etcd data directory.
func (c *Client) LeaveCluster(ctx context.Context, st state.State) error {
	memberID, err := GetLocalMemberID(ctx, st)
	if err != nil {
		return err
	}

	if err := retry.Constant(5*time.Minute, retry.WithUnits(10*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
		err := c.RemoveMemberByMemberID(ctx, memberID)
		if err == nil {
			return nil
		}

		if errors.Is(err, rpctypes.ErrUnhealthy) {
			// unhealthy is returned when the member hasn't established connections with quorum other members
			return retry.ExpectedError(err)
		}

		return err
	}); err != nil {
		return err
	}

	if err := system.Services(nil).Stop(ctx, "etcd"); err != nil {
		return fmt.Errorf("failed to stop etcd: %w", err)
	}

	// Once the member is removed, the data is no longer valid.
	if err := os.RemoveAll(constants.EtcdDataPath); err != nil {
		return fmt.Errorf("failed to remove %s: %w", constants.EtcdDataPath, err)
	}

	return nil
}

// GetMemberID returns the member ID of the node client is connected to.
func (c *Client) GetMemberID(ctx context.Context) (uint64, error) {
	resp, err := c.Client.Maintenance.AlarmList(ctx)
	if err != nil {
		return 0, err
	}

	return resp.Header.MemberId, nil
}

// RemoveMemberByMemberID removes the member from the etcd cluster.
func (c *Client) RemoveMemberByMemberID(ctx context.Context, memberID uint64) error {
	_, err := c.MemberRemove(ctx, memberID)
	if err != nil {
		return fmt.Errorf("failed to remove member %d: %w", memberID, err)
	}

	return nil
}

// ForfeitLeadership transfers leadership from the current member to another
// member.
//
//nolint:gocyclo
func (c *Client) ForfeitLeadership(ctx context.Context, memberID string) (string, error) {
	resp, err := c.MemberList(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list etcd members: %w", err)
	}

	if len(resp.Members) == 1 {
		return "", errors.New("cannot forfeit leadership, only one member")
	}

	var member *etcdserverpb.Member

	memberIDUint64, err := etcdresource.ParseMemberID(memberID)
	if err != nil {
		return "", err
	}

	for _, m := range resp.Members {
		if m.ID == memberIDUint64 {
			member = m

			break
		}
	}

	if member == nil {
		return "", fmt.Errorf("failed to find %q in list of etcd members", memberID)
	}

	for _, ep := range member.GetClientURLs() {
		var status *clientv3.StatusResponse

		status, err = c.Status(ctx, ep)
		if err != nil {
			return "", err
		}

		if status.Leader != member.GetID() {
			return "", nil
		}

		for _, m := range resp.Members {
			if m.GetID() != member.GetID() {
				log.Printf("moving leadership from %q to %q", member.GetName(), m.GetName())

				conn, err := c.Dial(ep)
				if err != nil {
					return "", err
				}

				maintenance := clientv3.NewMaintenanceFromMaintenanceClient(clientv3.RetryMaintenanceClient(c.Client, conn), c.Client)

				_, err = maintenance.MoveLeader(ctx, m.GetID())
				if err != nil {
					if errors.Is(err, rpctypes.ErrNotLeader) {
						// this member is not a leader anymore, so nothing to be done for the forfeit leadership
						return "", nil
					}

					return "", err
				}

				return m.GetName(), nil
			}
		}
	}

	return "", nil
}
