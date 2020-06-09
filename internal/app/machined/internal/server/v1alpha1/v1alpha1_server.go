// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/api/common"
	"github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/syslinux"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/pkg/containers"
	taloscontainerd "github.com/talos-systems/talos/internal/pkg/containers/containerd"
	"github.com/talos-systems/talos/internal/pkg/containers/cri"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/internal/pkg/kubeconfig"
	"github.com/talos-systems/talos/internal/pkg/tail"
	"github.com/talos-systems/talos/pkg/archiver"
	"github.com/talos-systems/talos/pkg/chunker"
	filechunker "github.com/talos-systems/talos/pkg/chunker/file"
	"github.com/talos-systems/talos/pkg/chunker/stream"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/version"
)

// OSPathSeparator is the string version of the os.PathSeparator
const OSPathSeparator = string(os.PathSeparator)

// Server implements the gRPC service server.
type Server struct {
	Controller runtime.Controller

	server *grpc.Server
}

// Register implements the factory.Registrator interface.
func (s *Server) Register(obj *grpc.Server) {
	s.server = obj

	machine.RegisterMachineServiceServer(obj, s)
}

// Reboot implements the machine.MachineServer interface.
//
// nolint: dupl
func (s *Server) Reboot(ctx context.Context, in *empty.Empty) (reply *machine.RebootResponse, err error) {
	log.Printf("reboot via API received")

	go func() {
		if err := s.Controller.Run(runtime.SequenceReboot, in); err != nil {
			log.Println("reboot failed:", err)

			if err != runtime.ErrLocked {
				// NB: We stop the gRPC server since a failed sequence triggers a
				// reboot.
				s.server.GracefulStop()
			}
		}
	}()

	reply = &machine.RebootResponse{
		Messages: []*machine.Reboot{
			{},
		},
	}

	return reply, nil
}

// Rollback implements the machine.MachineServer interface.
//
// nolint: dupl
func (s *Server) Rollback(ctx context.Context, in *machine.RollbackRequest) (reply *machine.RollbackResponse, err error) {
	log.Printf("rollback via API received")

	if err := syslinux.Revert(); err != nil {
		return nil, fmt.Errorf("failed to revert bootloader: %v", err)
	}

	go func() {
		if err := s.Controller.Run(runtime.SequenceReboot, in, runtime.WithForce()); err != nil {
			log.Println("reboot failed:", err)

			if err != runtime.ErrLocked {
				// NB: We stop the gRPC server since a failed sequence triggers a
				// reboot.
				s.server.GracefulStop()
			}
		}
	}()

	reply = &machine.RollbackResponse{
		Messages: []*machine.Rollback{
			{},
		},
	}

	return reply, nil
}

// Bootstrap implements the machine.MachineServer interface.
//
// nolint: dupl
func (s *Server) Bootstrap(ctx context.Context, in *machine.BootstrapRequest) (reply *machine.BootstrapResponse, err error) {
	log.Printf("bootstrap request received")

	if s.Controller.Runtime().Config().Machine().Type() == runtime.MachineTypeJoin {
		return nil, fmt.Errorf("bootstrap can only be performed on a control plane node")
	}

	go func() {
		if err := s.Controller.Run(runtime.SequenceBootstrap, in); err != nil {
			log.Println("bootstrap failed:", err)

			if err != runtime.ErrLocked {
				// NB: We stop the gRPC server since a failed sequence triggers a
				// reboot.
				s.server.GracefulStop()
			}
		}
	}()

	reply = &machine.BootstrapResponse{
		Messages: []*machine.Bootstrap{
			{},
		},
	}

	return reply, nil
}

// Shutdown implements the machine.MachineServer interface.
//
// nolint: dupl
func (s *Server) Shutdown(ctx context.Context, in *empty.Empty) (reply *machine.ShutdownResponse, err error) {
	log.Printf("shutdown via API received")

	go func() {
		if err := s.Controller.Run(runtime.SequenceShutdown, in); err != nil {
			log.Println("shutdown failed:", err)

			if err != runtime.ErrLocked {
				// NB: We stop the gRPC server since a failed sequence triggers a
				// reboot.
				s.server.GracefulStop()
			}
		}
	}()

	reply = &machine.ShutdownResponse{
		Messages: []*machine.Shutdown{
			{},
		},
	}

	return reply, nil
}

