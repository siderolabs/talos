/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kernel

import (
	"io/ioutil"
	"strings"
)

// ReadProcCmdline reads /proc/cmdline.
func ReadProcCmdline() (cmdlineBytes []byte, err error) {
	cmdlineBytes, err = ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return nil, err
	}

	return cmdlineBytes, nil
}

// ParseProcCmdline parses /proc/cmdline and returns a map reprentation of the
// kernel parameters.
func ParseProcCmdline() (cmdline map[string]string, err error) {
	var cmdlineBytes []byte
	cmdlineBytes, err = ReadProcCmdline()
	if err != nil {
		return
	}

	cmdline = ParseKernelBootParameters(cmdlineBytes)

	return
}

// ParseKernelBootParameters parses kernel boot time parameters
//
// Ref: http://man7.org/linux/man-pages/man7/bootparam.7.html
func ParseKernelBootParameters(parameters []byte) (parsed map[string]string) {
	parsed = map[string]string{}

	line := strings.TrimSuffix(string(parameters), "\n")
	for _, arg := range strings.Fields(line) {
		kv := strings.SplitN(arg, "=", 2)
		// TODO: doesn't handle duplicate key names well (overwrites
		//       previous value)
		if len(kv) == 1 {
			parsed[kv[0]] = ""
		} else {
			parsed[kv[0]] = kv[1]
		}
	}

	return
}
