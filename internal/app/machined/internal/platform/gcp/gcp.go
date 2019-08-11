/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package gcp

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
)

const (
	// GCUserDataEndpoint is the local metadata endpoint inside of DO
	GCUserDataEndpoint = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/user-data"
)

// GCP is the concrete type that implements the platform.Platform interface.
type GCP struct{}

// Name implements the platform.Platform interface.
func (gc *GCP) Name() string {
	return "GCP"
}

// UserData implements the platform.Platform interface.
func (gc *GCP) UserData() (data *userdata.UserData, err error) {
	return userdata.Download(GCUserDataEndpoint, userdata.WithHeaders(map[string]string{"Metadata-Flavor": "Google"}))
}

// Mode implements the platform.Platform interface.
func (gc *GCP) Mode() runtime.Mode {
	return runtime.Cloud
}

// Hostname implements the platform.Platform interface.
func (gc *GCP) Hostname() (hostname []byte, err error) {
	return nil, nil
}
