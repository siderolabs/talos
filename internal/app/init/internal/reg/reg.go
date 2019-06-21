/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sync/atomic"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/init/pkg/system"
	"github.com/talos-systems/talos/internal/app/init/proto"
	"github.com/talos-systems/talos/internal/pkg/archiver"
	"github.com/talos-systems/talos/internal/pkg/chunker/stream"
	"github.com/talos-systems/talos/internal/pkg/upgrade"
	"github.com/talos-systems/talos/pkg/userdata"
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

	// stop kubelet
	if _, err = r.Stop(ctx, &proto.StopRequest{Id: "kubelet"}); err != nil {
		return data, err
	}

	// kubeadm Reset
	if err = upgrade.Reset(); err != nil {
		return data, err
	}

	// Trigger reboot
	// we need to use defer to ensure we send back a response to the client.
	// we're calling this at the end of the stack so we can be sure
	// we dont boot the node in an err situation
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

// Stop implements the proto.InitServer interface and stops a
// service running on Talos.
func (r *Registrator) Stop(ctx context.Context, in *proto.StopRequest) (reply *proto.StopReply, err error) {
	if err = system.Services(r.Data).Stop(ctx, in.Id); err != nil {
		return &proto.StopReply{}, err
	}

	reply = &proto.StopReply{Resp: fmt.Sprintf("Service %q stopped", in.Id)}
	return reply, err
}

// CopyOut implements the proto.InitServer interface and copies data out of Talos node
func (r *Registrator) CopyOut(req *proto.CopyOutRequest, s proto.Init_CopyOutServer) error {
	path := req.RootPath
	path = filepath.Clean(path)

	if !filepath.IsAbs(path) {
		return errors.Errorf("path is not absolute %v", path)
	}

	pr, pw := io.Pipe()

	errCh := make(chan error, 1)

	ctx, ctxCancel := context.WithCancel(s.Context())
	defer ctxCancel()

	go func() {
		// nolint: errcheck
		defer pw.Close()
		errCh <- archiver.TarGz(ctx, path, pw)
	}()

	chunker := stream.NewChunker(pr)
	chunkCh := chunker.Read(ctx)

	for data := range chunkCh {
		err := s.SendMsg(&proto.StreamingData{Bytes: data})
		if err != nil {
			ctxCancel()
		}
	}

	archiveErr := <-errCh
	if archiveErr != nil {
		return s.SendMsg(&proto.StreamingData{Errors: archiveErr.Error()})
	}

	return nil
}
