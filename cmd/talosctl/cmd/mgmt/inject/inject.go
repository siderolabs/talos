// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package inject

import "github.com/spf13/cobra"

// Cmd represents the debug command.
var Cmd = &cobra.Command{
	Use:   "inject",
	Short: "Inject Talos API resources into Kubernetes manifests",
	Long:  ``,
}
