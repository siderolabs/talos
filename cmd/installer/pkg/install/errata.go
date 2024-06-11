// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
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

// errataArm64ZBoot handles the case when upgrading from Talos < 1.8.0 on arm64 to compressed kernel.
func errataArm64ZBoot() {
	if runtime.GOARCH != "arm64" {
		return
	}

	oldConfig, err := os.OpenFile("/proc/config.gz", os.O_RDONLY, 0)
	if err != nil {
		log.Printf("failed to open /proc/config.gz: %s", err)

		return
	}

	defer oldConfig.Close() //nolint:errcheck

	r, err := gzip.NewReader(oldConfig)
	if err != nil {
		log.Printf("failed to read /proc/config.gz: %s", err)

		return
	}

	contents, err := io.ReadAll(r)
	if err != nil {
		log.Printf("failed to read /proc/config.gz: %s", err)

		return
	}

	if bytes.Contains(contents, []byte("CONFIG_ARM64_ZBOOT=y")) {
		// nothing to do
		return
	}

	log.Printf("disabling kexec due to upgrade to the compressed kernel")

	if err = pkgkernel.WriteParam(&kernel.Param{
		Key:   "proc.sys.kernel.kexec_load_disabled",
		Value: "1",
	}); err != nil {
		log.Printf("failed to disable kexec: %s", err)
	}
}

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
		client.WithGRPCDialOptions(grpc.WithTransportCredentials(insecure.NewCredentials())),
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
