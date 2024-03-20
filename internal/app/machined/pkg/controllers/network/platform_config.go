// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// Virtual link name for external IPs.
const externalLink = "external"

// PlatformConfigController manages updates hostnames and addressstatuses based on platform information.
type PlatformConfigController struct {
	V1alpha1Platform v1alpha1runtime.Platform
	PlatformState    state.State
	StatePath        string
}

// Name implements controller.Controller interface.
func (ctrl *PlatformConfigController) Name() string {
	return "network.PlatformConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PlatformConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      runtimeres.MountStatusType,
			ID:        optional.Some(constants.StatePartitionLabel),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *PlatformConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.AddressSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.LinkSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.RouteSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.HostnameSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.ResolverSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.TimeServerSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.AddressStatusType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.OperatorSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.ProbeSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: runtimeres.PlatformMetadataType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *PlatformConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.StatePath == "" {
		ctrl.StatePath = constants.StateMountPoint
	}

	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	if ctrl.V1alpha1Platform == nil {
		// no platform, no work to be done
		return nil
	}

	platformCtx, platformCtxCancel := context.WithCancel(ctx)
	defer platformCtxCancel()

	platformCh := make(chan *v1alpha1runtime.PlatformNetworkConfig, 1)

	var platformWg sync.WaitGroup

	platformWg.Add(1)

	go func() {
		defer platformWg.Done()

		ctrl.runWithRestarts(platformCtx, logger, func() error {
			return ctrl.V1alpha1Platform.NetworkConfiguration(platformCtx, ctrl.PlatformState, platformCh)
		})
	}()

	defer platformWg.Wait()

	r.QueueReconcile()

	var cachedNetworkConfig, networkConfig *v1alpha1runtime.PlatformNetworkConfig

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case networkConfig = <-platformCh:
		}

		var stateMounted bool

		if _, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, runtimeres.MountStatusType, constants.StatePartitionLabel, resource.VersionUndefined)); err == nil {
			stateMounted = true
		} else {
			if state.IsNotFoundError(err) {
				// in container mode STATE is always mounted
				if ctrl.V1alpha1Platform.Mode() == v1alpha1runtime.ModeContainer {
					stateMounted = true
				}
			} else {
				return fmt.Errorf("error reading mount status: %w", err)
			}
		}

		if stateMounted && cachedNetworkConfig == nil {
			var err error

			cachedNetworkConfig, err = ctrl.loadConfig(filepath.Join(ctrl.StatePath, constants.PlatformNetworkConfigFilename))
			if err != nil {
				logger.Warn("ignored failure loading cached platform network config", zap.Error(err))
			} else if cachedNetworkConfig != nil {
				logger.Debug("loaded cached platform network config")
			}
		}

		if stateMounted && networkConfig != nil {
			if err := ctrl.storeConfig(filepath.Join(ctrl.StatePath, constants.PlatformNetworkConfigFilename), networkConfig); err != nil {
				return fmt.Errorf("error saving platform network config: %w", err)
			}

			logger.Debug("stored cached platform network config")

			cachedNetworkConfig = networkConfig
		}

		switch {
		// prefer live network config over cached config always
		case networkConfig != nil:
			if err := ctrl.apply(ctx, r, networkConfig); err != nil {
				return err
			}
		// cached network is only used as last resort
		case cachedNetworkConfig != nil:
			if err := ctrl.apply(ctx, r, cachedNetworkConfig); err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

//nolint:dupl,gocyclo
func (ctrl *PlatformConfigController) apply(ctx context.Context, r controller.Runtime, networkConfig *v1alpha1runtime.PlatformNetworkConfig) error {
	metadataLength := 0

	if networkConfig.Metadata != nil {
		metadataLength = 1
	}

	// handle all network specs in a loop as all specs can be handled in a similar way
	for _, specType := range []struct {
		length           int
		getter           func(i int) interface{}
		idBuilder        func(spec interface{}) (resource.ID, error)
		resourceBuilder  func(id string) resource.Resource
		resourceModifier func(newSpec interface{}) func(r resource.Resource) error
	}{
		// AddressSpec
		{
			length: len(networkConfig.Addresses),
			getter: func(i int) interface{} {
				return networkConfig.Addresses[i]
			},
			idBuilder: func(spec interface{}) (resource.ID, error) {
				addressSpec := spec.(network.AddressSpecSpec) //nolint:errcheck,forcetypeassert

				return network.LayeredID(network.ConfigPlatform, network.AddressID(addressSpec.LinkName, addressSpec.Address)), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewAddressSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec interface{}) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.AddressSpec).TypedSpec()

					*spec = newSpec.(network.AddressSpecSpec) //nolint:errcheck,forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// LinkSpec
		{
			length: len(networkConfig.Links),
			getter: func(i int) interface{} {
				return networkConfig.Links[i]
			},
			idBuilder: func(spec interface{}) (resource.ID, error) {
				linkSpec := spec.(network.LinkSpecSpec) //nolint:errcheck,forcetypeassert

				return network.LayeredID(network.ConfigPlatform, network.LinkID(linkSpec.Name)), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewLinkSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec interface{}) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.LinkSpec).TypedSpec()

					*spec = newSpec.(network.LinkSpecSpec) //nolint:errcheck,forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// RouteSpec
		{
			length: len(networkConfig.Routes),
			getter: func(i int) interface{} {
				return networkConfig.Routes[i]
			},
			idBuilder: func(spec interface{}) (resource.ID, error) {
				routeSpec := spec.(network.RouteSpecSpec) //nolint:errcheck,forcetypeassert

				return network.LayeredID(
					network.ConfigPlatform,
					network.RouteID(routeSpec.Table, routeSpec.Family, routeSpec.Destination, routeSpec.Gateway, routeSpec.Priority, routeSpec.OutLinkName),
				), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewRouteSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec interface{}) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.RouteSpec).TypedSpec()

					*spec = newSpec.(network.RouteSpecSpec) //nolint:errcheck,forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// HostnameSpec
		{
			length: len(networkConfig.Hostnames),
			getter: func(i int) interface{} {
				return networkConfig.Hostnames[i]
			},
			idBuilder: func(spec interface{}) (resource.ID, error) {
				return network.LayeredID(network.ConfigPlatform, network.HostnameID), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewHostnameSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec interface{}) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.HostnameSpec).TypedSpec()

					*spec = newSpec.(network.HostnameSpecSpec) //nolint:errcheck,forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// ResolverSpec
		{
			length: len(networkConfig.Resolvers),
			getter: func(i int) interface{} {
				return networkConfig.Resolvers[i]
			},
			idBuilder: func(spec interface{}) (resource.ID, error) {
				return network.LayeredID(network.ConfigPlatform, network.ResolverID), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewResolverSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec interface{}) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.ResolverSpec).TypedSpec()

					*spec = newSpec.(network.ResolverSpecSpec) //nolint:errcheck,forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// TimeServerSpec
		{
			length: len(networkConfig.TimeServers),
			getter: func(i int) interface{} {
				return networkConfig.TimeServers[i]
			},
			idBuilder: func(spec interface{}) (resource.ID, error) {
				return network.LayeredID(network.ConfigPlatform, network.TimeServerID), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewTimeServerSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec interface{}) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.TimeServerSpec).TypedSpec()

					*spec = newSpec.(network.TimeServerSpecSpec) //nolint:errcheck,forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// OperatorSpec
		{
			length: len(networkConfig.Operators),
			getter: func(i int) interface{} {
				return networkConfig.Operators[i]
			},
			idBuilder: func(spec interface{}) (resource.ID, error) {
				operatorSpec := spec.(network.OperatorSpecSpec) //nolint:errcheck,forcetypeassert

				return network.LayeredID(network.ConfigPlatform, network.OperatorID(operatorSpec.Operator, operatorSpec.LinkName)), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewOperatorSpec(network.ConfigNamespaceName, id)
			},
			resourceModifier: func(newSpec interface{}) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.OperatorSpec).TypedSpec()

					*spec = newSpec.(network.OperatorSpecSpec) //nolint:errcheck,forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// ExternalIPs
		{
			length: len(networkConfig.ExternalIPs),
			getter: func(i int) interface{} {
				return networkConfig.ExternalIPs[i]
			},
			idBuilder: func(spec interface{}) (resource.ID, error) {
				ipAddr := spec.(netip.Addr) //nolint:errcheck,forcetypeassert
				ipPrefix := netip.PrefixFrom(ipAddr, ipAddr.BitLen())

				return network.AddressID(externalLink, ipPrefix), nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewAddressStatus(network.NamespaceName, id)
			},
			resourceModifier: func(newSpec interface{}) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					ipAddr := newSpec.(netip.Addr) //nolint:errcheck,forcetypeassert
					ipPrefix := netip.PrefixFrom(ipAddr, ipAddr.BitLen())

					status := r.(*network.AddressStatus).TypedSpec()

					status.Address = ipPrefix
					status.LinkName = externalLink

					if ipAddr.Is4() {
						status.Family = nethelpers.FamilyInet4
					} else {
						status.Family = nethelpers.FamilyInet6
					}

					status.Scope = nethelpers.ScopeGlobal

					return nil
				}
			},
		},
		// ProbeSpec
		{
			length: len(networkConfig.Probes),
			getter: func(i int) interface{} {
				return networkConfig.Probes[i]
			},
			idBuilder: func(spec interface{}) (resource.ID, error) {
				probeSpec := spec.(network.ProbeSpecSpec) //nolint:errcheck,forcetypeassert

				return probeSpec.ID()
			},
			resourceBuilder: func(id string) resource.Resource {
				return network.NewProbeSpec(network.NamespaceName, id)
			},
			resourceModifier: func(newSpec interface{}) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					spec := r.(*network.ProbeSpec).TypedSpec()

					*spec = newSpec.(network.ProbeSpecSpec) //nolint:errcheck,forcetypeassert
					spec.ConfigLayer = network.ConfigPlatform

					return nil
				}
			},
		},
		// Platform metadata
		{
			length: metadataLength,
			getter: func(i int) interface{} {
				return networkConfig.Metadata
			},
			idBuilder: func(spec interface{}) (resource.ID, error) {
				return runtimeres.PlatformMetadataID, nil
			},
			resourceBuilder: func(id string) resource.Resource {
				return runtimeres.NewPlatformMetadataSpec(runtimeres.NamespaceName, id)
			},
			resourceModifier: func(newSpec interface{}) func(r resource.Resource) error {
				return func(r resource.Resource) error {
					metadata := newSpec.(*runtimeres.PlatformMetadataSpec) //nolint:errcheck,forcetypeassert

					*r.(*runtimeres.PlatformMetadata).TypedSpec() = *metadata

					return nil
				}
			},
		},
	} {
		touchedIDs := make(map[resource.ID]struct{}, specType.length)

		resourceEmpty := specType.resourceBuilder("")
		resourceNamespace := resourceEmpty.Metadata().Namespace()
		resourceType := resourceEmpty.Metadata().Type()

		for i := range specType.length {
			spec := specType.getter(i)

			id, err := specType.idBuilder(spec)
			if err != nil {
				return fmt.Errorf("error building resource %s ID: %w", resourceType, err)
			}

			if err = r.Modify(ctx, specType.resourceBuilder(id), specType.resourceModifier(spec)); err != nil {
				return fmt.Errorf("error modifying resource %s: %w", resourceType, err)
			}

			touchedIDs[id] = struct{}{}
		}

		list, err := r.List(ctx, resource.NewMetadata(resourceNamespace, resourceType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if res.Metadata().Owner() != ctrl.Name() {
				continue
			}

			if _, ok := touchedIDs[res.Metadata().ID()]; ok {
				continue
			}

			if err = r.Destroy(ctx, res.Metadata()); err != nil {
				return fmt.Errorf("error deleting %s: %w", res, err)
			}
		}
	}

	return nil
}

