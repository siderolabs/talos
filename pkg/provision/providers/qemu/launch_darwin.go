// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"os/exec"
)

func withNetworkContext(ctx context.Context, config *LaunchConfig, f func(config *LaunchConfig) error) error {
	panic("not implemented")
}

func checkPartitions(config *LaunchConfig) (bool, error) {
	panic("not implemented")
}

// startQemuCmd on darwin just runs cmd.Start.
func startQemuCmd(_ *LaunchConfig, cmd *exec.Cmd) error {
	return cmd.Start()
}
