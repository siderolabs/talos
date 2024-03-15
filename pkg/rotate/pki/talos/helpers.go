// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/pkg/cluster"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
)

func mapToInternalIP(in []cluster.NodeInfo) []string {
	return xslices.Map(in, func(i cluster.NodeInfo) string {
		return i.InternalIP.String()
	})
}

func patchNodeConfig(ctx context.Context, c *client.Client, node string, encoderOpt encoder.Option, patchFunc func(config *v1alpha1.Config) error) error {
	return retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond), retry.WithErrorLogging(true)).RetryWithContext(
		ctx,
		func(ctx context.Context) error {
			err := patchNodeConfigInternal(ctx, c, node, encoderOpt, patchFunc)
			if err != nil {
				if client.StatusCode(err) == codes.Unavailable || client.StatusCode(err) == codes.Canceled {
					return retry.ExpectedError(err)
				}
			}

			return err
		},
	)
}

func patchNodeConfigInternal(ctx context.Context, c *client.Client, node string, encoderOpt encoder.Option, patchFunc func(config *v1alpha1.Config) error) error {
	ctx = client.WithNode(ctx, node)

	mc, err := safe.StateGetByID[*configres.MachineConfig](ctx, c.COSI, configres.V1Alpha1ID)
	if err != nil {
		return fmt.Errorf("error fetching config resource: %w", err)
	}

	provider := mc.Provider()

	newProvider, err := provider.PatchV1Alpha1(patchFunc)
	if err != nil {
		return fmt.Errorf("error patching config: %w", err)
	}

	cfgBytes, err := newProvider.EncodeBytes(encoderOpt)
	if err != nil {
		return fmt.Errorf("error serializing config: %w", err)
	}

	_, err = c.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
		Data: cfgBytes,
		Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
	})
	if err != nil {
		return fmt.Errorf("error applying config: %w", err)
	}

	return nil
}
