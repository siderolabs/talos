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
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/procfs"
	"github.com/rs/xid"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/pkg/containers"
	taloscontainerd "github.com/talos-systems/talos/internal/pkg/containers/containerd"
	"github.com/talos-systems/talos/internal/pkg/containers/cri"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/internal/pkg/kmsg"
	"github.com/talos-systems/talos/internal/pkg/kubeconfig"
	"github.com/talos-systems/talos/pkg/archiver"
	"github.com/talos-systems/talos/pkg/chunker"
	"github.com/talos-systems/talos/pkg/chunker/stream"
	"github.com/talos-systems/talos/pkg/machinery/api/cluster"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	osapi "github.com/talos-systems/talos/pkg/machinery/api/os"
	"github.com/talos-systems/talos/pkg/machinery/config"
	machinetype "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/version"
)

// OSPathSeparator is the string version of the os.PathSeparator.
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
	osapi.RegisterOSServiceServer(obj, &osdServer{Server: s}) //nolint: staticcheck
	cluster.RegisterClusterServiceServer(obj, s)
}

// ApplyConfiguration implements machine.MachineServer.
func (s *Server) ApplyConfiguration(ctx context.Context, in *machine.ApplyConfigurationRequest) (reply *machine.ApplyConfigurationResponse, err error) {
	if err = s.Controller.Runtime().SetConfig(in.GetData()); err != nil {
		return nil, err
	}

	go func() {
		if err = s.Controller.Run(runtime.SequenceApplyConfiguration, in); err != nil {
			log.Println("apply configuration failed:", err)

			if err != runtime.ErrLocked {
				s.server.GracefulStop()
			}
		}
	}()

	reply = &machine.ApplyConfigurationResponse{
		Messages: []*machine.ApplyConfiguration{
			{},
		},
	}

	return reply, nil
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

	grub := &grub.Grub{
		BootDisk: s.Controller.Runtime().Config().Machine().Install().Disk(),
	}

	_, next, err := grub.Labels()
	if err != nil {
		return nil, err
	}

	if _, err = os.Stat(filepath.Join(constants.BootMountPoint, next)); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("cannot rollback to %q, label does not exist", next)
	}

	if err := grub.Default(next); err != nil {
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

	if s.Controller.Runtime().Config().Machine().Type() == machinetype.TypeJoin {
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
		return nil, fmt.Errorf("error validating installer image %q: %w", in.GetImage(), err)
	}

	if err = etcd.ValidateForUpgrade(s.Controller.Runtime().Config(), in.GetPreserve()); err != nil {
		return nil, fmt.Errorf("error validating etcd for upgrade: %w", err)
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

	if s.Controller.Runtime().Config().Machine().Type() == machinetype.TypeJoin {
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

// ServiceList returns list of the registered services and their status.
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

// Copy implements the machine.MachineServer interface and copies data out of Talos node.
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

	chunker := stream.NewChunker(ctx, pr)
	chunkCh := chunker.Read()

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

// Mounts implements the machine.MachineServer interface.
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

// Kubeconfig implements the machine.MachineServer interface.
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
		Mode:     0o600,
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
		var options []runtime.LogOption

		if req.Follow {
			options = append(options, runtime.WithFollow())
		}

		if req.TailLines >= 0 {
			options = append(options, runtime.WithTailLines(int(req.TailLines)))
		}

		var logR io.ReadCloser

		logR, err = s.Controller.Runtime().Logging().ServiceLog(req.Id).Reader(options...)
		if err != nil {
			return
		}

		// nolint: errcheck
		defer logR.Close()

		chunk = stream.NewChunker(l.Context(), logR)
	default:
		var file io.Closer

		if chunk, file, err = k8slogs(l.Context(), req); err != nil {
			return err
		}
		// nolint: errcheck
		defer file.Close()
	}

	for data := range chunk.Read() {
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

	return container.GetLogChunker(ctx, req.Follow, int(req.TailLines))
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

		chunker := stream.NewChunker(ctx, f)
		chunkCh := chunker.Read()

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
//
//nolint: gocyclo
func (s *Server) Events(req *machine.EventsRequest, l machine.MachineService_EventsServer) error {
	errCh := make(chan error)

	var opts []runtime.WatchOptionFunc

	if req.TailEvents != 0 {
		opts = append(opts, runtime.WithTailEvents(int(req.TailEvents)))
	}

	if req.TailId != "" {
		tailID, err := xid.FromString(req.TailId)
		if err != nil {
			return fmt.Errorf("error parsing tail_id: %w", err)
		}

		opts = append(opts, runtime.WithTailID(tailID))
	}

	if req.TailSeconds != 0 {
		opts = append(opts, runtime.WithTailDuration(time.Duration(req.TailSeconds)*time.Second))
	}

	if err := s.Controller.Runtime().Events().Watch(func(events <-chan runtime.Event) {
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
	}, opts...); err != nil {
		return err
	}

	return <-errCh
}

func pullAndValidateInstallerImage(ctx context.Context, reg config.Registries, ref string) error {
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

// Containers implements the machine.MachineServer interface.
func (s *Server) Containers(ctx context.Context, in *machine.ContainersRequest) (reply *machine.ContainersResponse, err error) {
	inspector, err := getContainerInspector(ctx, in.Namespace, in.Driver)
	if err != nil {
		return nil, err
	}
	// nolint: errcheck
	defer inspector.Close()

	pods, err := inspector.Pods()
	if err != nil {
		// fatal error
		if pods == nil {
			return nil, err
		}
		// TODO: only some failed, need to handle it better via client
		log.Println(err.Error())
	}

	containers := []*machine.ContainerInfo{}

	for _, pod := range pods {
		for _, container := range pod.Containers {
			container := &machine.ContainerInfo{
				Namespace: in.Namespace,
				Id:        container.Display,
				PodId:     pod.Name,
				Name:      container.Name,
				Image:     container.Image,
				Pid:       container.Pid,
				Status:    container.Status,
			}
			containers = append(containers, container)
		}
	}

	reply = &machine.ContainersResponse{
		Messages: []*machine.Container{
			{
				Containers: containers,
			},
		},
	}

	return reply, nil
}

// Stats implements the machine.MachineServer interface.
// nolint: gocyclo
func (s *Server) Stats(ctx context.Context, in *machine.StatsRequest) (reply *machine.StatsResponse, err error) {
	inspector, err := getContainerInspector(ctx, in.Namespace, in.Driver)
	if err != nil {
		return nil, err
	}
	// nolint: errcheck
	defer inspector.Close()

	pods, err := inspector.Pods()
	if err != nil {
		// fatal error
		if pods == nil {
			return nil, err
		}
		// TODO: only some failed, need to handle it better via client
		log.Println(err.Error())
	}

	stats := []*machine.Stat{}

	for _, pod := range pods {
		for _, container := range pod.Containers {
			if container.Metrics == nil {
				continue
			}

			stat := &machine.Stat{
				Namespace:   in.Namespace,
				Id:          container.Display,
				PodId:       pod.Name,
				Name:        container.Name,
				MemoryUsage: container.Metrics.MemoryUsage,
				CpuUsage:    container.Metrics.CPUUsage,
			}

			stats = append(stats, stat)
		}
	}

	reply = &machine.StatsResponse{
		Messages: []*machine.Stats{
			{
				Stats: stats,
			},
		},
	}

	return reply, nil
}

// Restart implements the machine.MachineServer interface.
func (s *Server) Restart(ctx context.Context, in *machine.RestartRequest) (*machine.RestartResponse, error) {
	inspector, err := getContainerInspector(ctx, in.Namespace, in.Driver)
	if err != nil {
		return nil, err
	}
	// nolint: errcheck
	defer inspector.Close()

	container, err := inspector.Container(in.Id)
	if err != nil {
		return nil, err
	}

	if container == nil {
		return nil, fmt.Errorf("container %q not found", in.Id)
	}

	err = container.Kill(syscall.SIGTERM)
	if err != nil {
		return nil, err
	}

	return &machine.RestartResponse{
		Messages: []*machine.Restart{
			{},
		},
	}, nil
}

// Dmesg implements the machine.MachineServer interface.
//
//nolint: gocyclo
func (s *Server) Dmesg(req *machine.DmesgRequest, srv machine.MachineService_DmesgServer) error {
	ctx := srv.Context()

	var options []kmsg.Option

	if req.Follow {
		options = append(options, kmsg.Follow())
	}

	if req.Tail {
		options = append(options, kmsg.FromTail())
	}

	reader, err := kmsg.NewReader(options...)
	if err != nil {
		return fmt.Errorf("error opening /dev/kmsg reader: %w", err)
	}
	defer reader.Close() //nolint: errcheck

	ch := reader.Scan(ctx)

	for {
		select {
		case <-ctx.Done():
			if err = reader.Close(); err != nil {
				return err
			}
		case packet, ok := <-ch:
			if !ok {
				return nil
			}

			if packet.Err != nil {
				err = srv.Send(&common.Data{
					Metadata: &common.Metadata{
						Error: packet.Err.Error(),
					},
				})
			} else {
				msg := packet.Message
				err = srv.Send(&common.Data{
					Bytes: []byte(fmt.Sprintf("%s: %7s: [%s]: %s", msg.Facility, msg.Priority, msg.Timestamp.Format(time.RFC3339Nano), msg.Message)),
				})
			}

			if err != nil {
				return err
			}
		}
	}
}

// Processes implements the machine.MachineServer interface.
func (s *Server) Processes(ctx context.Context, in *empty.Empty) (reply *machine.ProcessesResponse, err error) {
	procs, err := procfs.AllProcs()
	if err != nil {
		return nil, err
	}

	processes := make([]*machine.ProcessInfo, 0, len(procs))

	var (
		command    string
		executable string
		args       []string
		stats      procfs.ProcStat
	)

	for _, proc := range procs {
		// due to race condition, reading process info might fail if process has already terminated
		command, err = proc.Comm()
		if err != nil {
			continue
		}

		executable, err = proc.Executable()
		if err != nil {
			continue
		}

		args, err = proc.CmdLine()
		if err != nil {
			continue
		}

		stats, err = proc.Stat()
		if err != nil {
			continue
		}

		p := &machine.ProcessInfo{
			Pid:            int32(proc.PID),
			Ppid:           int32(stats.PPID),
			State:          stats.State,
			Threads:        int32(stats.NumThreads),
			CpuTime:        stats.CPUTime(),
			VirtualMemory:  uint64(stats.VirtualMemory()),
			ResidentMemory: uint64(stats.ResidentMemory()),
			Command:        command,
			Executable:     executable,
			Args:           strings.Join(args, " "),
		}

		processes = append(processes, p)
	}

	reply = &machine.ProcessesResponse{
		Messages: []*machine.Process{
			{
				Processes: processes,
			},
		},
	}

	return reply, nil
}

// Memory implements the machine.MachineServer interface.
func (s *Server) Memory(ctx context.Context, in *empty.Empty) (reply *machine.MemoryResponse, err error) {
	proc, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	info, err := proc.Meminfo()
	if err != nil {
		return nil, err
	}

	meminfo := &machine.MemInfo{
		Memtotal:          info.MemTotal,
		Memfree:           info.MemFree,
		Memavailable:      info.MemAvailable,
		Buffers:           info.Buffers,
		Cached:            info.Cached,
		Swapcached:        info.SwapCached,
		Active:            info.Active,
		Inactive:          info.Inactive,
		Activeanon:        info.ActiveAnon,
		Inactiveanon:      info.InactiveAnon,
		Activefile:        info.ActiveFile,
		Inactivefile:      info.InactiveFile,
		Unevictable:       info.Unevictable,
		Mlocked:           info.Mlocked,
		Swaptotal:         info.SwapTotal,
		Swapfree:          info.SwapFree,
		Dirty:             info.Dirty,
		Writeback:         info.Writeback,
		Anonpages:         info.AnonPages,
		Mapped:            info.Mapped,
		Shmem:             info.Shmem,
		Slab:              info.Slab,
		Sreclaimable:      info.SReclaimable,
		Sunreclaim:        info.SUnreclaim,
		Kernelstack:       info.KernelStack,
		Pagetables:        info.PageTables,
		Nfsunstable:       info.NFSUnstable,
		Bounce:            info.Bounce,
		Writebacktmp:      info.WritebackTmp,
		Commitlimit:       info.CommitLimit,
		Committedas:       info.CommittedAS,
		Vmalloctotal:      info.VmallocTotal,
		Vmallocused:       info.VmallocUsed,
		Vmallocchunk:      info.VmallocChunk,
		Hardwarecorrupted: info.HardwareCorrupted,
		Anonhugepages:     info.AnonHugePages,
		Shmemhugepages:    info.ShmemHugePages,
		Shmempmdmapped:    info.ShmemPmdMapped,
		Cmatotal:          info.CmaTotal,
		Cmafree:           info.CmaFree,
		Hugepagestotal:    info.HugePagesTotal,
		Hugepagesfree:     info.HugePagesFree,
		Hugepagesrsvd:     info.HugePagesRsvd,
		Hugepagessurp:     info.HugePagesSurp,
		Hugepagesize:      info.Hugepagesize,
		Directmap4K:       info.DirectMap4k,
		Directmap2M:       info.DirectMap2M,
		Directmap1G:       info.DirectMap1G,
	}

	reply = &machine.MemoryResponse{
		Messages: []*machine.Memory{
			{
				Meminfo: meminfo,
			},
		},
	}

	return reply, err
}
