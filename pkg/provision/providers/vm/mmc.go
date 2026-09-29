// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/provision"
)

// MMCDiscardWorkaroundOptions returns the config bundle options disabling discards on QEMU-emulated SD cards.
//
// QEMU emulates an SD card erase by writing every 512-byte block one by one with a synchronous
// host write, while the vCPU (and the whole VM, via the big QEMU lock) is blocked in the MMIO exit.
// Talos discards the whole disk before installing, and mkfs discards each partition, so with an
// emulated SD card the VM freezes for a minute per discard and the guest reports RCU stalls.
// Disable discards on the emulated cards: the kernel then reports them as unsupported, and Talos
// falls back to wiping the headers only.
//
// The rule is delivered as a UdevRulesConfig document, so it is merged with UdevRulesConfig documents
// from user config patches, but it takes precedence over rules in the deprecated `.machine.udev.rules`
// field (this is how the machine config resolves the two).
//
// Returns nil if the cluster has no "mmc" disks, or if the Talos version doesn't support the UdevRulesConfig document.
func MMCDiscardWorkaroundOptions(clusterReq provision.ClusterRequest, contract *config.VersionContract) []bundle.Option {
	if !clusterReq.HasDiskDriver("mmc") || !contract.Greater(config.TalosVersion1_13) {
		return nil
	}

	udevRules := runtimecfg.NewUdevRulesConfigV1Alpha1()
	udevRules.UdevRules = []string{
		`ACTION=="add|change", SUBSYSTEM=="block", ENV{DEVTYPE}=="disk", KERNEL=="mmcblk*", ATTRS{name}=="QEMU!", ATTR{queue/discard_max_bytes}="0"`,
	}

	ctr, err := container.New(udevRules)
	if err != nil {
		panic(err)
	}

	return []bundle.Option{
		bundle.WithPatch([]configpatcher.Patch{configpatcher.NewStrategicMergePatch(ctr)}),
	}
}
