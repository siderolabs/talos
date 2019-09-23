/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package sysctls

import (
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/platform"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/sysctl"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Sysctls represents the Sysctls task.
type Sysctls struct{}

// NewSysctlsTask initializes and returns an UserData task.
func NewSysctlsTask() phase.Task {
	return &Sysctls{}
}

// RuntimeFunc returns the runtime function.
func (task *Sysctls) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *Sysctls) runtime(platform platform.Platform, data *userdata.UserData) error {
	var multiErr *multierror.Error

	if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.ipv4.ip_forward", Value: "1"}); err != nil {
		multiErr = multierror.Append(multiErr, errors.Wrapf(err, "failed to set IPv4 forwarding"))
	}
	if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "net.ipv6.conf.default.forwarding", Value: "1"}); err != nil {
		multiErr = multierror.Append(multiErr, errors.Wrap(err, "failed to set IPv6 forwarding"))
	}
	if err := sysctl.WriteSystemProperty(&sysctl.SystemProperty{Key: "kernel.pid_max", Value: "262144"}); err != nil {
		multiErr = multierror.Append(multiErr, errors.Wrap(err, "failed to set pid_max"))
	}

	return multiErr.ErrorOrNil()
}
