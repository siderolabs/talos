// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/hashicorp/go-multierror"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// RoutingRuleSpecController applies network.RoutingRuleSpec to the kernel.
type RoutingRuleSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *RoutingRuleSpecController) Name() string {
	return "network.RoutingRuleSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RoutingRuleSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.RoutingRuleSpecType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RoutingRuleSpecController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *RoutingRuleSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// watch link changes as some routes might need to be re-applied if the link appears
	watcher, err := watch.NewRtNetlink(watch.NewDefaultRateLimitedTrigger(ctx, r), unix.RTMGRP_IPV4_RULE)
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

		// list source network configuration resources
		list, err := safe.ReaderListAll[*network.RoutingRuleSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing source routing rules: %w", err)
		}

		// add finalizers for all live resources
		for res := range list.All() {
			if res.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if err = r.AddFinalizer(ctx, res.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer: %w", err)
			}
		}

		// list existing kernel rules
		existingRules, err := conn.Rule.List()
		if err != nil {
			return fmt.Errorf("error listing kernel rules: %w", err)
		}

		var multiErr *multierror.Error

		// loop over rules and make reconcile decision
		for rule := range list.All() {
			if err = ctrl.syncRule(ctx, r, logger, conn, existingRules, rule); err != nil {
				multiErr = multierror.Append(multiErr, err)
			}
		}

		if err = multiErr.ErrorOrNil(); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo,cyclop
func (ctrl *RoutingRuleSpecController) syncRule(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	conn *rtnetlink.Conn,
	existingRules []rtnetlink.RuleMessage,
	rule *network.RoutingRuleSpec,
) error {
	spec := rule.TypedSpec()

	switch rule.Metadata().Phase() {
	case resource.PhaseTearingDown:
		for i := range existingRules {
			if ctrl.matchesRuleKey(&existingRules[i], spec) {
				if err := conn.Rule.Delete(&existingRules[i]); err != nil {
					if !errors.Is(err, os.ErrNotExist) {
						return fmt.Errorf("error removing routing rule: %w", err)
					}
				}

				logger.Info("deleted routing rule",
					zap.Uint8("family", existingRules[i].Family),
					zap.Uint8("table", existingRules[i].Table),
					zap.Uint8("action", existingRules[i].Action),
					zap.Uint32("priority", pointer.SafeDeref(existingRules[i].Attributes.Priority)),
					zap.Stringer("src", pointer.SafeDeref(existingRules[i].Attributes.Src)),
					zap.Stringer("dst", pointer.SafeDeref(existingRules[i].Attributes.Dst)),
					zap.String("iif", pointer.SafeDeref(existingRules[i].Attributes.IIFName)),
					zap.String("oif", pointer.SafeDeref(existingRules[i].Attributes.OIFName)),
					zap.Uint32("fwmark", pointer.SafeDeref(existingRules[i].Attributes.FwMark)),
					zap.Uint32("fwmask", pointer.SafeDeref(existingRules[i].Attributes.FwMask)),
				)
			}
		}

		// remove finalizer
		if err := r.RemoveFinalizer(ctx, rule.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error removing finalizer: %w", err)
		}

	case resource.PhaseRunning:
		existingIdx := []int{}

		for i := range existingRules {
			// find rules that match the unique key but differ in other attributes - these need to be deleted and re-created to update
			if ctrl.matchesRuleKey(&existingRules[i], spec) && !ctrl.matchesRule(&existingRules[i], spec) {
				existingIdx = append(existingIdx, i)
			}
		}

		msg := ctrl.buildRuleMessage(spec)

		for _, idx := range existingIdx {
			if err := conn.Rule.Delete(&existingRules[idx]); err != nil {
				return fmt.Errorf("error deleting routing rule during update: %w, spec %+v", err, *spec)
			}
		}

		if err := conn.Rule.Add(msg); err != nil {
			// If the rule already exists, it means there was no change in attributes and we can ignore the error.
			if !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("error adding routing rule: %w, spec %+v", err, *spec)
			}

			return nil
		}

		action := "created"
		if len(existingIdx) > 0 {
			action = "replaced"
		}

		logger.Info(action+" routing rule",
			zap.Stringer("family", spec.Family),
			zap.Stringer("src", spec.Src),
			zap.Stringer("dst", spec.Dst),
			zap.Stringer("table", spec.Table),
			zap.Uint32("priority", spec.Priority),
			zap.Stringer("action", spec.Action),
			zap.String("iif", spec.IIFName),
			zap.String("oif", spec.OIFName),
			zap.Uint32("fwmark", spec.FwMark),
			zap.Uint32("fwmask", spec.FwMask),
		)
	}

	return nil
}

