/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kernel

import (
	"io/ioutil"
	"strings"
)

// ParseProcCmdline parses /proc/cmdline and returns a map reprentation of the
// kernel parameters.
func ParseProcCmdline() (cmdline map[string]string, err error) {
	cmdline = map[string]string{}
	cmdlineBytes, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return
	}
	line := strings.TrimSuffix(string(cmdlineBytes), "\n")
	arguments := strings.Split(line, " ")
	for _, a := range arguments {
		kv := strings.Split(a, "=")
		if len(kv) == 2 {
			cmdline[kv[0]] = kv[1]
		}
	}

	return cmdline, err
}
