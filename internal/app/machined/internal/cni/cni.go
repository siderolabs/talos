// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cni

import (
	specs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// Mounts returns the set of mounts required by the requested CNI plugin. All
// paths are relative to the root file system after switching the root.
func Mounts(config runtime.Configurator) ([]specs.Mount, error) {
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/etc/cni", Source: "/etc/cni", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/opt/cni", Source: "/opt/cni", Options: []string{"rbind", "rshared", "rw"}},
	}

	return mounts, nil
}
