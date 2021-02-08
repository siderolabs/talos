// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"crypto/tls"
	"io"
	"time"

	"github.com/talos-systems/go-retry/retry"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/provision"
)

// ApplyConfigClient client to apply config.
type ApplyConfigClient struct {
	ClientProvider
	Info
}

// ApplyConfig on the node via the API using insecure mode.
func (s *APIBootstrapper) ApplyConfig(ctx context.Context, nodes []provision.NodeRequest, out io.Writer) error {
	for _, node := range nodes {
		n := node

		configureNode := func() error {
			c, err := client.New(ctx, client.WithTLSConfig(&tls.Config{
				InsecureSkipVerify: true,
			}), client.WithEndpoints(n.IPs[0].String()))
			if err != nil {
				return retry.UnexpectedError(err)
			}

			cfgBytes, err := n.Config.Bytes()
			if err != nil {
				return retry.UnexpectedError(err)
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
