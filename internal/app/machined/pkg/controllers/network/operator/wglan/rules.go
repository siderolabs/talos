// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package wglan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"github.com/hashicorp/go-multierror"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const reconciliationInterval = time.Minute

// RulesManager maintains the NFTables, Route Rules, and Routing Table for WgLAN.
type RulesManager struct {
	db *PeerDB

	// publicKey is the public key of this node, so that it can be filtered from the routing.
	publicKey string

	// externalMark is the firewall mark used by Wireguard to indicate packets which should not be routed through the Wireguard interface because they are the Wireguard interface's _own_ packets.
	externalMark uint32

	// internalMark is the firewall mark which is use by the RulesManager to indicate rules which _should_ be routed through the Wireguard.
	internalMark uint32

	// targetTable is the routing table to be used as the target for internalMark packets, to route them to through the Wireguard interface.
	targetTable int

	// currentSet records the current set of IP Prefixes which are stored in the NFTables set
	currentSet *netaddr.IPSet

	// nfTable is a handle for the WgLAN root table
	nfTable *nftables.Table

	// targetSet4 is a handle for the IPv4 target IP nftables set
	targetSet4 *nftables.Set

	// targetSet6 is a handle for the IPv6 target IP nftables set
	targetSet6 *nftables.Set

	logger *zap.Logger
}

// Run starts the Rules Manager, maintaining the components over time.
func (m *RulesManager) Run(ctx context.Context, logger *zap.Logger, db *PeerDB) error {
	if m.externalMark == 0 {
		m.externalMark = constants.WireguardDefaultFirewallMark
	}

	if m.internalMark == 0 {
		m.internalMark = constants.WireguardDefaultForceFirewallMark
	}

	if m.targetTable == 0 {
		m.targetTable = constants.WireguardDefaultRoutingTable
	}

	m.db = db

	m.logger = logger

	if err := m.setup(); err != nil {
		return fmt.Errorf("failed to setup initial routes and rules: %w", err)
	}

	go m.maintain(ctx)

	return nil
}

func (m *RulesManager) setup() error {
	if err := m.createRules(); err != nil {
		return fmt.Errorf("failed to ensure wireguard force rule: %w", err)
	}

	if err := m.reconcile(); err != nil {
		return fmt.Errorf("failed to perform initial table construction: %w", err)
	}

	return nil
}

func (m *RulesManager) maintain(ctx context.Context) {
	defer func() {
		if err := m.deleteNFTable(); err != nil {
			m.logger.Warn("failed to delete NFTable", zap.Error(err))
		}

		if err := m.deleteRules(); err != nil {
			m.logger.Warn("failed to delete IP route rules", zap.Error(err))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("shutting down rules manager")

			return
		case <-time.After(reconciliationInterval):
			if err := m.reconcile(); err != nil {
				m.logger.Warn("ip rule reconciliation failed", zap.Error(err))
			}
		}
	}
}

func (m *RulesManager) collectTargets() (*netaddr.IPSet, error) {
	b := new(netaddr.IPSetBuilder)

	for _, pp := range m.db.List() {
		if pp == nil {
			continue
		}

		// NOTE: it may be more reliable to pull our own entry from the database first and blacklist our AllowedPrefixes instead.
		if pp.PublicKey() == m.publicKey {
			continue // skip our own
		}

		if !pp.Up() {
			continue
		}

		routeSet, err := pp.AllowedPrefixes()
		if err != nil {
			return nil, fmt.Errorf("failed to acquire allowed prefixes for peer %s", pp.node.Name)
		}

		b.AddSet(routeSet)
	}

	return b.IPSet()
}

func (m *RulesManager) reconcile() error {
	desired, err := m.collectTargets()
	if err != nil {
		return fmt.Errorf("failed to collect desired rule targets: %w", err)
	}

	if m.currentSet == desired {
		return nil
	}

	if err := m.setNFTable(desired); err != nil {
		return fmt.Errorf("failed to update IP sets: %w", err)
	}

	m.currentSet = desired

	return nil
}

