// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/siderolabs/go-pointer"
	"golang.org/x/sys/unix"
)

// RulesManager manages routing rules outside of controllers/resources scope.
//
// TODO: this might be refactored later to support routing rules in the native network resources.
type RulesManager interface {
	Install() error
	Cleanup() error
}

// NewRulesManager initializes new RulesManager.
func NewRulesManager(targetTable uint8, internalMark, markMask uint32) RulesManager {
	return &rulesManager{
		TargetTable:  targetTable,
		InternalMark: internalMark,
		MarkMask:     markMask,
	}
}

type rulesManager struct {
	TargetTable  uint8
	InternalMark uint32
	MarkMask     uint32
}

// Install routing rules.
func (m *rulesManager) Install() error {
	nc, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("failed to get netlink handle: %w", err)
	}

	defer nc.Close() //nolint:errcheck

	if err := nc.Rule.Add(&rtnetlink.RuleMessage{
		Family: unix.AF_INET,
		Table:  m.TargetTable,
		Action: unix.RTN_UNICAST,
		Attributes: &rtnetlink.RuleAttributes{
			FwMark:   pointer.To(m.InternalMark),
			FwMask:   pointer.To(m.MarkMask),
			Priority: pointer.To(nextRuleNumber(nc, unix.AF_INET)),
		},
	}); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("failed to add IPv4 table-mark rule: %w", err)
		}
	}

	if err := nc.Rule.Add(&rtnetlink.RuleMessage{
		Family: unix.AF_INET6,
		Table:  m.TargetTable,
		Action: unix.RTN_UNICAST,
		Attributes: &rtnetlink.RuleAttributes{
			FwMark:   pointer.To(m.InternalMark),
			FwMask:   pointer.To(m.MarkMask),
			Priority: pointer.To(nextRuleNumber(nc, unix.AF_INET)),
		},
	}); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("failed to add IPv6 table-mark rule: %w", err)
		}
	}

	return nil
}

func (m *rulesManager) deleteRulesFamily(nc *rtnetlink.Conn, family uint8) error {
	var merr *multierror.Error

	list, err := nc.Rule.List()
	if err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to get route rules: %w", err))
	}

	for _, r := range list {
		if r.Family == family &&
			r.Table == m.TargetTable &&
			pointer.SafeDeref(r.Attributes.FwMark) == m.InternalMark {
			if err := nc.Rule.Delete(&r); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					merr = multierror.Append(merr, err)
				}
			}

			break
		}
	}

	return merr.ErrorOrNil()
}

// Cleanup the installed routing rules.
func (m *rulesManager) Cleanup() error {
	var merr *multierror.Error

	nc, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("failed to get netlink handle: %w", err)
	}

	defer nc.Close() //nolint:errcheck

	if err = m.deleteRulesFamily(nc, unix.AF_INET); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to delete all IPv4 route rules: %w", err))
	}

	if err = m.deleteRulesFamily(nc, unix.AF_INET6); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to delete all IPv6 route rules: %w", err))
	}

	return merr.ErrorOrNil()
}

func nextRuleNumber(nc *rtnetlink.Conn, family uint8) uint32 {
	list, err := nc.Rule.List()
	if err != nil {
		return 0
	}

	for i := uint32(32500); i > 0; i-- {
		var found bool

		for _, r := range list {
			if r.Family == family && pointer.SafeDeref(r.Attributes.Priority) == i {
				found = true

				break
			}
		}

		if !found {
			return i
		}
	}

	return 0
}
