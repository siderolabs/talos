// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/stretchr/testify/assert"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestResolveBondPrimary(t *testing.T) {
	t.Parallel()

	links := []rtnetlink.LinkMessage{
		{
			Index: 2,
			Attributes: &rtnetlink.LinkAttributes{
				Name:     "eno1",
				Alias:    new("uplink"),
				AltNames: []string{"slot-1"},
			},
		},
		{
			Index: 3,
			Attributes: &rtnetlink.LinkAttributes{
				Name: "eno2",
			},
		},
	}

	for _, test := range []struct {
		name    string
		primary string
		links   []rtnetlink.LinkMessage

		expectedIndex *uint32
	}{
		{
			name:    "by name",
			primary: "eno1",
			links:   links,

			expectedIndex: new(uint32(2)),
		},
		{
			name:    "second link",
			primary: "eno2",
			links:   links,

			expectedIndex: new(uint32(3)),
		},
		{
			name:    "by alias",
			primary: "uplink",
			links:   links,

			expectedIndex: new(uint32(2)),
		},
		{
			name:    "by altname",
			primary: "slot-1",
			links:   links,

			expectedIndex: new(uint32(2)),
		},
		{
			name:    "no primary configured",
			primary: "",
			links:   links,

			expectedIndex: nil,
		},
		{
			// the NIC hasn't shown up yet, or has been unplugged: leaving the index unset means the
			// attribute is not encoded at all, so the kernel's current primary is left alone rather
			// than being cleared (the kernel resolves an unknown index to an empty name)
			name:    "primary link absent",
			primary: "eno3",
			links:   links,

			expectedIndex: nil,
		},
		{
			name:    "no links at all",
			primary: "eno1",
			links:   nil,

			expectedIndex: nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			spec := network.BondMasterSpec{
				Mode:    nethelpers.BondModeActiveBackup,
				Primary: test.primary,
				// a stale index left over from a previous resolution must always be recomputed
				PrimaryIndex: new(uint32(42)),
			}

			resolved, primaryLink := netctrl.ResolveBondPrimaryForTest(spec, test.links)

			if test.expectedIndex == nil {
				assert.Nil(t, resolved.PrimaryIndex)
				assert.Nil(t, primaryLink)
			} else {
				assert.Equal(t, *test.expectedIndex, *resolved.PrimaryIndex)
				assert.Equal(t, *test.expectedIndex, primaryLink.Index)
			}

			// the name is carried through untouched, and the input spec is not mutated
			assert.Equal(t, test.primary, resolved.Primary)
			assert.Equal(t, uint32(42), *spec.PrimaryIndex)
		})
	}
}
