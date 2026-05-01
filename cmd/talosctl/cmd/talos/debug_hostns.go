// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows && sidero.debug

package talos

import "github.com/spf13/cobra"

// debugHostNs holds the value of the --host-ns flag, registered only in debug builds.
var debugHostNs bool

// registerDebugHostNsFlag registers the --host-ns flag and appends its usage examples.
// It is only compiled into debug (sidero.debug) builds; release builds get the no-op stub.
func registerDebugHostNsFlag(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&debugHostNs, "host-ns", false,
		"run in the host mount namespace with image tools overlaid: run host binaries (zpool, etcdctl, …) directly without nsenter")

	cmd.Example += `

  # Run in the host mount namespace: host binaries (zpool, etcdctl, …) work directly,
  # Nix tools from nixos/nix are available on PATH — no nsenter needed
    talosctl debug --host-ns

  # Same, but pull a custom image for tools instead of the default nixos/nix
    talosctl debug --host-ns ghcr.io/myorg/my-tools:latest`
}

// debugHostNsEnabled reports whether the debug session should run in the host
// mount namespace (PROFILE_HOST_NS).
func debugHostNsEnabled() bool {
	return debugHostNs
}
