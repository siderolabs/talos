// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package helpers provides helper functions for the rotate/pki package.
package helpers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/pkg/cluster"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	v1alpha1res "github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// MapToInternalIP maps a slice of NodeInfo to a slice of internal IPs.
func MapToInternalIP(in []cluster.NodeInfo) []string {
	return xslices.Map(in, func(i cluster.NodeInfo) string {
		return i.InternalIP.String()
	})
}

// PatchNodeConfig patches the node config for the given node.
func PatchNodeConfig(ctx context.Context, c *client.Client, node string, encoderOpt encoder.Option, patchFunc func(config *v1alpha1.Config) error) error {
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

// PatchNodeConfigWithKubeletRestart patches the node config for the given node waiting for the kubelet to be restarted.
//
//nolint:gocyclo,cyclop
func PatchNodeConfigWithKubeletRestart(ctx context.Context, c *client.Client, node string, encoderOpt encoder.Option, patchFunc func(config *v1alpha1.Config) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx = client.WithNode(ctx, node)

	watchCh := make(chan safe.WrappedStateEvent[*v1alpha1res.Service])

	if err := safe.StateWatch(ctx, c.COSI, resource.NewMetadata(v1alpha1res.NamespaceName, v1alpha1res.ServiceType, "kubelet", resource.VersionUndefined), watchCh); err != nil {
		return fmt.Errorf("error watching service: %w", err)
	}

	var ev safe.WrappedStateEvent[*v1alpha1res.Service]

	select {
	case ev = <-watchCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	if ev.Type() != state.Created {
		return fmt.Errorf("unexpected event type: %s", ev.Type())
	}

	initialService, err := ev.Resource()
	if err != nil {
		return fmt.Errorf("error inspecting service: %w", err)
	}

	if !initialService.TypedSpec().Running || !initialService.TypedSpec().Healthy {
		return errors.New("kubelet is not healthy")
	}

	if err = PatchNodeConfig(ctx, c, node, encoderOpt, patchFunc); err != nil {
		return fmt.Errorf("error patching node config: %w", err)
	}

	// first, wait for kubelet to go down
	for {
		select {
		case ev = <-watchCh:
		case <-ctx.Done():
			return ctx.Err()
		}

		if ev.Type() == state.Destroyed {
			break
		}
	}

	// now wait for kubelet to go up & healthy
	for {
		select {
		case ev = <-watchCh:
		case <-ctx.Done():
			return ctx.Err()
		}

		if ev.Type() == state.Created || ev.Type() == state.Updated {
			var service *v1alpha1res.Service

			service, err = ev.Resource()
			if err != nil {
				return fmt.Errorf("error inspecting service: %w", err)
			}

			if service.TypedSpec().Running && service.TypedSpec().Healthy {
				break
			}
		}
	}

	return nil
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
