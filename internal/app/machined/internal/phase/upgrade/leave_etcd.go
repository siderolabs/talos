// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package upgrade

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
)

// LeaveEtcd represents the task for removing a control plane node from etcd.
type LeaveEtcd struct{}

// NewLeaveEtcdTask initializes and returns a LeaveEtcd task.
func NewLeaveEtcdTask() phase.Task {
	return &LeaveEtcd{}
}

// TaskFunc returns the runtime function.
func (task *LeaveEtcd) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.standard
}

func (task *LeaveEtcd) standard(r runtime.Runtime) (err error) {
	if r.Config().Machine().Type() == machine.Worker {
		return nil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	client, err := etcd.NewClient([]string{"127.0.0.1:2379"})
	if err != nil {
		return err
	}

	// nolint: errcheck
	defer client.Close()

	resp, err := client.MemberList(context.Background())
	if err != nil {
		return err
	}

	var id *uint64

	for _, member := range resp.Members {
		if member.Name == hostname {
			id = &member.ID
		}
	}

	if id == nil {
		return fmt.Errorf("failed to find %q in list of etcd members", hostname)
	}

	log.Println("leaving etcd cluster")

	_, err = client.MemberRemove(context.Background(), *id)
	if err != nil {
		return err
	}

	return nil
}
