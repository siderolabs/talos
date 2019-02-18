/* This Source Code Form is subject to the terms of the Mozilla Public
* License, v. 2.0. If a copy of the MPL was not distributed with this
* file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package googlecloud

import (
	"github.com/autonomy/talos/internal/pkg/userdata"
	//	 yaml "gopkg.in/yaml.v2"
)

const (
	// GCUserDataEndpoint is the local metadata endpoint inside of DO
	GCUserDataEndpoint = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/talos"
)

// GoogleCloud is the concrete type that implements the platform.Platform interface.
type GoogleCloud struct{}

// Name implements the platform.Platform interface.
func (gc *GoogleCloud) Name() string {
	return "GoogleCloud"
}

// UserData implements the platform.Platform interface.
func (gc *GoogleCloud) UserData() (data userdata.UserData, err error) {

	return userdata.Download(GCUserDataEndpoint, &map[string]string{"Metadata-Flavor": "Google"})

}

// Prepare implements the platform.Platform interface.
func (gc *GoogleCloud) Prepare(data userdata.UserData) (err error) {
	return nil
}

// Install installs talos
func (gc *GoogleCloud) Install(data userdata.UserData) (err error) {
	return nil
}
