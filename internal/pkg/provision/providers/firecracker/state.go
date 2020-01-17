// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"fmt"

	"github.com/talos-systems/talos/internal/pkg/provision"
)

type state struct {
	ProvisionerName string
	BridgeName      string

	ClusterInfo provision.ClusterInfo

	statePath string
}

func (s *state) Provisioner() string {
	return "firecracker"
}

func (s *state) Info() provision.ClusterInfo {
	return s.ClusterInfo
}

func (s *state) StatePath() (string, error) {
	if s.statePath == "" {
		return "", fmt.Errorf("state path is not set")
	}

	return s.statePath, nil
}
