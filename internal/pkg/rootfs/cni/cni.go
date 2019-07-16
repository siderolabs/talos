/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cni

import (
	"os"
	"path"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Setup prepares the root file system for the requested CNI plugin.
func Setup(s string, data *userdata.UserData) error {
	paths := []string{"/etc/cni/net.d"}

	switch data.Services.Init.CNI {
	case constants.CNICalico:
		paths = append(paths, "/run/calico", "/var/lib/calico")
	case constants.CNIFlannel:
		paths = append(paths, "/run/flannel")
	default:
		return errors.Errorf("unknown CNI %s", data.Services.Init.CNI)
	}

	for _, p := range paths {
		if err := os.MkdirAll(path.Join(s, p), os.ModeDir); err != nil {
			return errors.Wrapf(err, "failed to create directory %s", path.Join(s, p))
		}
	}

	return nil
}

// Mounts returns the set of mounts required by the requested CNI plugin. All
// paths are relative to the root file system after switching the root.
func Mounts(data *userdata.UserData) ([]specs.Mount, error) {
	mounts := []specs.Mount{
		{Type: "bind", Destination: "/etc/cni", Source: "/etc/cni", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/opt/cni", Source: "/opt/cni", Options: []string{"rbind", "rshared", "rw"}},
	}

	switch data.Services.Init.CNI {
	case constants.CNICalico:
		calicoMounts := []specs.Mount{
			{Type: "bind", Destination: "/var/lib/calico", Source: "/var/lib/calico", Options: []string{"rbind", "rshared", "rw"}},
		}
		mounts = append(mounts, calicoMounts...)
	case constants.CNIFlannel:
		// Nothing to do.
	default:
		return nil, errors.Errorf("unknown CNI %s", data.Services.Init.CNI)
	}

	return mounts, nil
}
