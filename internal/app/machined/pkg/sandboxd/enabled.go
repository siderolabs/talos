// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package sandboxd

import (
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// Enabled reports whether the container plane should run inside the sandbox
// namespace (i.e. the sandboxd service should be started and CRI should run
// inside it).
//
// It is off in container mode, and otherwise gated on the SecurityProfileConfig
// machine config document: enabled only when the document is present with
// workloadIsolation=true. A missing document (e.g. a cluster upgraded from a Talos
// version that predates workload isolation) means disabled, preserving the old
// behavior.
func Enabled(r runtime.Runtime) bool {
	if r.State().Platform().Mode().InContainer() {
		return false
	}

	cfg := r.Config()
	if cfg == nil {
		return false
	}

	securityProfile := cfg.SecurityProfileConfig()

	return securityProfile != nil && securityProfile.WorkloadIsolation()
}
