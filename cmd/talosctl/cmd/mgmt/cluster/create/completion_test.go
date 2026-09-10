// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create_test

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"

	clustercmd "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	_ "github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create"
)

// TestConfigPatchFlagsHaveCompletion makes sure that every config patch flag of every cluster
// command offers `@file` completion, so that a newly added command cannot silently miss it.
func TestConfigPatchFlagsHaveCompletion(t *testing.T) {
	var walk func(cmd *cobra.Command, path string)

	walk = func(cmd *cobra.Command, path string) {
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			if !strings.HasPrefix(flag.Name, "config-patch") {
				return
			}

			_, ok := cmd.GetFlagCompletionFunc(flag.Name)

			assert.True(t, ok, "%s: --%s has no completion function registered", path, flag.Name)
		})

		for _, sub := range cmd.Commands() {
			walk(sub, path+" "+sub.Name())
		}
	}

	walk(clustercmd.Cmd, clustercmd.Cmd.Name())
}
