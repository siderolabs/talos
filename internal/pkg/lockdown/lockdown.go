// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lockdown reads the kernel lockdown level from securityfs.
package lockdown

import (
	"fmt"
	"os"
	"strings"

	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Path is the securityfs file exposing the kernel lockdown level.
const Path = "/sys/kernel/security/lockdown"

// ReadState reads the current kernel lockdown level from securityfs.
//
// The file lists every level with the active one in brackets, e.g.:
//
//	[none] integrity confidentiality
func ReadState(path string) (runtimeres.LockdownState, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return runtimeres.LockdownStateNone, err
	}

	for field := range strings.FieldsSeq(string(contents)) {
		level, ok := strings.CutPrefix(field, "[")
		if !ok {
			continue
		}

		return runtimeres.LockdownStateString(strings.TrimSuffix(level, "]"))
	}

	return runtimeres.LockdownStateNone, fmt.Errorf("failed to parse kernel lockdown level %q", string(contents))
}
