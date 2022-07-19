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

// NewNfTablesManager initializes NfTablesManager.
func NewNfTablesManager(externalMark, internalMark, markMask uint32) NfTablesManager {
	nfTable := &nftables.Table{
		Family: nftables.TableFamilyINet,
		Name:   "talos_kubespan",
	}

	return &nfTablesManager{
		ExternalMark: externalMark,
		InternalMark: internalMark,
		MarkMask:     markMask,

		nfTable: nfTable,
		targetSet4: &nftables.Set{
			Name:     "kubespan_targets_ipv4",
			Table:    nfTable,
			Interval: true,
			KeyType:  nftables.TypeIPAddr,  // prefix
			DataType: nftables.TypeInteger, // mask
		},
		targetSet6: &nftables.Set{
			Name:     "kubespan_targets_ipv6",
			Table:    nfTable,
			Interval: true,
			KeyType:  nftables.TypeIP6Addr,
		},
	}
}

type nfTablesManager struct {
	InternalMark uint32
	ExternalMark uint32
	MarkMask     uint32

	currentSet *netaddr.IPSet

	// nfTable is a handle for the KubeSpan root table
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
	foundExisting, err := m.tableExists()
	if err != nil {
		return err
	}

	if !foundExisting {
		return nil
	}

	c := &nftables.Conn{}

	c.FlushSet(m.targetSet4)
	c.FlushSet(m.targetSet6)
	c.FlushTable(m.nfTable)

	c.DelSet(m.targetSet4)
	c.DelSet(m.targetSet6)
	c.DelTable(m.nfTable)

	if err := c.Flush(); err != nil {
		return fmt.Errorf("failed to execute nftable cleanup: %w", err)
	}

	return nil
}

func (m *nfTablesManager) tableExists() (bool, error) {
	c := &nftables.Conn{}

	tables, err := c.ListTables()
	if err != nil {
		return false, fmt.Errorf("error listing tables: %w", err)
	}

	foundExisting := false

	for _, table := range tables {
		if table.Name == m.nfTable.Name && table.Family == m.nfTable.Family {
			foundExisting = true

			break
		}
	}

	return foundExisting, nil
}