func (m *RulesManager) deleteNFTable() error {
	c := &nftables.Conn{}

	// NB: sets should be flushed before new members because nftables will fail
	// if there are any conflicts between existing ranges and new ranges.

	c.FlushSet(m.targetSet4)

	c.FlushSet(m.targetSet6)

	c.FlushTable(m.nfTable)

	if err := c.Flush(); err != nil {
		return fmt.Errorf("failed to execute nftable creation: %w", err)
	}

	return nil
}

func (m *RulesManager) setNFTable(ips *netaddr.IPSet) error {
	c := &nftables.Conn{}

	// NB: sets should be flushed before new members because nftables will fail
	// if there are any conflicts between existing ranges and new ranges.

	if m.targetSet4 != nil {
		c.FlushSet(m.targetSet4)
	}

	if m.targetSet6 != nil {
		c.FlushSet(m.targetSet6)
	}

	if m.nfTable != nil {
		c.FlushTable(m.nfTable)
	}

	// Basic boilerplate; create a table & chain.
	table := &nftables.Table{
		Family: nftables.TableFamilyINet,
		Name:   "talos_wglan",
	}
	table = c.AddTable(table)

	preChain := c.AddChain(&nftables.Chain{
		Name:     "wglan_prerouting",
		Table:    table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookPrerouting,
		Priority: nftables.ChainPriorityFilter,
	})

	outChain := c.AddChain(&nftables.Chain{
		Name:     "wglan_outgoing",
		Table:    table,
		Type:     nftables.ChainTypeRoute,
		Hooknum:  nftables.ChainHookOutput,
		Priority: nftables.ChainPriorityFilter,
	})

	targetSetV4 := &nftables.Set{
		Name:     "wglan_targets_ipv4",
		Table:    table,
		Interval: true,
		KeyType:  nftables.TypeIPAddr,  // prefix
		DataType: nftables.TypeInteger, // mask
	}

	targetSetV6 := &nftables.Set{
		Name:     "wglan_targets_ipv6",
		Table:    table,
		Interval: true,
		KeyType:  nftables.TypeIP6Addr,
	}

	setElements4, setElements6 := m.setElements(ips)

	if err := c.AddSet(targetSetV4, setElements4); err != nil {
		return fmt.Errorf("failed to add IPv4 set: %w", err)
	}

	if err := c.AddSet(targetSetV6, setElements6); err != nil {
		return fmt.Errorf("failed to add IPv6 set: %w", err)
	}

	// match fwmark of Wireguard interface (not wglan mark)
	// accept and return without modifying the table or mark
	c.AddRule(&nftables.Rule{
		Table: table,
		Chain: preChain,
		Exprs: []expr.Any{
			&expr.Meta{
				Key:      expr.MetaKeyMARK,
				Register: 1,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     binaryutil.NativeEndian.PutUint32(m.externalMark),
			},
			&expr.Verdict{
				Kind: expr.VerdictAccept,
			},
		},
	})

	// match fwmark of Wireguard interface (not wglan mark)
	// accept and return without modifying the table or mark
	c.AddRule(&nftables.Rule{
		Table: table,
		Chain: outChain,
		Exprs: []expr.Any{
			&expr.Meta{
				Key:      expr.MetaKeyMARK,
				Register: 1,
			},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     binaryutil.NativeEndian.PutUint32(m.externalMark),
			},
			&expr.Verdict{
				Kind: expr.VerdictAccept,
			},
		},
	})

	c.AddRule(&nftables.Rule{
		Table: table,
		Chain: preChain,
		Exprs: matchIPv4Set(targetSetV4, m.internalMark),
	})

	c.AddRule(&nftables.Rule{
		Table: table,
		Chain: preChain,
		Exprs: matchIPv6Set(targetSetV6, m.internalMark),
	})

	c.AddRule(&nftables.Rule{
		Table: table,
		Chain: outChain,
		Exprs: matchIPv4Set(targetSetV4, m.internalMark),
	})

	c.AddRule(&nftables.Rule{
		Table: table,
		Chain: outChain,
		Exprs: matchIPv6Set(targetSetV6, m.internalMark),
	})

	if err := c.Flush(); err != nil {
		return fmt.Errorf("failed to execute nftable creation: %w", err)
	}

	m.nfTable = table

	m.targetSet4 = targetSetV4

	m.targetSet6 = targetSetV6

	return nil
}