//nolint:gocyclo
func (ctrl *RoutingRuleSpecController) matchesRule(existing *rtnetlink.RuleMessage, spec *network.RoutingRuleSpecSpec) bool {
	// Compare priority only - this is the unique key
	existingPriority := pointer.SafeDeref(existing.Attributes.Priority)
	if existingPriority != spec.Priority {
		return false
	}

	// Compare family
	if existing.Family != uint8(spec.Family) {
		return false
	}

	// Compare action
	if existing.Action != uint8(spec.Action) {
		return false
	}

	// compare table
	existingTable := uint32(existing.Table)
	if existing.Attributes.Table != nil {
		existingTable = *existing.Attributes.Table
	}

	if existingTable != uint32(spec.Table) {
		return false
	}

	// compare src
	if !ctrl.matchesPrefix(existing.Attributes.Src, existing.SrcLength, spec.Src, spec.Family) {
		return false
	}

	// compare dst
	if !ctrl.matchesPrefix(existing.Attributes.Dst, existing.DstLength, spec.Dst, spec.Family) {
		return false
	}

	// compare iif/oif
	if pointer.SafeDeref(existing.Attributes.IIFName) != spec.IIFName {
		return false
	}

	if pointer.SafeDeref(existing.Attributes.OIFName) != spec.OIFName {
		return false
	}

	// compare fwmark/fwmask
	if pointer.SafeDeref(existing.Attributes.FwMark) != spec.FwMark {
		return false
	}

	if pointer.SafeDeref(existing.Attributes.FwMask) != spec.FwMask {
		return false
	}

	return true
}

//nolint:gocyclo
func (ctrl *RoutingRuleSpecController) matchesPrefix(existingIP *net.IP, existingLen uint8, specPrefix netip.Prefix, family nethelpers.Family) bool {
	if specPrefix.IsValid() {
		if family == nethelpers.FamilyInet4 && !specPrefix.Addr().Is4() {
			return false
		}

		if family == nethelpers.FamilyInet6 && !specPrefix.Addr().Is6() {
			return false
		}
	}

	if !specPrefix.IsValid() || specPrefix.Bits() == 0 {
		return existingLen == 0
	}

	if existingLen != uint8(specPrefix.Bits()) {
		return false
	}

	if existingIP == nil {
		return false
	}

	existingAddr, ok := netip.AddrFromSlice(*existingIP)
	if !ok {
		return false
	}

	return existingAddr == specPrefix.Addr()
}

func (ctrl *RoutingRuleSpecController) matchesRuleKey(existing *rtnetlink.RuleMessage, spec *network.RoutingRuleSpecSpec) bool {
	if pointer.SafeDeref(existing.Attributes.Priority) != spec.Priority {
		return false
	}

	if existing.Family != uint8(spec.Family) {
		return false
	}

	return true
}

func (ctrl *RoutingRuleSpecController) buildRuleMessage(spec *network.RoutingRuleSpecSpec) *rtnetlink.RuleMessage {
	msg := &rtnetlink.RuleMessage{
		Family: uint8(spec.Family),
		Table:  uint8(spec.Table),
		Action: uint8(spec.Action),
		Attributes: &rtnetlink.RuleAttributes{
			Priority: new(spec.Priority),
			Table:    new(uint32(spec.Table)),
		},
	}

	if spec.Src.IsValid() && spec.Src.Bits() > 0 {
		msg.SrcLength = uint8(spec.Src.Bits())

		srcIP := net.IP(spec.Src.Addr().AsSlice())
		msg.Attributes.Src = &srcIP
	}

	if spec.Dst.IsValid() && spec.Dst.Bits() > 0 {
		msg.DstLength = uint8(spec.Dst.Bits())

		dstIP := net.IP(spec.Dst.Addr().AsSlice())
		msg.Attributes.Dst = &dstIP
	}

	if spec.IIFName != "" {
		msg.Attributes.IIFName = new(spec.IIFName)
	}

	if spec.OIFName != "" {
		msg.Attributes.OIFName = new(spec.OIFName)
	}

	if spec.FwMark != 0 {
		msg.Attributes.FwMark = new(spec.FwMark)
	}

	if spec.FwMask != 0 {
		msg.Attributes.FwMask = new(spec.FwMask)
	}

	// set protocol to indicate this rule was created by us
	proto := uint8(unix.RTPROT_STATIC)
	msg.Attributes.Protocol = &proto

	return msg
}
