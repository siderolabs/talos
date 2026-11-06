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

	// compile compiles the policy sources with the test statements and returns the output of secilc.
	compile := func(t *testing.T, statements string) (string, error) {
		dir := t.TempDir()
		module := filepath.Join(dir, "test.cil")

		require.NoError(t, os.WriteFile(module, []byte(statements), 0o644))

		cmd := exec.CommandContext(t.Context(), secilc, append([]string{"-c", "33", "-O", "-o", filepath.Join(dir, "policy.33"), "-f", filepath.Join(dir, "file_contexts")}, append(sources, module)...)...)
		output, err := cmd.CombinedOutput()

		return string(output), err
	}

	for _, test := range []struct {
		name, statements, wantErr string
	}{
		{
			"the booleans exist",
			"(booleanif pod_write_cni_plugins (true (allow pod_t pod_t (file (getattr)))))\n" +
				"(booleanif pod_use_network_fs (true (allow pod_t pod_t (file (getattr)))))\n" +
				"(booleanif pod_use_ebpf (true (allow pod_t pod_t (file (getattr)))))\n" +
				"(booleanif pod_use_control_plane_files (true (allow pod_t pod_t (file (getattr)))))\n",
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
			"each domain has the rights of its role only, and the process monitor writes no log",
			"(neverallow pod_cni_t network_fs_t (file (write)))\n(neverallow pod_cni_t kernel_t (system (ipc_info)))\n" +
				"(neverallow pod_csi_t cni_plugin_t (file (write)))\n(neverallow pod_csi_t bpf_t (file (write)))\n" +
				"(neverallow pod_netfs_t init_t (file (read)))\n(neverallow pod_ebpf_t init_t (file (read)))\n" +
				"(neverallow pod_hostmon_t log_f (file (write append)))\n",
			"",
		},
		{
			"each static pod of the control plane reads its own secrets, and no other domain uses them whatever the booleans say",
			"(neverallow kube_scheduler_t kube_apiserver_secret_t (file (read)))\n" +
				"(neverallow kube_apiserver_t kube_controller_manager_secret_t (file (read)))\n" +
				"(neverallow pod_hostmon_t kube_secret_f (file (read)))\n(neverallow pod_privileged_t kube_secret_f (file (read)))\n" +
				"(neverallow pod_cni_t kube_secret_f (file (read)))\n(neverallow pod_csi_t kube_secret_f (file (read)))\n" +
				"(neverallow kube_controller_manager_t kube_log_t (file (write)))\n(neverallow pod_privileged_t kube_log_t (file (write)))\n",
			"",
		},
		{
			"the CNI agent reads no host file outside its role, such as the configuration of the kubelet, and only the CRI connects to its sockets",
			"(neverallow pod_cni_t k8s_conf_t (file (read)))\n(neverallow pod_cni_t bin_exec_t (file (read)))\n" +
				"(neverallow pod_t pod_cni_t (unix_stream_socket (connectto)))\n",
			"",
		},
		{
			"the CRI relabels a volume formatted without labels",
			"(neverallow pod_containerd_t unlabeled_t (dir (relabelfrom)))\n",
			"neverallow check failed",
		},
		{
			"the CSI node plugin and a privileged container drive the disks through io_uring",
			"(neverallow pod_csi_t self (io_uring (sqpoll)))\n",
			"neverallow check failed",
		},
		{
			"the CSI node plugin tunes the virtual storage devices in sysfs",
			"(neverallow pod_csi_t sys_storage_t (file (write)))\n",
			"neverallow check failed",
		},
		{
			"neither the CNI agent nor a regular pod uses io_uring, and no workload writes the rest of sysfs",
			"(neverallow pod_cni_t self (io_uring (allowed)))\n(neverallow pod_t self (io_uring (allowed)))\n" +
				"(neverallow workload_p sysfs_t (file (write)))\n",
			"",
		},
		{
			"machined kexecs the kernel and the initramfs it holds in its memfds",
			"(neverallow init_t self (system (kexec_image_load kexec_initramfs_load)))\n",
			"neverallow check failed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, err := compile(t, test.statements)

			if test.wantErr == "" {
				require.NoError(t, err, output)
			} else {
				require.Error(t, err)
				require.Contains(t, output, test.wantErr)
			}
		})
	}

	// secilc traces a rule the policy grants whatever the booleans say through calls only, and a rule a boolean grants
	// through a booleanif.
	const unconditional = `<root>\n(\s+call at .+\n)*\s+allow at`

	// The domains that need the right of a boolean keep it whatever the boolean says.
	for domain, rights := range map[string][]string{
		"pod_cni_t":        {"cni_plugin_t", "bpf_t"},
		"pod_csi_t":        {"network_fs_t"},
		"pod_netfs_t":      {"network_fs_t"},
		"pod_ebpf_t":       {"bpf_t"},
		"pod_privileged_t": {"cni_plugin_t", "network_fs_t", "bpf_t"},
	} {
		for _, right := range rights {
			t.Run(domain+" writes "+right+" whatever the booleans say", func(t *testing.T) {
				output, err := compile(t, "(neverallow "+domain+" "+right+" (file (write)))\n")

				require.Error(t, err)
				require.Regexp(t, unconditional, output)
			})
		}
	}

	// The static pods of the control plane use their files whatever the booleans say, and pod_t only under
	// pod_use_control_plane_files.
	for _, test := range []struct{ domain, file, permission string }{
		{"kube_apiserver_t", "kube_apiserver_secret_t", "read"},
		{"kube_apiserver_t", "kube_log_t", "write"},
		{"kube_controller_manager_t", "kube_controller_manager_secret_t", "read"},
		{"kube_scheduler_t", "kube_scheduler_config_t", "read"},
		{"pod_t", "kube_apiserver_secret_t", "read"},
		{"pod_t", "kube_log_t", "write"},
	} {
		t.Run(test.domain+" "+test.permission+"s "+test.file, func(t *testing.T) {
			output, err := compile(t, "(neverallow "+test.domain+" "+test.file+" (file ("+test.permission+")))\n")

			require.Error(t, err)

			if test.domain == "pod_t" {
				require.NotRegexp(t, unconditional, output)
				require.Contains(t, output, "booleanif at")
			} else {
				require.Regexp(t, unconditional, output)
			}
		})
	}

	// A node may refuse a pod domain: the CRI enters it only under its boolean, and enters pod_t whatever the booleans
	// say.
	for _, domain := range []string{"pod_t", "pod_netfs_t", "pod_ebpf_t", "pod_hostmon_t", "pod_cni_t", "pod_csi_t", "pod_privileged_t"} {
		t.Run("the CRI enters "+domain, func(t *testing.T) {
			output, err := compile(t, "(neverallow pod_containerd_t "+domain+" (process (transition dyntransition)))\n")

			require.Error(t, err)

			if domain == "pod_t" {
				require.Regexp(t, unconditional, output)
			} else {
				require.NotRegexp(t, unconditional, output)
				require.Contains(t, output, "booleanif at")
			}
		})
	}
}
