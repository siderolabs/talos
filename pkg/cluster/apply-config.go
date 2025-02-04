// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/siderolabs/go-retry/retry"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/provision"
)

// ApplyConfigClient client to apply config.
type ApplyConfigClient struct {
	ClientProvider
	Info
}

// NodeAddress is the address of the node.
type NodeAddress struct {
	IP   netip.Addr
	UUID *uuid.UUID
}

// NodeApplyConfig is the config to be applied to the node.
type NodeApplyConfig struct {
	NodeAddress NodeAddress
	Config      config.Provider
}

// ApplyConfig on the node via the API using insecure mode. If UUID is set attempts to apply via SideroLink.
func (s *APIBootstrapper) ApplyConfig(ctx context.Context, nodes []NodeApplyConfig, sl *provision.SiderolinkRequest, out io.Writer) error {
	for _, node := range nodes {
		configureNode := func() error {
			ep := node.NodeAddress.IP.String()

			if sl != nil {
				if addr, ok := sl.GetAddr(node.NodeAddress.UUID); ok {
					fmt.Fprintln(out, "using SideroLink node address for 'with-apply-config'", node.NodeAddress.UUID, "=", addr.String())

					ep = addr.String()
				}
			}

			c, err := client.New(ctx, client.WithTLSConfig(&tls.Config{
				InsecureSkipVerify: true,
			}), client.WithEndpoints(ep))
			if err != nil {
				return err
			}

			cfgBytes, err := node.Config.Bytes()
			if err != nil {
				return err
			}

			_, err = c.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
				Data: cfgBytes,
			})
			if err != nil {
				return retry.ExpectedError(err)
			}

			return nil
		}

		if err := retry.Constant(2*time.Minute, retry.WithUnits(250*time.Millisecond), retry.WithJitter(50*time.Millisecond)).Retry(configureNode); err != nil {
			return err
		}
	}

	return nil
}
