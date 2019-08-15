/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package googlecloud

import (
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/internal/pkg/mount/manager"
	"github.com/talos-systems/talos/internal/pkg/mount/manager/owned"
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
	return userdata.Download(GCUserDataEndpoint, userdata.WithHeaders(map[string]string{"Metadata-Flavor": "Google"}))
}

// Initialize implements the platform.Platform interface and handles additional system setup.
func (gc *GoogleCloud) Initialize(data *userdata.UserData) (err error) {
	var mountpoints *mount.Points
	mountpoints, err = owned.MountPointsFromLabels()
	if err != nil {
		return err
	}

	m := manager.NewManager(mountpoints)

	return m.MountAll()
}
