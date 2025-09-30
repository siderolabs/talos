// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"time"

	"github.com/siderolabs/go-retry/retry"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/provision"
)

// ApplyConfigClient client to apply config.
type ApplyConfigClient struct {
	ClientProvider
	Info
}

// ApplyConfig on the node via the API using insecure mode.
func (s *APIBootstrapper) ApplyConfig(ctx context.Context, nodes []provision.NodeRequest, sl provision.SiderolinkRequest, out io.Writer) error {
	for _, node := range nodes {
		configureNode := func(ctx context.Context) error {
			ep := node.IPs[0].String()

			if addr, ok := sl.GetAddr(node.UUID); ok {
				fmt.Fprintln(out, "using SideroLink node address for 'with-apply-config'", node.UUID, "=", addr.String())

				ep = addr.String()
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
		if err := retry.Constant(2*time.Minute, retry.WithUnits(250*time.Millisecond), retry.WithJitter(50*time.Millisecond)).RetryWithContext(ctx, configureNode); err != nil {
			return err
		}
	}

	return nil
}
