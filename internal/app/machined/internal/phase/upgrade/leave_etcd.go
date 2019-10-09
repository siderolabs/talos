/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package upgrade

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"
)

// LeaveEtcd represents the task for removing a control plane node from etcd.
type LeaveEtcd struct{}

// NewLeaveEtcdTask initializes and returns a LeaveEtcd task.
func NewLeaveEtcdTask() phase.Task {
	return &LeaveEtcd{}
}

// RuntimeFunc returns the runtime function.
func (task *LeaveEtcd) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.standard
}

func (task *LeaveEtcd) standard(args *phase.RuntimeArgs) (err error) {
	if args.Config().Machine().Type() == machine.Worker {
		return nil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	tlsInfo := transport.TLSInfo{
		CertFile:      constants.KubernetesEtcdPeerCert,
		KeyFile:       constants.KubernetesEtcdPeerKey,
		TrustedCAFile: constants.KubernetesEtcdCACert,
	}

	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return err
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	})
	if err != nil {
		return err
	}

	// nolint: errcheck
	defer cli.Close()

	resp, err := cli.MemberList(context.Background())
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
		return errors.Errorf("failed to find %q in list of etcd members", hostname)
	}

	log.Println("leaving etcd cluster")

	_, err = cli.MemberRemove(context.Background(), *id)
	if err != nil {
		return err
	}

	return nil
}