// Upgrade initiates an upgrade.
//
// nolint: dupl
func (s *Server) Upgrade(ctx context.Context, in *machine.UpgradeRequest) (reply *machine.UpgradeResponse, err error) {
	log.Printf("upgrade request received")

	log.Printf("validating %q", in.GetImage())

	if err = pullAndValidateInstallerImage(ctx, s.Controller.Runtime().Config().Machine().Registries(), in.GetImage()); err != nil {
		return nil, err
	}

	if err = etcd.ValidateForUpgrade(in.GetPreserve()); err != nil {
		return nil, err
	}

	go func() {
		if err := s.Controller.Run(runtime.SequenceUpgrade, in); err != nil {
			log.Println("upgrade failed:", err)

			if err != runtime.ErrLocked {
				// NB: We stop the gRPC server since a failed sequence triggers a
				// reboot.
				s.server.GracefulStop()
			}
		}
	}()

	reply = &machine.UpgradeResponse{
		Messages: []*machine.Upgrade{
			{
				Ack: "Upgrade request received",
			},
		},
	}

	return reply, nil
}

// Reset resets the node.
//
// nolint: dupl
func (s *Server) Reset(ctx context.Context, in *machine.ResetRequest) (reply *machine.ResetResponse, err error) {
	log.Printf("reset request received")

	go func() {
		if err := s.Controller.Run(runtime.SequenceReset, in); err != nil {
			log.Println("reset failed:", err)

			if err != runtime.ErrLocked {
				// NB: We stop the gRPC server since a failed sequence triggers a
				// reboot.
				s.server.GracefulStop()
			}
		}
	}()

	reply = &machine.ResetResponse{
		Messages: []*machine.Reset{
			{},
		},
	}

	return reply, nil
}

// Recover recovers the control plane.
//
// nolint: dupl
func (s *Server) Recover(ctx context.Context, in *machine.RecoverRequest) (reply *machine.RecoverResponse, err error) {
	log.Printf("recover request received")

	if s.Controller.Runtime().Config().Machine().Type() == runtime.MachineTypeJoin {
		return nil, fmt.Errorf("recover can only be performed on a control plane node")
	}

	go func() {
		if err := s.Controller.Run(runtime.SequenceRecover, in); err != nil {
			log.Println("recover failed:", err)

			if err != runtime.ErrLocked {
				// NB: We stop the gRPC server since a failed sequence triggers a
				// reboot.
				s.server.GracefulStop()
			}
		}
	}()

	reply = &machine.RecoverResponse{
		Messages: []*machine.Recover{
			{},
		},
	}

	return reply, nil
}

// ServiceList returns list of the registered services and their status
func (s *Server) ServiceList(ctx context.Context, in *empty.Empty) (result *machine.ServiceListResponse, err error) {
	services := system.Services(s.Controller.Runtime()).List()

	result = &machine.ServiceListResponse{
		Messages: []*machine.ServiceList{
			{
				Services: make([]*machine.ServiceInfo, len(services)),
			},
		},
	}

	for i := range services {
		result.Messages[0].Services[i] = services[i].AsProto()
	}

	return result, nil
}

// ServiceStart implements the machine.MachineServer interface and starts a
// service running on Talos.
func (s *Server) ServiceStart(ctx context.Context, in *machine.ServiceStartRequest) (reply *machine.ServiceStartResponse, err error) {
	if err = system.Services(s.Controller.Runtime()).APIStart(ctx, in.Id); err != nil {
		return &machine.ServiceStartResponse{}, err
	}

	reply = &machine.ServiceStartResponse{
		Messages: []*machine.ServiceStart{
			{
				Resp: fmt.Sprintf("Service %q started", in.Id),
			},
		},
	}

	return reply, err
}

// ServiceStop implements the machine.MachineServer interface and stops a
// service running on Talos.
func (s *Server) ServiceStop(ctx context.Context, in *machine.ServiceStopRequest) (reply *machine.ServiceStopResponse, err error) {
	if err = system.Services(s.Controller.Runtime()).APIStop(ctx, in.Id); err != nil {
		return &machine.ServiceStopResponse{}, err
	}

	reply = &machine.ServiceStopResponse{
		Messages: []*machine.ServiceStop{
			{
				Resp: fmt.Sprintf("Service %q stopped", in.Id),
			},
		},
	}

	return reply, err
}

// ServiceRestart implements the machine.MachineServer interface and stops a
// service running on Talos.
func (s *Server) ServiceRestart(ctx context.Context, in *machine.ServiceRestartRequest) (reply *machine.ServiceRestartResponse, err error) {
	if err = system.Services(s.Controller.Runtime()).APIRestart(ctx, in.Id); err != nil {
		return &machine.ServiceRestartResponse{}, err
	}

	reply = &machine.ServiceRestartResponse{
		Messages: []*machine.ServiceRestart{
			{
				Resp: fmt.Sprintf("Service %q restarted", in.Id),
			},
		},
	}

	return reply, err
}

