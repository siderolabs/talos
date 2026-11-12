// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"crypto/tls"
	stdx509 "crypto/x509"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"slices"
	"time"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
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

	return newClient(ctx, endpoints, tlsConfig, dialOpts...)
}

// NewClientFromSpec initializes and returns an etcd client configured to talk to a list of endpoints,
// using the credentials from the etcd client spec (instead of the etcd PKI on disk).
func NewClientFromSpec(ctx context.Context, spec *etcdresource.ClientSpecSpec, endpoints []string, dialOpts ...grpc.DialOption) (*Client, error) {
	if spec.Certificate == nil {
		return nil, errors.New("etcd client certificate is missing")
	}

	cert, err := tls.X509KeyPair(spec.Certificate.Crt, spec.Certificate.Key)
	if err != nil {
		return nil, fmt.Errorf("error loading etcd client certificate: %w", err)
	}

	rootCAs := stdx509.NewCertPool()

	for _, ca := range spec.AcceptedCAs {
		if !rootCAs.AppendCertsFromPEM(ca.Crt) {
			return nil, errors.New("error loading etcd CA certificate")
		}
	}

	return newClient(ctx, endpoints, &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		RootCAs:      rootCAs,
	}, dialOpts...)
}

// NewClusterClientFromSpec initializes and returns an etcd client configured to talk to the etcd members across the cluster.
func NewClusterClientFromSpec(ctx context.Context, spec *etcdresource.ClientSpecSpec, dialOpts ...grpc.DialOption) (*Client, error) {
	if len(spec.ClusterEndpoints) == 0 {
		return nil, errNoEndpoints
	}

	endpoints := slices.Clone(spec.ClusterEndpoints)

	// Shuffle endpoints to establish random order on each call to avoid patterns based on sorted IP list.
	rand.Shuffle(len(endpoints), func(i, j int) { endpoints[i], endpoints[j] = endpoints[j], endpoints[i] })

	return NewClientFromSpec(ctx, spec, endpoints, dialOpts...)
}

func newClient(ctx context.Context, endpoints []string, tlsConfig *tls.Config, dialOpts ...grpc.DialOption) (*Client, error) {
	c, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		Context:     ctx,
		DialOptions: dialOpts,
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
		LocalEndpoints(),
		dialOpts...,
	)
}

// NewClientFromControlPlaneIPs initializes and returns an etcd client
// configured to talk to all members.
func NewClientFromControlPlaneIPs(ctx context.Context, resources ResourceLister, dialOpts ...grpc.DialOption) (client *Client, err error) {
	endpoints, err := GetEndpoints(ctx, resources)
	if err != nil {
		return nil, err
	}

	// Shuffle endpoints to establish random order on each call to avoid patterns based on sorted IP list.
	rand.Shuffle(len(endpoints), func(i, j int) { endpoints[i], endpoints[j] = endpoints[j], endpoints[i] })

	return NewClient(ctx, endpoints, dialOpts...)
}

// PromoteMember promotes a learner member to a voting member.
//
// It is a fail-fast version of clientv3's MemberPromote: clientv3 hardcodes grpc.WaitForReady(true)
// in its default call options (and provides no way to override it via clientv3.Config), which makes
// a call against an endpoint which is not accepting connections block until the context deadline
// instead of returning the connection error. As the promotion call has to find a voting member to
// talk to, blocking on a dead endpoint is exactly the wrong behavior.
func (c *Client) PromoteMember(ctx context.Context, memberID uint64) error {
	_, err := clientv3.RetryClusterClient(c.Client).MemberPromote(
		ctx,
		&etcdserverpb.MemberPromoteRequest{ID: memberID},
		grpc.WaitForReady(false),
	)

	return clientv3.ContextError(ctx, err)
}

// ValidateForUpgrade validates the etcd cluster state to ensure that performing
// an upgrade is safe.
func (c *Client) ValidateForUpgrade(ctx context.Context, config config.Config) error {
	if config.Machine().Type() == machine.TypeWorker {
		return nil
	}

	resp, err := c.MemberList(ctx)
	if err != nil {
		return err
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

	defer c.Close() //nolint:errcheck

	return c.ValidateQuorum(ctx)
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