func (ctrl *PlatformConfigController) runWithRestarts(ctx context.Context, logger *zap.Logger, f func() error) {
	backoff := backoff.NewExponentialBackOff()

	// disable number of retries limit
	backoff.MaxElapsedTime = 0

	for ctx.Err() == nil {
		var err error

		if err = ctrl.runWithPanicHandler(logger, f); err == nil {
			// operator finished without an error
			return
		}

		// skip restarting if context is already done
		select {
		case <-ctx.Done():
			return
		default:
		}

		interval := backoff.NextBackOff()

		logger.Error("restarting platform network config", zap.Duration("interval", interval), zap.Error(err))

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (ctrl *PlatformConfigController) runWithPanicHandler(logger *zap.Logger, f func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)

			logger.Error("platform panicked", zap.Stack("stack"), zap.Error(err))
		}
	}()

	err = f()

	return
}

func (ctrl *PlatformConfigController) loadConfig(path string) (*v1alpha1runtime.PlatformNetworkConfig, error) {
	marshaled, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var networkConfig v1alpha1runtime.PlatformNetworkConfig

	if err = yaml.Unmarshal(marshaled, &networkConfig); err != nil {
		return nil, fmt.Errorf("error unmarshaling network config: %w", err)
	}

	return &networkConfig, nil
}

func (ctrl *PlatformConfigController) storeConfig(path string, networkConfig *v1alpha1runtime.PlatformNetworkConfig) error {
	marshaled, err := yaml.Marshal(networkConfig)
	if err != nil {
		return fmt.Errorf("error marshaling network config: %w", err)
	}

	if _, err := os.Stat(path); err == nil {
		existing, err := os.ReadFile(path)
		if err == nil && bytes.Equal(marshaled, existing) {
			// existing contents are identical, skip writing to avoid no-op writes
			return nil
		}
	}

	return os.WriteFile(path, marshaled, 0o400)
}
