// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package networkd

import (
	"fmt"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
)

// buildOptions translates the supplied config to nic.Option used for
// configuring the interface.
// nolint: gocyclo
func buildOptions(device machine.Device) (name string, opts []nic.Option, err error) {
	opts = append(opts, nic.WithName(device.Interface))

	if device.Ignore || kernel.ProcCmdline().Get(constants.KernelParamNetworkInterfaceIgnore).Contains(device.Interface) {
		opts = append(opts, nic.WithIgnore())
		return device.Interface, opts, err
	}

	// Configure Addressing
	switch {
	case device.CIDR != "":
		s := &address.Static{Device: &device}
		opts = append(opts, nic.WithAddressing(s))
	default:
		d := &address.DHCP{}
		opts = append(opts, nic.WithAddressing(d))
	}

	// Configure Bonding
	if device.Bond == nil {
		return device.Interface, opts, err
	}

	opts = append(opts, nic.WithBond(true))

	if len(device.Bond.Interfaces) == 0 {
		return device.Interface, opts, fmt.Errorf("invalid bond configuration for %s: must supply sub interfaces for bonded interface", device.Interface)
	}

	opts = append(opts, nic.WithSubInterface(device.Bond.Interfaces...))

	if device.Bond.Mode != "" {
		opts = append(opts, nic.WithBondMode(device.Bond.Mode))
	}

	if device.Bond.HashPolicy != "" {
		opts = append(opts, nic.WithHashPolicy(device.Bond.HashPolicy))
	}

	if device.Bond.LACPRate != "" {
		opts = append(opts, nic.WithLACPRate(device.Bond.LACPRate))
	}

	if device.Bond.MIIMon > 0 {
		opts = append(opts, nic.WithMIIMon(device.Bond.MIIMon))
	}

	if device.Bond.UpDelay > 0 {
		opts = append(opts, nic.WithUpDelay(device.Bond.UpDelay))
	}

	if device.Bond.DownDelay > 0 {
		opts = append(opts, nic.WithDownDelay(device.Bond.DownDelay))
	}

	if !device.Bond.UseCarrier {
		opts = append(opts, nic.WithUseCarrier(device.Bond.UseCarrier))
	}

	if device.Bond.ARPInterval > 0 {
		opts = append(opts, nic.WithARPInterval(device.Bond.ARPInterval))
	}

	// if device.Bond.ARPIPTarget {
	//	opts = append(opts, nic.WithUseCarrier(device.Bond.UseCarrier))
	//}

	if device.Bond.ARPValidate != "" {
		opts = append(opts, nic.WithUseCarrier(device.Bond.UseCarrier))
	}

	if device.Bond.ARPAllTargets != "" {
		opts = append(opts, nic.WithUseCarrier(device.Bond.UseCarrier))
	}

	if device.Bond.Primary != "" {
		opts = append(opts, nic.WithUseCarrier(device.Bond.UseCarrier))
	}

	if device.Bond.PrimaryReselect != "" {
		opts = append(opts, nic.WithPrimaryReselect(device.Bond.PrimaryReselect))
	}

	if device.Bond.FailOverMac != "" {
		opts = append(opts, nic.WithFailOverMAC(device.Bond.FailOverMac))
	}

	if device.Bond.ResendIGMP > 0 {
		opts = append(opts, nic.WithResendIGMP(device.Bond.ResendIGMP))
	}

	if device.Bond.NumPeerNotif > 0 {
		opts = append(opts, nic.WithNumPeerNotif(device.Bond.NumPeerNotif))
	}

	if device.Bond.AllSlavesActive > 0 {
		opts = append(opts, nic.WithAllSlavesActive(device.Bond.AllSlavesActive))
	}

	if device.Bond.MinLinks > 0 {
		opts = append(opts, nic.WithMinLinks(device.Bond.MinLinks))
	}

	if device.Bond.LPInterval > 0 {
		opts = append(opts, nic.WithLPInterval(device.Bond.LPInterval))
	}

	if device.Bond.PacketsPerSlave > 0 {
		opts = append(opts, nic.WithPacketsPerSlave(device.Bond.PacketsPerSlave))
	}

	if device.Bond.ADSelect != "" {
		opts = append(opts, nic.WithADSelect(device.Bond.ADSelect))
	}

	if device.Bond.ADActorSysPrio > 0 {
		opts = append(opts, nic.WithADActorSysPrio(device.Bond.ADActorSysPrio))
	}

	if device.Bond.ADUserPortKey > 0 {
		opts = append(opts, nic.WithADUserPortKey(device.Bond.ADUserPortKey))
	}

	// if device.Bond.ADActorSystem != "" {
	//	opts = append(opts, nic.WithADActorSystem(device.Bond.ADActorSystem))
	//}

	if device.Bond.TLBDynamicLB > 0 {
		opts = append(opts, nic.WithTLBDynamicLB(device.Bond.TLBDynamicLB))
	}

	if device.Bond.PeerNotifyDelay > 0 {
		opts = append(opts, nic.WithPeerNotifyDelay(device.Bond.PeerNotifyDelay))
	}

	return device.Interface, opts, err
}
