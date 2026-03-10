// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// RoutingRuleStatusController observes kernel routing rules and publishes them as resources.
type RoutingRuleStatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *RoutingRuleStatusController) Name() string {
	return "network.RoutingRuleStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RoutingRuleStatusController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *RoutingRuleStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.RoutingRuleStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *RoutingRuleStatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	// watch link changes as some routes might need to be re-applied if the link appears
	watcher, err := watch.NewRtNetlink(watch.NewDefaultRateLimitedTrigger(ctx, r), unix.RTMGRP_IPV4_RULE,
		unix.RTNLGRP_IPV4_RULE, unix.RTNLGRP_IPV6_RULE)
	if err != nil {
		return err
	}

	defer watcher.Done()

	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("error dialing rtnetlink socket: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		rules, err := conn.Rule.List()
		if err != nil {
			return fmt.Errorf("error listing kernel rules: %w", err)
		}

		for _, rule := range rules {
			family := nethelpers.Family(rule.Family)

			var src netip.Prefix

			if rule.Attributes.Src != nil {
				srcAddr, ok := netip.AddrFromSlice(*rule.Attributes.Src)
				if ok {
					src = netip.PrefixFrom(srcAddr, int(rule.SrcLength))
				}
			}

			var dst netip.Prefix

			if rule.Attributes.Dst != nil {
				dstAddr, ok := netip.AddrFromSlice(*rule.Attributes.Dst)
				if ok {
					dst = netip.PrefixFrom(dstAddr, int(rule.DstLength))
				}
			}

			priority := pointer.SafeDeref(rule.Attributes.Priority)

			table := uint32(rule.Table)
			if rule.Attributes.Table != nil {
				table = *rule.Attributes.Table
			}

			id := network.RoutingRuleID(family, priority)

			if err = safe.WriterModify(ctx, r, network.NewRoutingRuleStatus(network.NamespaceName, id), func(res *network.RoutingRuleStatus) error {
				status := res.TypedSpec()

				status.Family = family
				status.Src = src
				status.Dst = dst
				status.Table = nethelpers.RoutingTable(table)
				status.Priority = priority
				status.Action = nethelpers.RoutingRuleAction(rule.Action)
				status.IIFName = pointer.SafeDeref(rule.Attributes.IIFName)
				status.OIFName = pointer.SafeDeref(rule.Attributes.OIFName)
				status.FwMark = pointer.SafeDeref(rule.Attributes.FwMark)
				status.FwMask = pointer.SafeDeref(rule.Attributes.FwMask)

				return nil
			}); err != nil {
				return fmt.Errorf("error modifying resource: %w", err)
			}
		}

		if err := safe.CleanupOutputs[*network.RoutingRuleStatus](ctx, r); err != nil {
			return fmt.Errorf("error doing cleanup: %w", err)
		}
	}
}
