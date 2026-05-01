// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows && !sidero.debug

package talos

import "github.com/spf13/cobra"

// registerDebugHostNsFlag is a no-op in release builds: the --host-ns flag is
// only available in debug (sidero.debug) builds. The server-side PROFILE_HOST_NS
// API is unaffected and remains available.
func registerDebugHostNsFlag(*cobra.Command) {}

// debugHostNsEnabled always reports false in release builds.
func debugHostNsEnabled() bool {
	return false
}
