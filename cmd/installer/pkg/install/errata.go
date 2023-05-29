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

	pkgkernel "github.com/siderolabs/talos/pkg/kernel"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// errataBTF handles the case when kexec from pre-BTF kernel to BTF enabled kernel always fails.
//
// This applies to upgrades of Talos < 1.3.0 to Talos >= 1.3.0.
func errataBTF() {
	_, err := os.Stat("/sys/kernel/btf/vmlinux")
	if err == nil {
		// BTF is enabled, nothing to do
		return
	}

	log.Printf("disabling kexec due to upgrade to the BTF enabled kernel")

	if err = pkgkernel.WriteParam(&kernel.Param{
		Key:   "proc.sys.kernel.kexec_load_disabled",
		Value: "1",
	}); err != nil {
		log.Printf("failed to disable kexec: %s", err)
	}
}

// errataNetIfnames appends the `net.ifnames=0` kernel parameter to the kernel command line if upgrading
// from an old enough version of Talos.
func (i *Installer) errataNetIfnames() error {
	if i.cmdline.Get(constants.KernelParamNetIfnames).First() != nil {
		// net.ifnames is already set, nothing to do
		return nil
	}

	oldTalos, err := upgradeFromPreIfnamesTalos()
	if err != nil {
		return err
	}

	if oldTalos {
		log.Printf("appending net.ifnames=0 to the kernel command line")

		i.cmdline.Append(constants.KernelParamNetIfnames, "0")
	}

	return nil
}

func upgradeFromPreIfnamesTalos() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if _, err := os.Stat(constants.MachineSocketPath); err != nil {
		// old Talos version, include fallback
		return true, nil //nolint:nilerr
	}

	c, err := client.New(ctx,
		client.WithUnixSocket(constants.MachineSocketPath),
		client.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		return false, fmt.Errorf("error connecting to the machine service: %w", err)
	}

	defer c.Close() //nolint:errcheck

	// inject "fake" authorization
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(constants.APIAuthzRoleMetadataKey, string(role.Admin)))

	resp, err := c.Version(ctx)
	if err != nil {
		return false, fmt.Errorf("error getting Talos version: %w", err)
	}

	hostVersion := unpack(resp.Messages)

	talosVersion, err := compatibility.ParseTalosVersion(hostVersion.Version)
	if err != nil {
		return false, fmt.Errorf("error parsing Talos version: %w", err)
	}

	return talosVersion.DisablePredictableNetworkInterfaces(), nil
}
