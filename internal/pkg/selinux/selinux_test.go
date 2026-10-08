// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package selinux_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/selinux"
)

// TestCompile compiles the policy sources shipped in the rootfs with modules, the way machined does for SELinuxPolicyConfig.
//
// A neverallow on a rule of the base policy fails to compile, which proves the rule exists without a policy query tool.
func TestCompile(t *testing.T) {
	if _, err := os.Stat("/usr/bin/secilc"); err != nil {
		t.Skip("secilc and the policy sources are only available in the Talos rootfs")
	}

	for _, test := range []struct {
		name, module, wantErr string
	}{
		{
			"modules built on the macros",
			"(type pod_hostmon_t)\n(call pod_hostmon_domain (pod_hostmon_t))\n(type pod_cni_t)\n(call pod_privileged_domain (pod_cni_t))\n",
			"",
		},
		{
			"STATE stays out of reach of every pod domain",
			"(type pod_evil_t)\n(call pod_domain (pod_evil_t))\n(allow pod_evil_t system_state_t (fs_classes (ro)))\n",
			"neverallow check failed",
		},
		{
			"the kubelet relabels its plugin directories from pod_file_t at every start",
			"(neverallow kubelet_t pod_file_t (dir (relabelfrom relabelto)))\n",
			"neverallow check failed",
		},
		{
			"a domain the CRI starts outside the macros is a workload all the same",
			"(type x_t)\n(call pod_p (x_t))\n",
			"neverallow check failed",
		},
		{
			"a pod domain has the rights of pod_t, NFS volumes included",
			"(type pod_nfs_t)\n(call pod_domain (pod_nfs_t))\n(neverallow pod_nfs_t network_fs_t (file (write)))\n",
			"neverallow check failed",
		},
		{
			"an unknown type is reported with the module and the line",
			"(allow pod_t nonexistent_t (file (read)))\n",
			"module.cil:1",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := selinux.Compile(t.Context(), map[string]string{"module": test.module})

			if test.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantErr)
			}
		})
	}
}
