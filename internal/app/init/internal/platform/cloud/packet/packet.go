/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package packet

import (
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/install"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/userdata"
)

const (
	// PacketUserDataEndpoint is the local metadata endpoint for Packet.
	PacketUserDataEndpoint = "https://metadata.packet.net/userdata"
)

// Packet is a discoverer for non-cloud environments.
type Packet struct{}

// Name implements the platform.Platform interface.
func (p *Packet) Name() string {
	return "Packet"
}

// UserData implements the platform.Platform interface.
func (p *Packet) UserData() (data *userdata.UserData, err error) {
	return userdata.Download(PacketUserDataEndpoint, nil)
}

// Prepare implements the platform.Platform interface.
func (p *Packet) Prepare(data *userdata.UserData) (err error) {
	return install.Prepare(data)
}

// Install provides the functionality to install talos by
// download the necessary bits and write them to a target device
// nolint: dupl
func (p *Packet) Install(data *userdata.UserData) (err error) {
	var cmdlineBytes []byte
	cmdlineBytes, err = kernel.ReadProcCmdline()
	if err != nil {
		return err
	}
	if err = install.Install(string(cmdlineBytes), data); err != nil {
		return errors.Wrap(err, "failed to install")
	}

	return nil
}
