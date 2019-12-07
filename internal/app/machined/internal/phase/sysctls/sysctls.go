// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysctls

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/sysctl"
)

// Task represents the Task task.
type Task struct{}

// NewSysctlsTask initializes and returns a Task task.
func NewSysctlsTask() phase.Task {
	return &Task{}
}

// TaskFunc returns the runtime function.
func (task *Task) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.runtime
}

func (task *Task) runtime(r runtime.Runtime) error {
	var multiErr *multierror.Error

	if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.ipv4.ip_forward", Value: "1"}); err != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set net.ipv4.ip_forward: %w", err))
	}

	if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.bridge.bridge-nf-call-iptables", Value: "1"}); err != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set net.bridge.bridge-nf-call-iptables: %w", err))
	}

	if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.bridge.bridge-nf-call-ip6tables", Value: "1"}); err != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set net.bridge.bridge-nf-call-ip6tables: %w", err))
	}

	if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.ipv6.conf.default.forwarding", Value: "1"}); err != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set net.ipv6.conf.default.forwarding: %w", err))
	}

	if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "kernel.pid_max", Value: "262144"}); err != nil {
		multiErr = multierror.Append(multiErr, fmt.Errorf("failed to set kernel.pid_max: %w", err))
	}

	return multiErr.ErrorOrNil()
}
