// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package debug

import (
	"os/exec"

	"github.com/siderolabs/go-cmd/pkg/cmd/proc/reaper"
)

// WaitHostNsCommand exposes the waitHostNsCommand function for testing purposes only.
func WaitHostNsCommand(cmd *exec.Cmd, usingReaper bool, notifyCh chan reaper.ProcessInfo) (int, error) {
	return waitHostNsCommand(cmd, usingReaper, notifyCh)
}