func matchIPv4Set(set *nftables.Set, mark uint32) []expr.Any {
	return matchIPSet(set, mark, nftables.TableFamilyIPv4)
}

func matchIPv6Set(set *nftables.Set, mark uint32) []expr.Any {
	return matchIPSet(set, mark, nftables.TableFamilyIPv6)
}

func matchIPSet(set *nftables.Set, mark uint32, family nftables.TableFamily) []expr.Any {
	var (
		offset uint32 = 16

		length uint32 = 4
	)

	if family == nftables.TableFamilyIPv6 {
		offset = 24

		length = 16
	}

	return []expr.Any{
		// Store protocol type to register 1
		&expr.Meta{
			Key:      expr.MetaKeyNFPROTO,
			Register: 1,
		},
		// Match IP Family
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     []byte{byte(family)},
		},
		// Store the destination IP address to register 1
		&expr.Payload{
			DestRegister: 1,
			Base:         expr.PayloadBaseNetworkHeader,
			Offset:       offset,
			Len:          length,
		},
		// Match from target set
		&expr.Lookup{
			SourceRegister: 1,
			SetName:        set.Name,
			SetID:          set.ID,
		},
		// Store Firewall Force mark to register 1
		&expr.Immediate{
			Register: 1,
			Data:     binaryutil.NativeEndian.PutUint32(mark),
		},
		// Set firewall mark
		&expr.Meta{
			Key:            expr.MetaKeyMARK,
			SourceRegister: true,
			Register:       1,
		},
		&expr.Verdict{
			Kind: expr.VerdictAccept,
		},
	}
}

func (m *RulesManager) setElements(ips *netaddr.IPSet) (setElements4, setElements6 []nftables.SetElement) {
	if ips == nil {
		return nil, nil
	}

	for _, r := range ips.Ranges() {
		fromBin, err := r.From().MarshalBinary()
		if err != nil {
			m.logger.Sugar().Warn("failed to marshal from set from IP: %q: %w", r.From().String(), err)

			continue
		}

		toBin, err := r.To().Next().MarshalBinary()
		if err != nil {
			m.logger.Sugar().Warn("failed to marshal to IP %q: %w", r.To().Next().String(), err)

			continue
		}

		se := []nftables.SetElement{
			{
				Key:         fromBin,
				IntervalEnd: false,
			},
			{
				Key:         toBin,
				IntervalEnd: true,
			},
		}

		if r.From().Is6() {
			setElements6 = append(setElements6, se...)
		} else {
			setElements4 = append(setElements4, se...)
		}
	}

	metricRouteCount.WithLabelValues("ipv4").
		Set(float64(len(setElements4)))

	metricRouteCount.WithLabelValues("ipv6").
		Set(float64(len(setElements6)))

	return setElements4, setElements6
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

func (m *RulesManager) createRules() error {
	nc, err := netlink.NewHandle()
	if err != nil {
		return fmt.Errorf("failed to get netlink handle: %w", err)
	}

	defer nc.Delete()

	if err := nc.RuleAdd(&netlink.Rule{
		Priority:          nextRuleNumber(nc, unix.AF_INET),
		Family:            unix.AF_INET,
		Table:             m.targetTable,
		Mark:              int(m.internalMark),
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
		Table:             m.targetTable,
		Mark:              int(m.internalMark),
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

func (m *RulesManager) deleteRulesFamily(nc *netlink.Handle, family int) error {
	var merr *multierror.Error

	list, err := nc.RuleList(family)
	if err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to get route rules: %w", err))
	}

	for _, r := range list {
		if r.Table == m.targetTable &&
			r.Mark == int(m.internalMark) {
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

func (m *RulesManager) deleteRules() error {
	var merr *multierror.Error

	nc, err := netlink.NewHandle()
	if err != nil {
		return fmt.Errorf("failed to get netlink handle: %w", err)
	}

	defer nc.Delete()

	if err = m.deleteRulesFamily(nc, unix.AF_INET); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to delete all IPv4 route rules: %w", err))
	}

	if err = m.deleteRulesFamily(nc, unix.AF_INET6); err != nil {
		merr = multierror.Append(merr, fmt.Errorf("failed to delete all IPv6 route rules: %w", err))
	}

	return merr.ErrorOrNil()
}