func (m *nfTablesManager) setNFTable(ips *netaddr.IPSet) error {
	c := &nftables.Conn{}

	// NB: sets should be flushed before new members because nftables will fail
	// if there are any conflicts between existing ranges and new ranges.

	foundExisting, err := m.tableExists()
	if err != nil {
		return err
	}

	if foundExisting {
		c.FlushSet(m.targetSet4)
		c.FlushSet(m.targetSet6)
		c.FlushTable(m.nfTable)
	}

	// Basic boilerplate; create a table & chain.
	c.AddTable(m.nfTable)

	preChain := c.AddChain(&nftables.Chain{
		Name:     "kubespan_prerouting",
		Table:    m.nfTable,
		Type:     nftables.ChainTypeFilter,
		Hooknum:  nftables.ChainHookPrerouting,
		Priority: nftables.ChainPriorityFilter,
	})

	outChain := c.AddChain(&nftables.Chain{
		Name:     "kubespan_outgoing",
		Table:    m.nfTable,
		Type:     nftables.ChainTypeRoute,
		Hooknum:  nftables.ChainHookOutput,
		Priority: nftables.ChainPriorityFilter,
	})

	setElements4, setElements6 := m.setElements(ips)

	if err := c.AddSet(m.targetSet4, setElements4); err != nil {
		return fmt.Errorf("failed to add IPv4 set: %w", err)
	}

	if err := c.AddSet(m.targetSet6, setElements6); err != nil {
		return fmt.Errorf("failed to add IPv6 set: %w", err)
	}

	// meta mark & 0x00000060 == 0x00000020 accept
	ruleExpr := []expr.Any{
		// Load the firewall mark into register 1
		&expr.Meta{
			Key:      expr.MetaKeyMARK,
			Register: 1,
		},
		// Mask the mark with the configured mask:
		//  R1 = R1 & mask
		&expr.Bitwise{
			SourceRegister: 1,
			DestRegister:   1,
			Len:            4,
			Xor:            binaryutil.NativeEndian.PutUint32(0),
			Mask:           binaryutil.NativeEndian.PutUint32(m.MarkMask),
		},
		// Compare the masked firewall mark with expected value
		&expr.Cmp{
			Op:       expr.CmpOpEq,
			Register: 1,
			Data:     binaryutil.NativeEndian.PutUint32(m.ExternalMark),
		},
		// Accept the packet to stop the ruleset processing
		&expr.Verdict{
			Kind: expr.VerdictAccept,
		},
	}

	// match fwmark of Wireguard interface (not kubespan mark)
	// accept and return without modifying the table or mark
	c.AddRule(&nftables.Rule{
		Table: m.nfTable,
		Chain: preChain,
		Exprs: ruleExpr,
	})

	// match fwmark of Wireguard interface (not kubespan mark)
	// accept and return without modifying the table or mark
	c.AddRule(&nftables.Rule{
		Table: m.nfTable,
		Chain: outChain,
		Exprs: ruleExpr,
	})

	c.AddRule(&nftables.Rule{
		Table: m.nfTable,
		Chain: preChain,
		Exprs: matchIPv4Set(m.targetSet4, m.InternalMark, m.MarkMask),
	})

	c.AddRule(&nftables.Rule{
		Table: m.nfTable,
		Chain: preChain,
		Exprs: matchIPv6Set(m.targetSet6, m.InternalMark, m.MarkMask),
	})

	c.AddRule(&nftables.Rule{
		Table: m.nfTable,
		Chain: outChain,
		Exprs: matchIPv4Set(m.targetSet4, m.InternalMark, m.MarkMask),
	})

	c.AddRule(&nftables.Rule{
		Table: m.nfTable,
		Chain: outChain,
		Exprs: matchIPv6Set(m.targetSet6, m.InternalMark, m.MarkMask),
	})

	if err := c.Flush(); err != nil {
		return fmt.Errorf("failed to execute nftable creation: %w", err)
	}

	return nil
}

func matchIPv4Set(set *nftables.Set, mark, mask uint32) []expr.Any {
	return matchIPSet(set, mark, mask, nftables.TableFamilyIPv4)
}

func matchIPv6Set(set *nftables.Set, mark, mask uint32) []expr.Any {
	return matchIPSet(set, mark, mask, nftables.TableFamilyIPv6)
}

func matchIPSet(set *nftables.Set, mark, mask uint32, family nftables.TableFamily) []expr.Any {
	var (
		offset uint32 = 16
		length uint32 = 4
	)

	if family == nftables.TableFamilyIPv6 {
		offset = 24
		length = 16
	}

	// ip daddr @kubespan_targets_ipv4 meta mark set meta mark & 0xffffffdf | 0x00000040 accept
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
		// Load the current packet mark into register 1
		&expr.Meta{
			Key:      expr.MetaKeyMARK,
			Register: 1,
		},
		// This bitwise is equivalent to: R1 = R1 | (R1 & mask | mark)
		//
		// The NFTables backend bitwise operation is R3 = R2 & MASK ^ XOR,
		// so we need to do a bit of a trick to do what we need: R1 = R1 & ^mask ^ mark
		&expr.Bitwise{
			SourceRegister: 1,
			DestRegister:   1,
			Len:            4,
			Xor:            binaryutil.NativeEndian.PutUint32(mark),
			Mask:           binaryutil.NativeEndian.PutUint32(^mask),
		},
		// Set firewall mark to the value computed in register 1
		&expr.Meta{
			Key:            expr.MetaKeyMARK,
			SourceRegister: true,
			Register:       1,
		},
		// Accept the packet to stop the ruleset processing
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
