/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/retry"
)

// LabelNodeAsMaster represents the LabelNodeAsMaster task.
type LabelNodeAsMaster struct{}

// NewLabelNodeAsMasterTask initializes and returns an Services task.
func NewLabelNodeAsMasterTask() phase.Task {
	return &LabelNodeAsMaster{}
}

// TaskFunc returns the runtime function.
func (task *LabelNodeAsMaster) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.standard
}

func (task *LabelNodeAsMaster) standard(r runtime.Runtime) (err error) {
	if r.Config().Machine().Type() == machine.Worker {
		return nil
	}

	endpoint := net.ParseIP(r.Config().Cluster().Endpoint())

	h, err := kubernetes.NewTemporaryClientFromPKI(r.Config().Cluster().CA().Crt, r.Config().Cluster().CA().Key, endpoint.String(), "6443")
	if err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	err = retry.Constant(10*time.Minute, retry.WithUnits(3*time.Second)).Retry(func() error {
		if err = h.LabelNodeAsMaster(hostname); err != nil {
			return retry.ExpectedError(err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to label node as master: %w", err)
	}

	return nil
}
