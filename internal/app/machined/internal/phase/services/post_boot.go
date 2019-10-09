/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"net"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/kubernetes"
)

// LabelNodeAsMaster represents the LabelNodeAsMaster task.
type LabelNodeAsMaster struct{}

// NewLabelNodeAsMasterTask initializes and returns an Services task.
func NewLabelNodeAsMasterTask() phase.Task {
	return &LabelNodeAsMaster{}
}

// RuntimeFunc returns the runtime function.
func (task *LabelNodeAsMaster) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.standard
}

func (task *LabelNodeAsMaster) standard(args *phase.RuntimeArgs) (err error) {
	if args.Config().Machine().Type() == machine.Worker {
		return nil
	}

	endpoint := net.ParseIP(args.Config().Cluster().IPs()[0])

	h, err := kubernetes.NewTemporaryClientFromPKI(args.Config().Cluster().CA().Crt, args.Config().Cluster().CA().Key, endpoint.String(), "6443")
	if err != nil {
		return err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	for i := 0; i < 200; i++ {
		if err = h.LabelNodeAsMaster(hostname); err == nil {
			return nil
		}

		time.Sleep(3 * time.Second)
	}

	return errors.New("failed to label node as master")
}
