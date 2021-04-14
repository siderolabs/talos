// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/net"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/v3/transport"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// QuorumCheckTimeout is the amount of time to allow for KV operations before quorum is declared invalid.
const QuorumCheckTimeout = 15 * time.Second

// Client is a wrapper around the official etcd client.
type Client struct {
	*clientv3.Client
}

// NewClient initializes and returns an etcd client configured to talk to
// a list of endpoints.
func NewClient(endpoints []string) (client *Client, err error) {
	tlsInfo := transport.TLSInfo{
		CertFile:      constants.KubernetesEtcdPeerCert,
		KeyFile:       constants.KubernetesEtcdPeerKey,
		TrustedCAFile: constants.KubernetesEtcdCACert,
	}

	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error building etcd client TLS config: %w", err)
	}

	c, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
		TLS:         tlsConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("error building etcd client: %w", err)
	}

	return &Client{Client: c}, nil
}

// NewLocalClient initializes and returns etcd client configured to talk to localhost endpoint.
func NewLocalClient() (client *Client, err error) {
	return NewClient([]string{"127.0.0.1:2379"})
}

// NewClientFromControlPlaneIPs initializes and returns an etcd client
// configured to talk to all members.
func NewClientFromControlPlaneIPs(ctx context.Context, creds *x509.PEMEncodedCertificateAndKey, endpoint *url.URL) (client *Client, err error) {
	h, err := kubernetes.NewTemporaryClientFromPKI(creds, endpoint)
	if err != nil {
		return nil, fmt.Errorf("error building kubernetes client from PKI: %w", err)
	}

	var endpoints []string

	if endpoints, err = h.MasterIPs(ctx); err != nil {
		return nil, fmt.Errorf("error getting kubernetes endpoints: %w", err)
	}

	// Etcd expects host:port format.
	for i := 0; i < len(endpoints); i++ {
		endpoints[i] = net.FormatAddress(endpoints[i]) + ":2379"
	}

	return NewClient(endpoints)
}

// ValidateForUpgrade validates the etcd cluster state to ensure that performing
// an upgrade is safe.
func (c *Client) ValidateForUpgrade(ctx context.Context, config config.Provider, preserve bool) error {
	if config.Machine().Type() != machine.TypeJoin {
		resp, err := c.MemberList(context.Background())
		if err != nil {
			return err
		}

		if !preserve {
			if len(resp.Members) == 1 {
				return fmt.Errorf("only 1 etcd member found. assuming this is not an HA setup and refusing to upgrade")
			}
		}

		if len(resp.Members) == 2 {
			return fmt.Errorf("etcd member count(%d) is insufficient to maintain quorum if upgrade commences", len(resp.Members))
		}

		for _, member := range resp.Members {
			// If the member is not started, the name will be an empty string.
			if len(member.Name) == 0 {
				return fmt.Errorf("etcd member %d is not started, all members must be running to perform an upgrade", member.ID)
			}

			if err = validateMemberHealth(ctx, member.GetClientURLs()); err != nil {
				return fmt.Errorf("etcd member %d is not healthy; all members must be healthy to perform an upgrade: %w", member.ID, err)
			}
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
	c, err := NewClient(memberURIs)
	if err != nil {
		return fmt.Errorf("failed to create client to member: %w", err)
	}

	return c.ValidateQuorum(ctx)
}

// LeaveCluster removes the current member from the etcd cluster and nukes etcd data directory.
func (c *Client) LeaveCluster(ctx context.Context) error {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	if err = c.RemoveMember(ctx, hostname); err != nil {
		return err
	}

	if err = system.Services(nil).Stop(ctx, "etcd"); err != nil {
		return fmt.Errorf("failed to stop etcd: %w", err)
	}

	// Once the member is removed, the data is no longer valid.
	if err = os.RemoveAll(constants.EtcdDataPath); err != nil {
		return fmt.Errorf("failed to remove %s: %w", constants.EtcdDataPath, err)
	}

	return nil
}

// RemoveMember removes the member from the etcd cluster.
func (c *Client) RemoveMember(ctx context.Context, hostname string) error {
	resp, err := c.MemberList(ctx)
	if err != nil {
		return err
	}

	var id *uint64

	for _, member := range resp.Members {
		if member.Name == hostname {
			member := member
			id = &member.ID

			break
		}
	}

	if id == nil {
		return fmt.Errorf("failed to find %q in list of etcd members", hostname)
	}

	_, err = c.MemberRemove(ctx, *id)
	if err != nil {
		return fmt.Errorf("failed to remove member %d: %w", *id, err)
	}

	return nil
}

// ForfeitLeadership transfers leadership from the current member to another
// member.
//
//nolint:gocyclo
func (c *Client) ForfeitLeadership(ctx context.Context) (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	resp, err := c.MemberList(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list etcd members: %w", err)
	}

	if len(resp.Members) == 1 {
		return "", fmt.Errorf("cannot forfeit leadership, only one member")
	}

	var member *etcdserverpb.Member

	for _, m := range resp.Members {
		if m.Name == hostname {
			member = m

			break
		}
	}

	if member == nil {
		return "", fmt.Errorf("failed to find %q in list of etcd members", hostname)
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

				c.SetEndpoints(ep)

				_, err = c.MoveLeader(ctx, m.GetID())
				if err != nil {
					return "", err
				}

				return m.GetName(), nil
			}
		}
	}

	return "", nil
}
