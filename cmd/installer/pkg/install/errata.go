// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// errataNetIfnames appends the `net.ifnames=0` kernel parameter to the kernel command line if upgrading
// from an old enough version of Talos.
func (i *Installer) errataNetIfnames(talosVersion *compatibility.TalosVersion) {
	if i.cmdline.Get(constants.KernelParamNetIfnames).First() != nil {
		// net.ifnames is already set, nothing to do
		return
	}

	oldTalos := upgradeFromPreIfnamesTalos(talosVersion)

	if oldTalos {
		log.Printf("appending net.ifnames=0 to the kernel command line")

		i.cmdline.Append(constants.KernelParamNetIfnames, "0")
	}
}

func readHostTalosVersion() (*compatibility.TalosVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if _, err := os.Stat(constants.MachineSocketPath); err != nil {
		// can't read Talos version
		return nil, nil
	}

	c, err := client.New(ctx,
		client.WithUnixSocket(constants.MachineSocketPath),
		client.WithGRPCDialOptions(
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("error connecting to the machine service: %w", err)
	}

	defer c.Close() //nolint:errcheck

	// inject "fake" authorization
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(constants.APIAuthzRoleMetadataKey, string(role.Admin)))

	resp, err := c.Version(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting Talos version: %w", err)
	}

	hostVersion := unpack(resp.Messages)

	talosVersion, err := compatibility.ParseTalosVersion(hostVersion.Version)
	if err != nil {
		return nil, fmt.Errorf("error parsing Talos version: %w", err)
	}

	return talosVersion, nil
}

func upgradeFromPreIfnamesTalos(talosVersion *compatibility.TalosVersion) bool {
	if talosVersion == nil {
		// old Talos version, include fallback
		return true
	}

	return talosVersion.DisablePredictableNetworkInterfaces()
}
