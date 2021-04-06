// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/talos-systems/go-retry/retry"
	"google.golang.org/grpc/backoff"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// APIBootstrapper bootstraps cluster via Talos API.
type APIBootstrapper struct {
	ClientProvider
	Info
}

// Bootstrap the cluster via the API.
//
// Bootstrap implements Bootstrapper interface.
func (s *APIBootstrapper) Bootstrap(ctx context.Context, out io.Writer) error {
	cli, err := s.Client()
	if err != nil {
		return err
	}

	controlPlaneNodes := s.NodesByType(machine.TypeControlPlane)
	if len(controlPlaneNodes) == 0 {
		return fmt.Errorf("no control plane nodes to bootstrap")
	}

	sort.Strings(controlPlaneNodes)

	node := controlPlaneNodes[0]
	nodeCtx := client.WithNodes(ctx, node)

	fmt.Fprintln(out, "waiting for API")

	err = retry.Constant(5*time.Minute, retry.WithUnits(500*time.Millisecond)).Retry(func() error {
		retryCtx, cancel := context.WithTimeout(nodeCtx, 500*time.Millisecond)
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

	return retry.Constant(backoff.DefaultConfig.MaxDelay, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		retryCtx, cancel := context.WithTimeout(nodeCtx, 500*time.Millisecond)
		defer cancel()

		if err = cli.Bootstrap(retryCtx, &machineapi.BootstrapRequest{}); err != nil {
			if strings.Contains(err.Error(), "connection refused") {
				return retry.ExpectedError(err)
			}

			return retry.UnexpectedError(err)
		}

		return nil
	})
}
