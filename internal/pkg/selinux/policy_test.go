// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package selinux_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPolicy compiles the policy sources of the tree with test statements.
//
// A neverallow on a rule of the policy fails to compile, which proves the rule exists without a policy query tool, and
// an allow rule that a neverallow of the policy forbids fails as well.
func TestPolicy(t *testing.T) {
	secilc, err := exec.LookPath("secilc")
	if err != nil {
		t.Skip("secilc is not available")
	}

	sources, err := filepath.Glob("policy/selinux/*/*.cil")
	require.NoError(t, err)
	require.NotEmpty(t, sources)

	for _, test := range []struct {
		name, statements, wantErr string
	}{
		{
			"the booleans exist",
			"(booleanif pod_write_cni_plugins (true (allow pod_t pod_t (file (getattr)))))\n" +
				"(booleanif pod_use_network_fs (true (allow pod_t pod_t (file (getattr)))))\n" +
				"(booleanif pod_use_ebpf (true (allow pod_t pod_t (file (getattr)))))\n",
			"",
		},
		{
			"STATE stays out of reach of every pod domain",
			"(allow pod_cni_t system_state_t (file (read)))\n",
			"neverallow check failed",
		},
		{
			"no pod domain connects to machined",
			"(allow pod_hostmon_t init_t (unix_stream_socket (connectto)))\n",
			"neverallow check failed",
		},
		{
			"a domain the CRI starts outside the macros is a workload all the same",
			"(type x_t)\n(call pod_p (x_t))\n",
			"neverallow check failed",
		},
		{
			"only init and machined set the booleans",
			"(allow pod_privileged_t security_t (security (setbool)))\n",
			"neverallow check failed",
		},
		{
			"the process monitor reads the /proc of every domain",
			"(neverallow pod_hostmon_t init_t (file (read)))\n",
			"neverallow check failed",
		},
		{
			"the CSI node plugin reads the host like a privileged container",
			"(neverallow pod_csi_t privileged_readable_f (file (read)))\n",
			"neverallow check failed",
		},
		{
			"each role domain has the rights of its role only, and the process monitor writes no log",
			"(neverallow pod_cni_t network_fs_t (file (write)))\n(neverallow pod_cni_t kernel_t (system (ipc_info)))\n" +
				"(neverallow pod_csi_t cni_plugin_t (file (write)))\n(neverallow pod_csi_t bpf_t (file (write)))\n" +
				"(neverallow pod_hostmon_t log_f (file (write append)))\n",
			"",
		},
		{
			"a privileged container keeps the network filesystems of the CSI node plugin",
			"(neverallow pod_privileged_t network_fs_t (file (write)))\n",
			"neverallow check failed",
		},
		{
			"a privileged container keeps the CNI plugins of the CNI agent",
			"(neverallow pod_privileged_t cni_plugin_t (file (write)))\n",
			"neverallow check failed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			module := filepath.Join(dir, "test.cil")

			require.NoError(t, os.WriteFile(module, []byte(test.statements), 0o644))

			cmd := exec.CommandContext(t.Context(), secilc, append([]string{"-c", "33", "-O", "-o", filepath.Join(dir, "policy.33"), "-f", filepath.Join(dir, "file_contexts")}, append(sources, module)...)...)
			output, err := cmd.CombinedOutput()

			if test.wantErr == "" {
				require.NoError(t, err, string(output))
			} else {
				require.Error(t, err)
				require.Contains(t, string(output), test.wantErr)
			}
		})
	}
}
