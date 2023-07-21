// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package environment provides a set of functions to get environment variables.
package environment

import (
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Get the desired set of the environment variables based on kernel cmdline and machine config.
//
// The returned value is a list of strings in the form of "key=value".
func Get(cfg config.Config) []string {
	return GetCmdline(procfs.ProcCmdline(), cfg)
}

// GetCmdline the desired set of the environment variables based on kernel cmdline.
func GetCmdline(cmdline *procfs.Cmdline, cfg config.Config) []string {
	var result []string

	param := cmdline.Get(constants.KernelParamEnvironment)

	for idx := 0; ; idx++ {
		val := param.Get(idx)
		if val == nil {
			break
		}

		result = append(result, *val)
	}

	if cfg != nil && cfg.Machine() != nil {
		for k, v := range cfg.Machine().Env() {
			result = append(result, k+"="+v)
		}
	}

	return result
}
