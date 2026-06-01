// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// StaticHostController translates machine configuration ExtraHostEntries and the
// local node's hostname/addresses into network.StaticHost resources consumed by
// the in-process DNS server.
type StaticHostController struct{}

// Name implements controller.Controller interface.
func (ctrl *StaticHostController) Name() string {
	return "network.StaticHostController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StaticHostController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        optional.Some(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        optional.Some(network.NodeAddressCurrentID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *StaticHostController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.StaticHostType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *StaticHostController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r); err != nil {
			return err
		}
	}
}

//nolint:gocyclo
func (ctrl *StaticHostController) reconcile(ctx context.Context, r controller.Runtime) error {
	r.StartTrackingOutputs()

	hosts := map[string][]netip.Addr{}

	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting machine config: %w", err)
	}

	if cfg != nil {
		for _, entry := range cfg.Config().NetworkStaticHostConfig() {
			addr, parseErr := netip.ParseAddr(entry.IP())
			if parseErr != nil {
				// skip invalid entries; etcfile rendering accepts the raw string,
				// but DNS needs a parseable address
				continue
			}

			for _, alias := range entry.Aliases() {
				name := normalizeHostName(alias)
				if name == "" {
					continue
				}

				hosts[name] = append(hosts[name], addr)
			}
		}
	}

	hostnameStatus, err := safe.ReaderGetByID[*network.HostnameStatus](ctx, r, network.HostnameID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting hostname status: %w", err)
	}

	nodeAddresses, err := safe.ReaderGetByID[*network.NodeAddress](ctx, r, network.NodeAddressCurrentID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting node addresses: %w", err)
	}

	if hostnameStatus != nil && nodeAddresses != nil {
		names := []string{
			normalizeHostName(hostnameStatus.TypedSpec().Hostname),
			normalizeHostName(hostnameStatus.TypedSpec().FQDN()),
		}

		for _, name := range names {
			if name == "" {
				continue
			}

			hosts[name] = append(hosts[name], nodeAddresses.TypedSpec().IPs()...)
		}
	}

	for name, addrs := range hosts {
		addrs = dedupSortAddrs(addrs)

		if err := safe.WriterModify(ctx, r, network.NewStaticHost(network.NamespaceName, name), func(res *network.StaticHost) error {
			res.TypedSpec().Addresses = addrs

			return nil
		}); err != nil {
			return fmt.Errorf("error writing static host %q: %w", name, err)
		}
	}

	if err := r.CleanupOutputs(ctx, resource.NewMetadata(network.NamespaceName, network.StaticHostType, "", resource.VersionUndefined)); err != nil {
		return fmt.Errorf("error cleaning up static hosts: %w", err)
	}

	return nil
}

// normalizeHostName lowercases the name and strips a trailing dot.
//
// DNS host names are case-insensitive; storing them in a canonical lowercase
// form lets the DNS handler match queries without per-lookup normalization.
func normalizeHostName(name string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(name), "."))
}

func dedupSortAddrs(addrs []netip.Addr) []netip.Addr {
	slices.SortFunc(addrs, func(a, b netip.Addr) int { return a.Compare(b) })

	return slices.Compact(addrs)
}
