// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// APIBootstrapper bootstraps cluster via Talos API.
type APIBootstrapper struct {
	ClientProvider
	Info
}

// Bootstrap the cluster via the API.
//
// Bootstrap implements Bootstrapper interface.
//
//nolint:gocyclo
func (s *APIBootstrapper) Bootstrap(ctx context.Context, out io.Writer) error {
	cli, err := s.Client()
	if err != nil {
		return err
	}

	controlPlaneNodes := s.NodesByType(machine.TypeControlPlane)

	if len(controlPlaneNodes) == 0 {
		return fmt.Errorf("no control plane nodes to bootstrap")
	}

	sort.Slice(controlPlaneNodes, func(i, j int) bool {
		return controlPlaneNodes[i].IPs[0].String() < controlPlaneNodes[j].IPs[0].String()
	})

	nodeIP := controlPlaneNodes[0].IPs[0]
	nodeCtx := client.WithNodes(ctx, nodeIP.String())

	fmt.Fprintln(out, "waiting for API")

	err = retry.Constant(10*time.Minute, retry.WithUnits(500*time.Millisecond)).RetryWithContext(nodeCtx, func(nodeCtx context.Context) error {
		retryCtx, cancel := context.WithTimeout(nodeCtx, 2*time.Second)
		defer cancel()

		if _, err = cli.Version(retryCtx); err != nil {
			return retry.ExpectedError(err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "bootstrapping cluster")

	return retry.Constant(backoff.DefaultConfig.MaxDelay, retry.WithUnits(100*time.Millisecond)).RetryWithContext(nodeCtx, func(nodeCtx context.Context) error {
		retryCtx, cancel := context.WithTimeout(nodeCtx, 2*time.Second)
		defer cancel()

		if err = cli.Bootstrap(retryCtx, &machineapi.BootstrapRequest{}); err != nil {
			switch {
			// deadline exceeded in case it's verbatim context error
			case errors.Is(err, context.DeadlineExceeded):
				return retry.ExpectedError(err)
			// FailedPrecondition when time is not in sync yet on the server
			// DeadlineExceeded when the call fails in the gRPC stack either on the server or client side
			case client.StatusCode(err) == codes.FailedPrecondition || client.StatusCode(err) == codes.DeadlineExceeded:
				return retry.ExpectedError(err)
			// connection refused, including proxied connection refused via the endpoint to the node
			case strings.Contains(err.Error(), "connection refused"):
				return retry.ExpectedError(err)
			// connection timeout
			case strings.Contains(err.Error(), "error reading from server: EOF"):
				return retry.ExpectedError(err)
			}

			return err
		}

		return nil
	})
}
