// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	networkadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NfTablesChainController applies network.NfTablesChain to the Linux nftables interface.
type NfTablesChainController struct {
	TableName string
}

// Name implements controller.Controller interface.
func (ctrl *NfTablesChainController) Name() string {
	return "network.NfTablesChainController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NfTablesChainController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.NfTablesChainType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NfTablesChainController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *NfTablesChainController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.TableName == "" {
		ctrl.TableName = constants.DefaultNfTablesTableName
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var conn nftables.Conn

		if err := ctrl.preCreateIptablesNFTable(logger, &conn); err != nil {
			return fmt.Errorf("error pre-creating iptables-nft table: %w", err)
		}

		list, err := safe.ReaderListAll[*network.NfTablesChain](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing nftables chains: %w", err)
		}

		existingTables, err := conn.ListTablesOfFamily(nftables.TableFamilyINet)
		if err != nil {
			return fmt.Errorf("error listing existing nftables tables: %w", err)
		}

		var talosTable *nftables.Table

		if idx := slices.IndexFunc(existingTables, func(t *nftables.Table) bool { return t.Name == ctrl.TableName }); idx != -1 {
			talosTable = existingTables[idx]
		}

		if talosTable == nil {
			talosTable = &nftables.Table{
				Family: nftables.TableFamilyINet,
				Name:   ctrl.TableName,
			}

			conn.AddTable(talosTable)
		}

		// drop all chains, they will be re-created
		existingChains, err := conn.ListChains()
		if err != nil {
			return fmt.Errorf("error listing existing nftables chains: %w", err)
		}

		for _, chain := range existingChains {
			if chain.Table.Name != ctrl.TableName { // not our chain
				continue
			}

			conn.DelChain(chain)
		}

		setID := uint32(0)

		for iter := list.Iterator(); iter.Next(); {
			chain := iter.Value()

			nfChain := conn.AddChain(&nftables.Chain{
				Name:     chain.Metadata().ID(),
				Table:    talosTable,
				Hooknum:  pointer.To(nftables.ChainHook(chain.TypedSpec().Hook)),
				Priority: pointer.To(nftables.ChainPriority(chain.TypedSpec().Priority)),
				Type:     nftables.ChainType(chain.TypedSpec().Type),
				Policy:   pointer.To(nftables.ChainPolicy(chain.TypedSpec().Policy)),
			})

			for _, rule := range chain.TypedSpec().Rules {
				compiled, err := networkadapter.NfTablesRule(&rule).Compile()
				if err != nil {
					return fmt.Errorf("error compiling nftables rule for chain %s: %w", nfChain.Name, err)
				}

				for _, compiledRule := range compiled.Rules {
					// check for lookup rules and add/fix up the set ID if needed
					for i := range compiledRule {
						if lookup, ok := compiledRule[i].(*expr.Lookup); ok {
							if lookup.SetID >= uint32(len(compiled.Sets)) {
								return fmt.Errorf("invalid set ID %d in lookup", lookup.SetID)
							}

							set := compiled.Sets[lookup.SetID]
							setName := "__set" + strconv.Itoa(int(setID))

							if err = conn.AddSet(&nftables.Set{
								Table:     talosTable,
								ID:        setID,
								Name:      setName,
								Anonymous: true,
								Constant:  true,
								Interval:  set.IsInterval(),
								KeyType:   set.KeyType(),
							}, set.SetElements()); err != nil {
								return fmt.Errorf("error adding nftables set for chain %s: %w", nfChain.Name, err)
							}

							lookupOp := *lookup
							lookupOp.SetID = setID
							lookupOp.SetName = setName

							compiledRule[i] = &lookupOp

							setID++
						}
					}

					conn.AddRule(&nftables.Rule{
						Table: talosTable,
						Chain: nfChain,
						Exprs: compiledRule,
					})
				}
			}
		}

		if err := conn.Flush(); err != nil {
			return fmt.Errorf("error flushing nftables: %w", err)
		}

		chainNames := safe.ToSlice(list, func(chain *network.NfTablesChain) string { return chain.Metadata().ID() })
		logger.Info("nftables chains updated", zap.Strings("chains", chainNames))

		r.ResetRestartBackoff()
	}
}

func (ctrl *NfTablesChainController) preCreateIptablesNFTable(logger *zap.Logger, conn *nftables.Conn) error {
	// Pre-create the iptables-nft table, if it doesn't exist.
	// This is required to ensure that the iptables universal binary prefers iptables-nft over
	// iptables-legacy can be used to manage the nftables rules.
	tables, err := conn.ListTablesOfFamily(nftables.TableFamilyIPv4)
	if err != nil {
		return fmt.Errorf("error listing existing nftables tables: %w", err)
	}

	if slices.IndexFunc(tables, func(t *nftables.Table) bool { return t.Name == "mangle" }) != -1 {
		return nil
	}

	table := &nftables.Table{
		Family: nftables.TableFamilyIPv4,
		Name:   "mangle",
	}
	conn.AddTable(table)

	chain := &nftables.Chain{
		Name:  "KUBE-IPTABLES-HINT",
		Table: table,
		Type:  nftables.ChainTypeNAT,
	}
	conn.AddChain(chain)

	logger.Info("pre-created iptables-nft table 'mangle'/'KUBE-IPTABLES-HINT'")

	return nil
}
