// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kspp implements KSPP kernel parameters enforcement.
package kspp

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/pkg/machinery/kernel"
)

// RequiredKSPPKernelParameters is the set of kernel parameters required to
// satisfy the KSPP.
var RequiredKSPPKernelParameters = procfs.Parameters{
	// init_on_alloc and init_on_free are not enforced, as they default to '1' in kernel config
	// this way they can be overridden via installer extra args in case of severe performance issues
	// procfs.NewParameter("init_on_alloc").Append("1"),
	// procfs.NewParameter("init_on_free").Append("1"),
	procfs.NewParameter("slab_nomerge").Append(""),
	procfs.NewParameter("pti").Append("on"),
}

// EnforceKSPPKernelParameters verifies that all required KSPP kernel
// parameters are present with the right value.
func EnforceKSPPKernelParameters() error {
	var result *multierror.Error

	for _, values := range RequiredKSPPKernelParameters {
		var val *string
		if val = procfs.ProcCmdline().Get(values.Key()).First(); val == nil {
			result = multierror.Append(result, fmt.Errorf("KSPP kernel parameter %s is required", values.Key()))

			continue
		}

		expected := values.First()
		if *val != *expected {
			result = multierror.Append(result, fmt.Errorf("KSPP kernel parameter %s was found with value %s, expected %s", values.Key(), *val, *expected))
		}
	}

	return result.ErrorOrNil()
}

// GetKernelParams returns the list of KSPP kernel parameters.
func GetKernelParams() []*kernel.Param {
	return []*kernel.Param{
		{
			Key:   "proc.sys.dev.tty.ldisc_autoload",
			Value: "0",
		},
		{
			Key:   "proc.sys.dev.tty.legacy_tiocsti",
			Value: "0",
		},
		{
			Key:   "proc.sys.fs.protected_symlinks",
			Value: "1",
		},
		{
			Key:   "proc.sys.fs.protected_hardlinks",
			Value: "1",
		},
		{
			Key:   "proc.sys.fs.protected_fifos",
			Value: "2",
		},
		{
			Key:   "proc.sys.fs.protected_regular",
			Value: "2",
		},
		{
			Key:   "proc.sys.fs.suid_dumpable",
			Value: "0",
		},
		{
			Key:   "proc.sys.kernel.kptr_restrict",
			Value: "2",
		},
		{
			Key:   "proc.sys.kernel.dmesg_restrict",
			Value: "1",
		},
		{
			Key:   "proc.sys.kernel.perf_event_paranoid",
			Value: "3",
		},
		{
			Key:   "proc.sys.kernel.randomize_va_space",
			Value: "2",
		},
		{
			// Bumping this to 3 (https://www.kernel.org/doc/Documentation/security/Yama.txt)
			// breaks Kubernetes pods with user namespaces, which are not enabled by default, but still supported.
			Key:   "proc.sys.kernel.yama.ptrace_scope",
			Value: "2",
		},
		{
			Key:   "proc.sys.user.max_user_namespaces",
			Value: "0",
		},
		{
			Key:   "proc.sys.kernel.unprivileged_bpf_disabled",
			Value: "1",
		},
		{
			Key:   "proc.sys.net.core.bpf_jit_harden",
			Value: "2",
		},
	}
}
