// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/pkg/machinery/client"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// MachineContext represents the current machine configuration.
type MachineContext struct {
	Version    string
	Schematic  string
	SecureBoot bool
	Platform   string
	Arch       string
}

// QueryMachineContext retrieves the current machine's context.
func QueryMachineContext(ctx context.Context, c *client.Client) (*MachineContext, error) {
	machineCtx := &MachineContext{}

	// Get version and platform info via machine.Version RPC
	versionResp, err := c.Version(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	if len(versionResp.GetMessages()) > 0 {
		msg := versionResp.GetMessages()[0]
		machineCtx.Version = msg.GetVersion().GetTag()
		machineCtx.Platform = msg.GetPlatform().GetName()
		machineCtx.Arch = msg.GetPlatform().GetMode()
	}

	schematic, err := getSchematicID(ctx, c)
	if err != nil {
		log.Printf("warning: failed to get schematic ID: %v", err)
		schematic = ""
	}
	machineCtx.Schematic = schematic

	secureBoot, err := getSecureBootStatus(ctx, c)
	if err != nil {
		log.Printf("warning: failed to get secure boot status: %v", err)
		secureBoot = false
	}
	machineCtx.SecureBoot = secureBoot

	return machineCtx, nil
}

func getSchematicID(ctx context.Context, c *client.Client) (string, error) {
	configRes, err := safe.StateGet[*configres.MachineConfig](
		ctx,
		c.COSI,
		resource.NewMetadata(configres.NamespaceName, configres.MachineConfigType, configres.ActiveID, resource.VersionUndefined),
	)
	if err != nil {
		return "", fmt.Errorf("failed to get machine config: %w", err)
	}

	installImage := configRes.Provider().Machine().Install().Image()
	if installImage == "" {
		return "", nil
	}

	before, _, found := strings.Cut(installImage, ":")
	if !found {
		return "", nil
	}

	idx := strings.LastIndex(before, "/")
	if idx == -1 {
		return "", nil
	}
	
	schematic := before[idx+1:]

	if len(schematic) == 64 && isLowercaseHex(schematic) {
		return schematic, nil
	}

	return "", nil
}

func isLowercaseHex(s string) bool {
	for _, ch := range s {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			return false
		}
	}
	return true
}

func getSecureBootStatus(ctx context.Context, c *client.Client) (bool, error) {
	securityState, err := safe.StateGet[*runtimeres.SecurityState](
		ctx,
		c.COSI,
		resource.NewMetadata(runtimeres.NamespaceName, runtimeres.SecurityStateType, runtimeres.SecurityStateID, resource.VersionUndefined),
	)
	if err != nil {
		return false, fmt.Errorf("failed to get security state: %w", err)
	}

	return securityState.TypedSpec().SecureBoot, nil
}
