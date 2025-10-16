// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operator

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// GetClientIdentifier returns the DHCP client identifier to use.
func GetClientIdentifier(ctx context.Context, st state.State, linkName string, clientIdentifier nethelpers.ClientIdentifier) ([]byte, error) {
	if clientIdentifier == nethelpers.ClientIdentifierNone {
		return nil, nil
	}

	link, err := safe.StateGetByID[*network.LinkStatus](ctx, st, linkName)
	if err != nil {
		return nil, fmt.Errorf("error getting link %q: %w", linkName, err)
	}

	switch clientIdentifier {
	case nethelpers.ClientIdentifierNone:
		panic("unreachable")
	case nethelpers.ClientIdentifierMAC:
		if len(link.TypedSpec().HardwareAddr) == 0 {
			return nil, fmt.Errorf("link %q has no hardware address", linkName)
		}

		// per RFC 2132, section 9.14
		return append([]byte{1}, link.TypedSpec().HardwareAddr...), nil
	default:
		return nil, fmt.Errorf("unknown client identifier %d", clientIdentifier)
	}
}
