// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"fmt"

	"github.com/talos-systems/talos/pkg/provision"
)

type result struct {
	clusterInfo provision.ClusterInfo
}

func (res *result) Provisioner() string {
	return "docker"
}

func (res *result) Info() provision.ClusterInfo {
	return res.clusterInfo
}

func (res *result) StatePath() (string, error) {
	return "", fmt.Errorf("state path is not used for docker provisioner")
}
