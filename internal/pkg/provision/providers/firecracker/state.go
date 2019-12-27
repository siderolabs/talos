// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/internal/pkg/provision"
)

type state struct {
	tempDir             string
	baseConfigURL       string
	bridgeInterfaceName string

	talosConfig *config.Config

	clusterInfo provision.ClusterInfo
}

func (s *state) TalosConfig() *config.Config {
	return s.talosConfig
}

func (s *state) Info() provision.ClusterInfo {
	return s.clusterInfo
}