// Copy implements the machine.MachineServer interface and copies data out of Talos node
func (s *Server) Copy(req *machine.CopyRequest, obj machine.MachineService_CopyServer) error {
	path := req.RootPath
	path = filepath.Clean(path)

	if !filepath.IsAbs(path) {
		return fmt.Errorf("path is not absolute %v", path)
	}

	pr, pw := io.Pipe()

	errCh := make(chan error, 1)

	ctx, ctxCancel := context.WithCancel(obj.Context())
	defer ctxCancel()

	go func() {
		// nolint: errcheck
		defer pw.Close()
		errCh <- archiver.TarGz(ctx, path, pw)
	}()

	chunker := stream.NewChunker(pr)
	chunkCh := chunker.Read(ctx)

	for data := range chunkCh {
		err := obj.SendMsg(&common.Data{Bytes: data})
		if err != nil {
			ctxCancel()
		}
	}

	archiveErr := <-errCh
	if archiveErr != nil {
		return obj.SendMsg(&common.Data{
			Metadata: &common.Metadata{
				Error: archiveErr.Error(),
			},
		})
	}

	return nil
}

// List implements the machine.MachineServer interface.
func (s *Server) List(req *machine.ListRequest, obj machine.MachineService_ListServer) error {
	if req == nil {
		req = new(machine.ListRequest)
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

	files, err := archiver.Walker(obj.Context(), req.Root, archiver.WithMaxRecurseDepth(maxDepth))
	if err != nil {
		return err
	}

	for fi := range files {
		if fi.Error != nil {
			err = obj.Send(&machine.FileInfo{
				Name:         fi.FullPath,
				RelativeName: fi.RelPath,
				Error:        fi.Error.Error(),
			})
		} else {
			err = obj.Send(&machine.FileInfo{
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

// Mounts implements the machine.OSDServer interface.
func (s *Server) Mounts(ctx context.Context, in *empty.Empty) (reply *machine.MountsResponse, err error) {
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

	stats := []*machine.MountStat{}
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

		stat := &machine.MountStat{
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

	reply = &machine.MountsResponse{
		Messages: []*machine.Mounts{
			{
				Stats: stats,
			},
		},
	}

	return reply, multiErr.ErrorOrNil()
}

// Version implements the machine.MachineServer interface.
func (s *Server) Version(ctx context.Context, in *empty.Empty) (reply *machine.VersionResponse, err error) {
	var platform *machine.PlatformInfo

	if s.Controller.Runtime().State().Platform() != nil {
		platform = &machine.PlatformInfo{
			Name: s.Controller.Runtime().State().Platform().Name(),
			Mode: s.Controller.Runtime().State().Platform().Mode().String(),
		}
	}

	return &machine.VersionResponse{
		Messages: []*machine.Version{
			{
				Version:  version.NewVersion(),
				Platform: platform,
			},
		},
	}, nil
}

// Kubeconfig implements the osapi.OSDServer interface.
func (s *Server) Kubeconfig(empty *empty.Empty, obj machine.MachineService_KubeconfigServer) error {
	var b bytes.Buffer

	if err := kubeconfig.GenerateAdmin(s.Controller.Runtime().Config().Cluster(), &b); err != nil {
		return err
	}

	// wrap in .tar.gz to match Copy protocol
	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)

	tarW := tar.NewWriter(zw)

	err := tarW.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "kubeconfig",
		Size:     int64(b.Len()),
		ModTime:  time.Now(),
		Mode:     0600,
	})
	if err != nil {
		return err
	}

	_, err = io.Copy(tarW, &b)
	if err != nil {
		return err
	}

	if err = zw.Close(); err != nil {
		return err
	}

	return obj.Send(&common.Data{
		Bytes: buf.Bytes(),
	})
}

// Logs provides a service or container logs can be requested and the contents of the
// log file are streamed in chunks.
// nolint: gocyclo
func (s *Server) Logs(req *machine.LogsRequest, l machine.MachineService_LogsServer) (err error) {
	var chunk chunker.Chunker

	switch {
	case req.Namespace == constants.SystemContainerdNamespace || req.Id == "kubelet":
		filename := filepath.Join(constants.DefaultLogPath, filepath.Base(req.Id)+".log")

		var file *os.File

		file, err = os.OpenFile(filename, os.O_RDONLY, 0)
		if err != nil {
			return
		}
		// nolint: errcheck
		defer file.Close()

		if req.TailLines >= 0 {
			err = tail.SeekLines(file, int(req.TailLines))
			if err != nil {
				return fmt.Errorf("error tailing log: %w", err)
			}
		}

		options := []filechunker.Option{}
		if req.Follow {
			options = append(options, filechunker.WithFollow())
		}

		chunk = filechunker.NewChunker(file, options...)
	default:
		var file io.Closer

		if chunk, file, err = k8slogs(l.Context(), req); err != nil {
			return err
		}
		// nolint: errcheck
		defer file.Close()
	}

	for data := range chunk.Read(l.Context()) {
		if err = l.Send(&common.Data{Bytes: data}); err != nil {
			return
		}
	}

	return nil
}

func k8slogs(ctx context.Context, req *machine.LogsRequest) (chunker.Chunker, io.Closer, error) {
	inspector, err := getContainerInspector(ctx, req.Namespace, req.Driver)
	if err != nil {
		return nil, nil, err
	}
	// nolint: errcheck
	defer inspector.Close()

	container, err := inspector.Container(req.Id)
	if err != nil {
		return nil, nil, err
	}

	if container == nil {
		return nil, nil, fmt.Errorf("container %q not found", req.Id)
	}

	return container.GetLogChunker(req.Follow, int(req.TailLines))
}

func getContainerInspector(ctx context.Context, namespace string, driver common.ContainerDriver) (containers.Inspector, error) {
	switch driver {
	case common.ContainerDriver_CRI:
		if namespace != criconstants.K8sContainerdNamespace {
			return nil, errors.New("CRI inspector is supported only for K8s namespace")
		}

		return cri.NewInspector(ctx)
	case common.ContainerDriver_CONTAINERD:
		addr := constants.ContainerdAddress
		if namespace == constants.SystemContainerdNamespace {
			addr = constants.SystemContainerdAddress
		}

		return taloscontainerd.NewInspector(ctx, namespace, taloscontainerd.WithContainerdAddress(addr))
	default:
		return nil, fmt.Errorf("unsupported driver %q", driver)
	}
}

// Read implements the read API.
func (s *Server) Read(in *machine.ReadRequest, srv machine.MachineService_ReadServer) (err error) {
	stat, err := os.Stat(in.Path)
	if err != nil {
		return err
	}

	switch mode := stat.Mode(); {
	case mode.IsRegular():
		f, err := os.OpenFile(in.Path, os.O_RDONLY, 0)
		if err != nil {
			return err
		}

		defer f.Close() //nolint: errcheck

		ctx, cancel := context.WithCancel(srv.Context())
		defer cancel()

		chunker := stream.NewChunker(f)
		chunkCh := chunker.Read(ctx)

		for data := range chunkCh {
			err := srv.SendMsg(&common.Data{Bytes: data})
			if err != nil {
				cancel()
			}
		}

		return nil
	default:
		return fmt.Errorf("path must be a regular file")
	}
}

// Events streams runtime events.
func (s *Server) Events(req *machine.EventsRequest, l machine.MachineService_EventsServer) error {
	errCh := make(chan error)

	s.Controller.Runtime().Events().Watch(func(events <-chan runtime.Event) {
		errCh <- func() error {
			for {
				select {
				case <-l.Context().Done():
					return l.Context().Err()
				case event, ok := <-events:
					if !ok {
						return nil
					}

					msg, err := event.ToMachineEvent()
					if err != nil {
						return err
					}

					if err = l.Send(msg); err != nil {
						return err
					}
				}
			}
		}()
	})

	return <-errCh
}

func pullAndValidateInstallerImage(ctx context.Context, reg runtime.Registries, ref string) error {
	// Pull down specified installer image early so we can bail if it doesn't exist in the upstream registry
	containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	client, err := containerd.New(constants.SystemContainerdAddress)
	if err != nil {
		return err
	}

	img, err := image.Pull(containerdctx, reg, client, ref)
	if err != nil {
		return err
	}

	// Launch the container with a known help command for a simple check to make sure the image is valid
	args := []string{
		"/bin/installer",
		"--help",
	}

	specOpts := []oci.SpecOpts{
		oci.WithImageConfig(img),
		oci.WithProcessArgs(args...),
	}

	containerOpts := []containerd.NewContainerOpts{
		containerd.WithImage(img),
		containerd.WithNewSnapshot("validate", img),
		containerd.WithNewSpec(specOpts...),
	}

	container, err := client.NewContainer(containerdctx, "validate", containerOpts...)
	if err != nil {
		return err
	}

	//nolint: errcheck
	defer container.Delete(containerdctx, containerd.WithSnapshotCleanup)

	task, err := container.NewTask(containerdctx, cio.NullIO)
	if err != nil {
		return err
	}

	//nolint: errcheck
	defer task.Delete(containerdctx)

	exitStatusC, err := task.Wait(containerdctx)
	if err != nil {
		return err
	}

	if err = task.Start(containerdctx); err != nil {
		return err
	}

	status := <-exitStatusC

	code, _, err := status.Result()
	if err != nil {
		return err
	}

	if code != 0 {
		return fmt.Errorf("installer help returned non-zero exit. assuming invalid installer")
	}

	return nil
}
