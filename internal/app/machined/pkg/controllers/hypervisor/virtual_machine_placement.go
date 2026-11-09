// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/placement"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// readNUMATopology returns the published host NUMA inventory, or nil while there is none.
func readNUMATopology(ctx context.Context, r controller.Reader) (*hardware.NUMATopologySpec, error) {
	topology, err := safe.ReaderGetByID[*hardware.NUMATopology](ctx, r, hardware.NUMATopologyID)
	if state.IsNotFoundError(err) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("get host NUMA topology: %w", err)
	}

	return topology.TypedSpec(), nil
}

// checkDomainPlacement validates the exact domain definition against the current host inventory.
func checkDomainPlacement(ctx context.Context, r controller.Reader, name, domainXML string) error {
	topology, err := readNUMATopology(ctx, r)
	if err != nil {
		return err
	}

	return placement.Validate(name, domainXML, topology)
}
