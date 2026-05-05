// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	cfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

var reservedConfigRulePriorities = map[uint32]struct{}{
	constants.LinuxReservedRulePriorityLocal:   {},
	constants.KubeSpanDefaultRulePriority:      {},
	constants.LinuxReservedRulePriorityMain:    {},
	constants.LinuxReservedRulePriorityDefault: {},
}

// RoutingRuleConfigController manages network.RoutingRuleSpec based on machine configuration.
type RoutingRuleConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *RoutingRuleConfigController) Name() string {
	return "network.RoutingRuleConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RoutingRuleConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RoutingRuleConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.RoutingRuleSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *RoutingRuleConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		machineConfig, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting machine config: %w", err)
			}
		}

		if machineConfig != nil {
			rules := ctrl.processConfig(
				logger,
				machineConfig.Config().NetworkRoutingRuleConfigs(),
			)

			if err = ctrl.apply(ctx, r, rules); err != nil {
				return fmt.Errorf("error applying routing rule config: %w", err)
			}
		}

		if err = r.CleanupOutputs(ctx, resource.NewMetadata(network.ConfigNamespaceName, network.RoutingRuleSpecType, "", resource.VersionUndefined)); err != nil {
			return fmt.Errorf("error cleaning outputs: %w", err)
		}
	}
}

//nolint:dupl
func (ctrl *RoutingRuleConfigController) apply(ctx context.Context, r controller.Runtime, rules []network.RoutingRuleSpecSpec) error {
	for _, rule := range rules {
		id := network.LayeredID(rule.ConfigLayer, network.RoutingRuleID(rule.Family, rule.Priority))

		if err := safe.WriterModify(
			ctx,
			r,
			network.NewRoutingRuleSpec(network.ConfigNamespaceName, id),
			func(res *network.RoutingRuleSpec) error {
				*res.TypedSpec() = rule

				return nil
			},
		); err != nil {
			return err
		}
	}

	return nil
}

func (ctrl *RoutingRuleConfigController) processConfig(
	logger *zap.Logger,
	ruleConfigs []cfg.NetworkRoutingRuleConfig,
) []network.RoutingRuleSpecSpec {
	rules := make([]network.RoutingRuleSpecSpec, 0, len(ruleConfigs))

	for _, ruleCfg := range ruleConfigs {
		var rule network.RoutingRuleSpecSpec

		priority := ruleCfg.Priority()

		if _, reserved := reservedConfigRulePriorities[priority]; reserved {
			logger.Warn(
				"skipping routing rule at reserved priority",
				zap.Uint32("priority", priority),
			)

			continue
		}

		src := ruleCfg.Src().ValueOrZero()
		dst := ruleCfg.Dst().ValueOrZero()

		rule.Src = src
		rule.Dst = dst
		rule.IIFName = ruleCfg.IIFName()
		rule.OIFName = ruleCfg.OIFName()
		rule.FwMark = ruleCfg.FwMark()
		rule.FwMask = ruleCfg.FwMask()
		rule.ConfigLayer = network.ConfigMachineConfiguration
		rule.Priority = priority

		rule.Table = ruleCfg.Table()

		action := ruleCfg.Action()
		if action == nethelpers.RoutingRuleActionUnspec {
			action = nethelpers.RoutingRuleActionUnicast
		}

		rule.Action = action

		for _, family := range ctrl.determineFamily(src, dst) {
			rule.Family = family

			rules = append(rules, rule)
		}
	}

	return rules
}

func (ctrl *RoutingRuleConfigController) determineFamily(src, dst netip.Prefix) []nethelpers.Family {
	if src.IsValid() && src.Addr().Is6() {
		return []nethelpers.Family{nethelpers.FamilyInet6}
	}

	if dst.IsValid() && dst.Addr().Is6() {
		return []nethelpers.Family{nethelpers.FamilyInet6}
	}

	// If both src and dst are invalid, we need to create rules for both families, as we don't know which one will be used.
	if !src.IsValid() && !dst.IsValid() {
		return []nethelpers.Family{nethelpers.FamilyInet6, nethelpers.FamilyInet4}
	}

	return []nethelpers.Family{nethelpers.FamilyInet4}
}
