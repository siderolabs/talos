/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"sync/atomic"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/talos-systems/talos/internal/app/init/pkg/system"
	"github.com/talos-systems/talos/internal/app/init/proto"
	"github.com/talos-systems/talos/internal/pkg/upgrade"
	"github.com/talos-systems/talos/pkg/userdata"
	"google.golang.org/grpc"
)

// Registrator is the concrete type that implements the factory.Registrator and
// proto.Init interfaces.
type Registrator struct {
	Data *userdata.UserData

	ShutdownCh chan struct{}
	RebootCh   chan struct{}

	rebootCalled uint32
}

// NewRegistrator builds new Registrator instance
func NewRegistrator(data *userdata.UserData) *Registrator {
	return &Registrator{
		Data:       data,
		ShutdownCh: make(chan struct{}),
		RebootCh:   make(chan struct{}),
	}
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	proto.RegisterInitServer(s, r)
}

// Reboot implements the proto.InitServer interface.
func (r *Registrator) Reboot(ctx context.Context, in *empty.Empty) (reply *proto.RebootReply, err error) {
	reply = &proto.RebootReply{}

	// make sure channel is closed only once (and initiate either reboot or shutdown)
	if atomic.CompareAndSwapUint32(&r.rebootCalled, 0, 1) {
		close(r.RebootCh)
	}

	return
}

// Shutdown implements the proto.InitServer interface.
func (r *Registrator) Shutdown(ctx context.Context, in *empty.Empty) (reply *proto.ShutdownReply, err error) {
	reply = &proto.ShutdownReply{}

	// make sure channel is closed only once (and initiate either reboot or shutdown)
	if atomic.CompareAndSwapUint32(&r.rebootCalled, 0, 1) {
		close(r.ShutdownCh)
	}

	return
}

// Upgrade initiates a Talos upgrade
func (r *Registrator) Upgrade(ctx context.Context, in *proto.UpgradeRequest) (data *proto.UpgradeReply, err error) {

	if err = upgrade.NewUpgrade(in.Url); err != nil {
		return data, err
	}

	// Trigger reboot
	defer func() {
		if _, err = r.Reboot(ctx, &empty.Empty{}); err != nil {
			return
		}
	}()

	// profit
	data = &proto.UpgradeReply{Ack: "Upgrade completed, rebooting node"}
	return data, err
}

// ServiceList returns list of the registered services and their status
func (r *Registrator) ServiceList(ctx context.Context, in *empty.Empty) (result *proto.ServiceListReply, err error) {
	services := system.Services(r.Data).List()

	result = &proto.ServiceListReply{
		Services: make([]*proto.ServiceInfo, len(services)),
	}

	for i := range services {
		result.Services[i] = services[i].AsProto()
	}

	return result, nil
}
