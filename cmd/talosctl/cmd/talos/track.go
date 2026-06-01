// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"time"

	"github.com/spf13/cobra"
)

type trackableActionCmdFlags struct {
	wait    bool
	debug   bool
	timeout time.Duration
}

func (f *trackableActionCmdFlags) addTrackActionFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.wait, "wait", true, "wait for the operation to complete, tracking its progress. always set to true when --debug is set")
	cmd.Flags().BoolVar(&f.debug, "debug", false, "debug operation from kernel logs. --wait is set to true when this flag is set")
	cmd.Flags().DurationVar(&f.timeout, "timeout", 30*time.Minute, "time to wait for the operation is complete if --debug or --wait is set")
}
