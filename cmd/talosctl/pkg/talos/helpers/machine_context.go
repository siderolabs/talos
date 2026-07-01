// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/client"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// MachineContext is the machine state relevant to building an installer image reference.
//
// An empty Schematic means the machine doesn't expose its schematic ID (e.g. not a factory image).
// An empty FactoryHost means the machine doesn't expose the factory it was installed from.
type MachineContext struct {
	Schematic   string
	FactoryHost string
	Platform    string
	SecureBoot  bool
}

// QueryMachineContext reads the machine state of the node set in ctx.
//
// Only runtime resources are consulted, never the machine configuration.
func QueryMachineContext(ctx context.Context, c *client.Client) (*MachineContext, error) {
	machineCtx := &MachineContext{}

	platformMetadata, err := safe.StateGetByID[*runtimeres.PlatformMetadata](ctx, c.COSI, runtimeres.PlatformMetadataID)
	if err != nil {
		return nil, fmt.Errorf("failed to get platform metadata: %w", err)
	}

	machineCtx.Platform = platformMetadata.TypedSpec().Platform

	securityState, err := safe.StateGetByID[*runtimeres.SecurityState](ctx, c.COSI, runtimeres.SecurityStateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get security state: %w", err)
	}

	machineCtx.SecureBoot = securityState.TypedSpec().SecureBoot

	// The ImageFactorySchematic resource is surfaced by the runtime controller only when the
	// machine was installed from an Image Factory image, so its absence is not an error.
	schematic, err := safe.StateGetByID[*runtimeres.ImageFactorySchematic](ctx, c.COSI, runtimeres.ImageFactorySchematicID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return machineCtx, nil
		}

		return nil, fmt.Errorf("failed to get image factory schematic: %w", err)
	}

	machineCtx.Schematic = schematic.TypedSpec().SchematicID
	machineCtx.FactoryHost = factoryHostFromAPIURL(schematic.TypedSpec().APIURL)

	return machineCtx, nil
}

// factoryHostFromAPIURL strips the scheme (and any trailing slash) from the schematic's
// API URL so it can be used as the host component of a container image reference.
//
// e.g. "https://factory.talos.dev" -> "factory.talos.dev".
func factoryHostFromAPIURL(apiURL string) string {
	host := apiURL

	for _, scheme := range []string{"https://", "http://"} {
		if rest, ok := strings.CutPrefix(host, scheme); ok {
			host = rest

			break
		}
	}

	return strings.TrimSuffix(host, "/")
}
