// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package sandboxd

import (
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// ServiceEnabled reports whether the sandboxd service should run on this node.
//
// It is off in container mode (the container runtime provides the isolation) and
// in agent mode (which never runs the container plane); everywhere else it is on.
//
// sandboxd takes no configuration of its own, so this deliberately does not look
// at the machine config: the service is started early, before the config is
// loaded, so that the sandbox launcher is already published by the time any
// service that may need it is launched. Whether the container plane actually runs
// inside the namespace is a separate, configuration-driven decision, see
// [IsolationEnabled].
func ServiceEnabled(r runtime.Runtime) bool {
	mode := r.State().Platform().Mode()

	return !mode.InContainer() && !mode.IsAgent()
}

// IsolationEnabled reports whether the container plane should run inside the
// sandbox namespace.
//
// It is off wherever sandboxd does not run, and otherwise gated on the
// SecurityProfileConfig machine config document: enabled only when the document is
// present with workloadIsolation=true. A missing document (e.g. a cluster upgraded
// from a Talos version that predates workload isolation) means disabled,
// preserving the old behavior.
//
// This must only be consulted once the machine configuration is available, as a
// missing configuration is indistinguishable from isolation being disabled. The
// CRI service is gated on the configuration by cri.ServiceController.
func IsolationEnabled(r runtime.Runtime) bool {
	if !ServiceEnabled(r) {
		return false
	}

	cfg := r.Config()
	if cfg == nil {
		return false
	}

	securityProfile := cfg.SecurityProfileConfig()

	return securityProfile != nil && securityProfile.WorkloadIsolation()
}
