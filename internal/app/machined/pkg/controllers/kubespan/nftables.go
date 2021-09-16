// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"fmt"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"inet.af/netaddr"
)

// NfTablesManager manages nftables outside of controllers/resources scope.
type NfTablesManager interface {
	Update(*netaddr.IPSet) error
	Cleanup() error
}

type nfTablesManager struct {
	InternalMark uint32
	ExternalMark uint32

	currentSet *netaddr.IPSet

	// nfTable is a handle for the WgLAN root table
	nfTable *nftables.Table

	// targetSet4 is a handle for the IPv4 target IP nftables set
	targetSet4 *nftables.Set

	// targetSet6 is a handle for the IPv6 target IP nftables set
	targetSet6 *nftables.Set
}

// Update the nftables rules based on the IPSet.
func (m *nfTablesManager) Update(desired *netaddr.IPSet) error {
	if m.currentSet != nil && m.currentSet.Equal(desired) {
		return nil
	}

	if err := m.setNFTable(desired); err != nil {
		return fmt.Errorf("failed to update IP sets: %w", err)
	}

	m.currentSet = desired

	return nil
}

// Cleanup the nftables rules.
func (m *nfTablesManager) Cleanup() error {
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

func (m *nfTablesManager) setNFTable(ips *netaddr.IPSet) error {
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
		Name:   "talos_kubespan",
	}
	table = c.AddTable(table)

	preChain := c.AddChain(&nftables.Chain{
		Name:     "kubespan_prerouting",
		Table:    table,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookPrerouting,
		Priority: nftables.ChainPriorityFilter,
	})

	outChain := c.AddChain(&nftables.Chain{
		Name:     "kubespan_outgoing",
		Table:    table,
		Type:     nftables.ChainTypeRoute,
		Hooknum:  nftables.ChainHookOutput,
		Priority: nftables.ChainPriorityFilter,
	})

	targetSetV4 := &nftables.Set{
		Name:     "kubespan_targets_ipv4",
		Table:    table,
		Interval: true,
		KeyType:  nftables.TypeIPAddr,  // prefix
		DataType: nftables.TypeInteger, // mask
	}

	targetSetV6 := &nftables.Set{
		Name:     "kubespan_targets_ipv6",
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

	// match fwmark of Wireguard interface (not kubespan mark)
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
				Data:     binaryutil.NativeEndian.PutUint32(m.ExternalMark),
			},
			&expr.Verdict{
				Kind: expr.VerdictAccept,
			},
		},
	})

	// match fwmark of Wireguard interface (not kubespan mark)
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
				Data:     binaryutil.NativeEndian.PutUint32(m.ExternalMark),
			},
			&expr.Verdict{
				Kind: expr.VerdictAccept,
			},
		},
	})

	c.AddRule(&nftables.Rule{
		Table: table,
		Chain: preChain,
		Exprs: matchIPv4Set(targetSetV4, m.InternalMark),
	})

	c.AddRule(&nftables.Rule{
		Table: table,
		Chain: preChain,
		Exprs: matchIPv6Set(targetSetV6, m.InternalMark),
	})

	c.AddRule(&nftables.Rule{
		Table: table,
		Chain: outChain,
		Exprs: matchIPv4Set(targetSetV4, m.InternalMark),
	})

	c.AddRule(&nftables.Rule{
		Table: table,
		Chain: outChain,
		Exprs: matchIPv6Set(targetSetV6, m.InternalMark),
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

func (m *nfTablesManager) setElements(ips *netaddr.IPSet) (setElements4, setElements6 []nftables.SetElement) {
	if ips == nil {
		return nil, nil
	}

	for _, r := range ips.Ranges() {
		fromBin, _ := r.From().MarshalBinary() //nolint:errcheck // doesn't fail

		toBin, _ := r.To().Next().MarshalBinary() //nolint:errcheck // doesn't fail

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

	return setElements4, setElements6
}
