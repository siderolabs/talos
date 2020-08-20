// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"
	"google.golang.org/grpc"

	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/net"

	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// NewClient initializes and returns an etcd client configured to talk to
// a local endpoint.
func NewClient(endpoints []string) (client *clientv3.Client, err error) {
	tlsInfo := transport.TLSInfo{
		CertFile:      constants.KubernetesEtcdPeerCert,
		KeyFile:       constants.KubernetesEtcdPeerKey,
		TrustedCAFile: constants.KubernetesEtcdCACert,
	}

	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return nil, err
	}

	client, err = clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		DialOptions: []grpc.DialOption{grpc.WithBlock()},
		TLS:         tlsConfig,
	})
	if err != nil {
		return nil, err
	}

	return client, nil
}

// NewClientFromControlPlaneIPs initializes and returns an etcd client
// configured to talk to all members.
func NewClientFromControlPlaneIPs(ctx context.Context, creds *x509.PEMEncodedCertificateAndKey, endpoint *url.URL) (client *clientv3.Client, err error) {
	h, err := kubernetes.NewTemporaryClientFromPKI(creds, endpoint)
	if err != nil {
		return nil, err
	}

	var endpoints []string

	if endpoints, err = h.MasterIPs(ctx); err != nil {
		return nil, err
	}

	// Etcd expects host:port format.
	for i := 0; i < len(endpoints); i++ {
		endpoints[i] = net.FormatAddress(endpoints[i]) + ":2379"
	}

	return NewClient(endpoints)
}

// ValidateForUpgrade validates the etcd cluster state to ensure that performing
// an upgrade is safe.
func ValidateForUpgrade(config config.Provider, preserve bool) error {
	if config.Machine().Type() != machine.TypeJoin {
		client, err := NewClientFromControlPlaneIPs(context.TODO(), config.Cluster().CA(), config.Cluster().Endpoint())
		if err != nil {
			return err
		}

		// nolint: errcheck
		defer client.Close()

		resp, err := client.MemberList(context.Background())
		if err != nil {
			return err
		}

		if !preserve {
			if len(resp.Members) == 1 {
				return fmt.Errorf("only 1 etcd member found. assuming this is not an HA setup and refusing to upgrade")
			}
		}

		for _, member := range resp.Members {
			// If the member is not started, the name will be an empty string.
			if len(member.Name) == 0 {
				return fmt.Errorf("etcd member %d is not started, all members must be running to perform an upgrade", member.ID)
			}
		}
	}

	return nil
}
