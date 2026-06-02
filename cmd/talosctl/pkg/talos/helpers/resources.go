// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// MachineConfigUpdater gets resources from the controller runtime and runs a callback for each resource.
//
//nolint:gocyclo
func MachineConfigUpdater(ctx context.Context,
	clientFactory *global.ClientFactory,
	callback func(ctx context.Context, client *client.Client, node string, mc resource.Resource) error,
	args []string,
) error {
	if len(args) == 0 {
		return errors.New("not enough arguments: at least 1 is expected")
	}

	resourceType := args[0]

	// ignore args[1], always patch in current machine config resource ID
	resourceID := config.ActiveID

	// resolve resource type, we don't support anything but machineconfig now
	if len(clientFactory.Nodes()) == 0 {
		return nil
	}

	// resolve resource kind
	firstCtx, firstC, err := clientFactory.BuildClient(ctx, clientFactory.Nodes()[0])
	if err != nil {
		return err
	}

	var namespace string

	rd, err := firstC.ResolveResourceKind(firstCtx, &namespace, resourceType)
	if err != nil {
		return err
	}

	if rd.TypedSpec().Type != config.MachineConfigType {
		return errors.New("only machineconfig resource type is supported")
	}

	resourceType = rd.TypedSpec().Type

	for _, node := range clientFactory.Nodes() {
		nodeCtx, nodeClient, err := clientFactory.BuildClient(ctx, node)
		if err != nil {
			return fmt.Errorf("error building client for node %s: %w", node, err)
		}

		r, err := nodeClient.COSI.Get(
			nodeCtx,
			resource.NewMetadata(namespace, resourceType, resourceID, resource.VersionUndefined),
			state.WithGetUnmarshalOptions(state.WithSkipProtobufUnmarshal()),
		)
		if err != nil {
			return fmt.Errorf("error getting resource on node %s: %w", node, err)
		}

		if err = callback(nodeCtx, nodeClient, node, r); err != nil {
			return err
		}
	}

	return nil
}
