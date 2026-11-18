// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package base

import (
	"sync"
	"testing"

	"github.com/siderolabs/talos/pkg/provision"
)

type dhcpLeaseTracker struct {
	mu   sync.Mutex
	held map[string]struct{}
}

var dhcpLeases dhcpLeaseTracker

// LeaseDHCPRecord hands t one of the cluster's spare DHCP reservations for as long as t runs.
//
// Two holders of the same MAC on one network confuse both the bridge and the DHCP server, so a
// record goes to one test at a time. The lease is released by t.Cleanup; cleanups run last in,
// first out, so lease before registering the cleanup that tears down whatever uses the address,
// and make that cleanup wait until the address is gone.
//
// It returns false if the cluster has no spare reservations or all of them are held.
func (talosSuite *TalosSuite) LeaseDHCPRecord(t testing.TB) (provision.DHCPRecord, bool) {
	t.Helper()

	if talosSuite.Cluster == nil {
		return provision.DHCPRecord{}, false
	}

	dhcpLeases.mu.Lock()
	defer dhcpLeases.mu.Unlock()

	if dhcpLeases.held == nil {
		dhcpLeases.held = map[string]struct{}{}
	}

	for _, record := range talosSuite.Cluster.Info().Network.ExtraDHCPRecords {
		if _, held := dhcpLeases.held[record.MAC]; held {
			continue
		}

		dhcpLeases.held[record.MAC] = struct{}{}

		t.Logf("leased DHCP record %s (%s)", record.MAC, record.IP)

		t.Cleanup(func() {
			dhcpLeases.mu.Lock()
			defer dhcpLeases.mu.Unlock()

			delete(dhcpLeases.held, record.MAC)
		})

		return record, true
	}

	return provision.DHCPRecord{}, false
}
