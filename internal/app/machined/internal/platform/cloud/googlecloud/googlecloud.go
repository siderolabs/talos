/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package googlecloud

import (
	"github.com/talos-systems/talos/pkg/userdata"
)

const (
	// GCUserDataEndpoint is the local metadata endpoint inside of DO
	GCUserDataEndpoint = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/user-data"
)

// GoogleCloud is the concrete type that implements the platform.Platform interface.
type GoogleCloud struct{}

// Name implements the platform.Platform interface.
func (gc *GoogleCloud) Name() string {
	return "GoogleCloud"
}

// UserData implements the platform.Platform interface.
func (gc *GoogleCloud) UserData() (data *userdata.UserData, err error) {
	ud, err := userdata.Download(GCUserDataEndpoint, userdata.WithHeaders(map[string]string{"Metadata-Flavor": "Google"}))
	if err != nil {
		return nil, err
	}

	// TODO Readd this
	/*
		if ud.Networking == nil {
			ud.Networking = &userdata.Networking{
				OS: &userdata.OSNet{
					Devices: []userdata.Device{
						{
							Interface: network.DefaultInterface,
							DHCP:      true,
							MTU:       1460,
						},
					},
				},
			}
		}
	*/

	return ud, nil
}

// Prepare implements the platform.Platform interface and handles initial host preparation.
func (gc *GoogleCloud) Prepare(data *userdata.UserData) (err error) {
	return nil
}

// Install implements the platform.Platform interface and handles additional system setup.
func (gc *GoogleCloud) Install(data *userdata.UserData) (err error) {
	return nil
}
