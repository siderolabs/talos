/* This Source Code Form is subject to the terms of the Mozilla Public
* License, v. 2.0. If a copy of the MPL was not distributed with this
* file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package digitalocean

import (
	"github.com/autonomy/talos/internal/pkg/userdata"
	//	 yaml "gopkg.in/yaml.v2"
)

const (
	// DOUserDataEndpoint is the local metadata endpoint inside of DO
	DOUserDataEndpoint = "http://169.254.169.254/metadata/v1/user-data"
)

// DigitalOcean is the concrete type that implements the platform.Platform interface.
type DigitalOcean struct{}

// Name implements the platform.Platform interface.
func (do *DigitalOcean) Name() string {
	return "DigitalOcean"
}

// UserData implements the platform.Platform interface.
func (do *DigitalOcean) UserData() (data userdata.UserData, err error) {
	//Configure nic
	return userdata.Download(DOUserDataEndpoint, nil)

}

// Prepare implements the platform.Platform interface.
func (do *DigitalOcean) Prepare(data userdata.UserData) (err error) {
	return nil
}

// Install installs talos
func (do *DigitalOcean) Install(data userdata.UserData) (err error) {
	return nil
}
