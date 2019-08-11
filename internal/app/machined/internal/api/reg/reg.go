/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/machined/internal/event"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/machined/proto"
	"github.com/talos-systems/talos/internal/pkg/upgrade"
	"github.com/talos-systems/talos/pkg/archiver"
	"github.com/talos-systems/talos/pkg/chunker/stream"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// OSPathSeparator is the string version of the os.PathSeparator
const OSPathSeparator = string(os.PathSeparator)

// Registrator is the concrete type that implements the factory.Registrator and
// proto.Init interfaces.
type Registrator struct {
	Data *userdata.UserData
}

// NewRegistrator builds new Registrator instance
func NewRegistrator(data *userdata.UserData) *Registrator {
	return &Registrator{
		Data: data,
	}
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	proto.RegisterInitServer(s, r)
}

// Reboot implements the proto.InitServer interface.
func (r *Registrator) Reboot(ctx context.Context, in *empty.Empty) (reply *proto.RebootReply, err error) {
	reply = &proto.RebootReply{}

	log.Printf("reboot via API received")
	event.Bus().Publish(event.Reboot)

	return
}

// Shutdown implements the proto.InitServer interface.
func (r *Registrator) Shutdown(ctx context.Context, in *empty.Empty) (reply *proto.ShutdownReply, err error) {
	reply = &proto.ShutdownReply{}

	log.Printf("shutdown via API received")
	event.Bus().Publish(event.Shutdown)

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

// Reset initiates a Talos upgrade
func (r *Registrator) Reset(ctx context.Context, in *empty.Empty) (data *proto.ResetReply, err error) {
	// Stop the kubelet.
	if _, err = r.Stop(ctx, &proto.StopRequest{Id: "kubelet"}); err != nil {
		return data, err
	}

	// Run `kubeadm reset`.
	if err = upgrade.Reset(); err != nil {
		return data, err
	}

	// Remove the machine config.
	if err = os.Remove(constants.UserDataPath); err != nil {
		return nil, err
	}

	return &proto.ResetReply{}, err
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

// Start implements the proto.InitServer interface and starts a
// service running on Talos.
func (r *Registrator) Start(ctx context.Context, in *proto.StartRequest) (reply *proto.StartReply, err error) {
	if err = system.Services(r.Data).Start(in.Id); err != nil {
		return &proto.StartReply{}, err
	}

	reply = &proto.StartReply{Resp: fmt.Sprintf("Service %q started", in.Id)}
	return reply, err
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

// LS implements the proto.InitServer interface.
func (r *Registrator) LS(req *proto.LSRequest, s proto.Init_LSServer) error {

	if req == nil {
		req = new(proto.LSRequest)
	}
	if !strings.HasPrefix(req.Root, OSPathSeparator) {
		// Make sure we use complete paths
		req.Root = OSPathSeparator + req.Root
	}
	req.Root = strings.TrimSuffix(req.Root, OSPathSeparator)
	if req.Root == "" {
		req.Root = "/"
	}

	var maxDepth int
	if req.Recurse {
		if req.RecursionDepth == 0 {
			maxDepth = -1
		} else {
			maxDepth = int(req.RecursionDepth)
		}
	}

	files, err := archiver.Walker(s.Context(), req.Root, archiver.WithMaxRecurseDepth(maxDepth))
	if err != nil {
		return err
	}

	for fi := range files {
		if fi.Error != nil {
			err = s.Send(&proto.FileInfo{
				Name:         fi.FullPath,
				RelativeName: fi.RelPath,
				Error:        fi.Error.Error(),
			})
		} else {
			err = s.Send(&proto.FileInfo{
				Name:         fi.FullPath,
				RelativeName: fi.RelPath,
				Size:         fi.FileInfo.Size(),
				Mode:         uint32(fi.FileInfo.Mode()),
				Modified:     fi.FileInfo.ModTime().Unix(),
				IsDir:        fi.FileInfo.IsDir(),
				Link:         fi.Link,
			})
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// DF implements the proto.OSDServer interface.
func (r *Registrator) DF(ctx context.Context, in *empty.Empty) (reply *proto.DFReply, err error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	// nolint: errcheck
	defer file.Close()

	var (
		stat     unix.Statfs_t
		multiErr *multierror.Error
	)

	stats := []*proto.DFStat{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())

		if len(fields) < 2 {
			continue
		}

		filesystem := fields[0]
		mountpoint := fields[1]

		f, err := os.Stat(mountpoint)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
			continue
		}

		if mode := f.Mode(); !mode.IsDir() {
			continue
		}

		if err := unix.Statfs(mountpoint, &stat); err != nil {
			multiErr = multierror.Append(multiErr, err)
			continue
		}

		totalSize := uint64(stat.Bsize) * stat.Blocks
		totalAvail := uint64(stat.Bsize) * stat.Bavail

		stat := &proto.DFStat{
			Filesystem: filesystem,
			Size:       totalSize,
			Available:  totalAvail,
			MountedOn:  mountpoint,
		}

		stats = append(stats, stat)
	}

	if err := scanner.Err(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	reply = &proto.DFReply{
		Stats: stats,
	}

	return reply, multiErr.ErrorOrNil()
}
