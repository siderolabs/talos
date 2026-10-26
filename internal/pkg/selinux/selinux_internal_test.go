// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package selinux

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMDADMRuntimePolicyNamedCreationPreservesDefaults(t *testing.T) {
	t.Parallel()

	policy, err := os.ReadFile("policy/selinux/services/udev.cil")
	require.NoError(t, err)

	// mdmonitor starts in init_t; a udev mdadm invocation may also create the
	// directory. Only the basename "mdadm" gets the dedicated runtime type.
	var transitions []string

	for line := range strings.Lines(string(policy)) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "(typetransition ") && strings.Contains(line, "mdadm_run_t") {
			transitions = append(transitions, line)
		}
	}

	assert.Equal(t, []string{
		`(typetransition init_t run_t dir "mdadm" mdadm_run_t)`,
		`(typetransition udev_t run_t dir "mdadm" mdadm_run_t)`,
	}, transitions)

	for _, class := range []string{"file", "dir", "lnk_file", "chr_file", "blk_file", "fifo_file"} {
		t.Run(class, func(t *testing.T) {
			t.Parallel()

			assert.Contains(t, string(policy), "(typetransition udev_t run_t "+class+" udev_run_t)")
		})
	}

	// Generic /run stays add-only for udev.
	assert.Contains(t, string(policy), "(allow udev_t run_t (dir (add_name write)))")
	assert.NotContains(t, string(policy), "(allow udev_t run_t (dir (remove_name")
}

func TestMDADMRuntimePolicyMapLifecycle(t *testing.T) {
	t.Parallel()

	policy, err := os.ReadFile("policy/selinux/services/udev.cil")
	require.NoError(t, err)

	assert.Contains(t, string(policy), "(type mdadm_run_t)")
	assert.Contains(t, string(policy), "(call system_f (mdadm_run_t))")
	assert.NotContains(t, string(policy), "(call filesystem_f (mdadm_run_t))")
	assert.NotContains(t, string(policy), "(call common_f (mdadm_run_t))")

	var allows []string

	for line := range strings.Lines(string(policy)) {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "(allow ") && strings.Contains(line, "mdadm_run_t") {
			allows = append(allows, line)
		}
	}

	// Explicit directory and regular-file operations only: no fs_classes,
	// executable, device, socket, relabel or unconfined shortcut.
	assert.Equal(t, []string{
		"(allow mdadm_run_t tmpfs_t (filesystem (associate)))",
		"(allow init_t mdadm_run_t (dir (create getattr search open read write add_name remove_name)))",
		"(allow init_t mdadm_run_t (file (create read write open getattr lock rename unlink)))",
		"(allow udev_t mdadm_run_t (dir (create getattr search open read write add_name remove_name)))",
		"(allow udev_t mdadm_run_t (file (create read write open getattr lock rename unlink)))",
	}, allows)
}
