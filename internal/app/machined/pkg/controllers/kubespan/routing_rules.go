// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"errors"
	"fmt"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/vishvananda/netlink"
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
func NewRulesManager(targetTable, internalMark int) RulesManager {
	return &rulesManager{
		TargetTable:  targetTable,
		InternalMark: internalMark,
	}
}

type rulesManager struct {
	TargetTable  int
	InternalMark int
}

// Install routing rules.
func (m *rulesManager) Install() error {
	nc, err := netlink.NewHandle()
	if err != nil {
		return fmt.Errorf("failed to get netlink handle: %w", err)
	}

	defer nc.Close()

	if err := nc.RuleAdd(&netlink.Rule{
		Priority:          nextRuleNumber(nc, unix.AF_INET),
		Family:            unix.AF_INET,
		Table:             m.TargetTable,
		Mark:              m.InternalMark,
		Mask:              -1,
		Goto:              -1,
		Flow:              -1,
		SuppressIfgroup:   -1,
		SuppressPrefixlen: -1,
	}); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("failed to add IPv4 table-mark rule: %w", err)
		}
	}

	if err := nc.RuleAdd(&netlink.Rule{
		Priority:          nextRuleNumber(nc, unix.AF_INET6),
		Family:            unix.AF_INET6,
		Table:             m.TargetTable,
		Mark:              m.InternalMark,
		Mask:              -1,
		Goto:              -1,
		Flow:              -1,
		SuppressIfgroup:   -1,
		SuppressPrefixlen: -1,
	}); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("failed to add IPv6 table-mark rule: %w", err)
		}
	}

	return nil
}

func (m *rulesManager) deleteRulesFamily(nc *netlink.Handle, family int) error {
	var merr *multierror.Error

	list, err := nc.RuleList(family)
	if err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to get route rules: %w", err))
	}

	for _, r := range list {
		if r.Table == m.TargetTable &&
			r.Mark == m.InternalMark {
			thisRule := r

			if err := nc.RuleDel(&thisRule); err != nil {
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

	nc, err := netlink.NewHandle()
	if err != nil {
		return fmt.Errorf("failed to get netlink handle: %w", err)
	}

	defer nc.Close()

	if err = m.deleteRulesFamily(nc, unix.AF_INET); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to delete all IPv4 route rules: %w", err))
	}

	if err = m.deleteRulesFamily(nc, unix.AF_INET6); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to delete all IPv6 route rules: %w", err))
	}

	return merr.ErrorOrNil()
}

func nextRuleNumber(nc *netlink.Handle, family int) int {
	list, err := nc.RuleList(family)
	if err != nil {
		return 0
	}

	for i := 32500; i > 0; i-- {
		var found bool

		for _, r := range list {
			if r.Priority == i {
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
