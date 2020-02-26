// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package reg

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
	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/pkg/containers"
	taloscontainerd "github.com/talos-systems/talos/internal/pkg/containers/containerd"
	"github.com/talos-systems/talos/internal/pkg/containers/cri"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/internal/pkg/event"
	"github.com/talos-systems/talos/internal/pkg/kubeconfig"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/runtime/platform"
	"github.com/talos-systems/talos/internal/pkg/tail"
	"github.com/talos-systems/talos/pkg/archiver"
	"github.com/talos-systems/talos/pkg/chunker"
	filechunker "github.com/talos-systems/talos/pkg/chunker/file"
	"github.com/talos-systems/talos/pkg/chunker/stream"
	machinecfg "github.com/talos-systems/talos/pkg/config/machine"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/version"
)

// OSPathSeparator is the string version of the os.PathSeparator
const OSPathSeparator = string(os.PathSeparator)

// Registrator is the concrete type that implements the factory.Registrator and
// machineapi.Machine interfaces.
type Registrator struct {
	config   runtime.Configurator
	platform runtime.Platform
}

// NewRegistrator builds new Registrator instance
func NewRegistrator(config runtime.Configurator) *Registrator {
	platform, err := platform.CurrentPlatform()
	if err != nil {
		// should never happen
		log.Printf("failed discovering platform: %v", err)
	}

	return &Registrator{
		config:   config,
		platform: platform,
	}
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	machineapi.RegisterMachineServiceServer(s, r)
}

// Reboot implements the machineapi.MachineServer interface.
func (r *Registrator) Reboot(ctx context.Context, in *empty.Empty) (reply *machineapi.RebootResponse, err error) {
	reply = &machineapi.RebootResponse{
		Messages: []*machineapi.Reboot{
			{},
		},
	}

	log.Printf("reboot via API received")
	event.Bus().Notify(event.Event{Type: event.Reboot})

	return
}

// Shutdown implements the machineapi.MachineServer interface.
func (r *Registrator) Shutdown(ctx context.Context, in *empty.Empty) (reply *machineapi.ShutdownResponse, err error) {
	reply = &machineapi.ShutdownResponse{
		Messages: []*machineapi.Shutdown{
			{},
		},
	}

	log.Printf("shutdown via API received")
	event.Bus().Notify(event.Event{Type: event.Shutdown})

	return
}

// Upgrade initiates an upgrade.
func (r *Registrator) Upgrade(ctx context.Context, in *machineapi.UpgradeRequest) (data *machineapi.UpgradeResponse, err error) {
	if err = pullAndValidateInstallerImage(ctx, r.config.Machine().Registries(), in.GetImage()); err != nil {
		return nil, err
	}

	if err = etcd.ValidateForUpgrade(); err != nil {
		return nil, err
	}

	event.Bus().Notify(event.Event{Type: event.Upgrade, Data: in})

	data = &machineapi.UpgradeResponse{
		Messages: []*machineapi.Upgrade{
			{
				Ack: "Upgrade request received",
			},
		},
	}

	return data, err
}

// Reset resets the node.
func (r *Registrator) Reset(ctx context.Context, in *machineapi.ResetRequest) (data *machineapi.ResetResponse, err error) {
	event.Bus().Notify(event.Event{Type: event.Reset, Data: in})

	return &machineapi.ResetResponse{
		Messages: []*machineapi.Reset{
			{},
		},
	}, err
}

// ServiceList returns list of the registered services and their status
func (r *Registrator) ServiceList(ctx context.Context, in *empty.Empty) (result *machineapi.ServiceListResponse, err error) {
	services := system.Services(r.config).List()

	result = &machineapi.ServiceListResponse{
		Messages: []*machineapi.ServiceList{
			{
				Services: make([]*machineapi.ServiceInfo, len(services)),
			},
		},
	}

	for i := range services {
		result.Messages[0].Services[i] = services[i].AsProto()
	}

	return result, nil
}

// ServiceStart implements the machineapi.MachineServer interface and starts a
// service running on Talos.
func (r *Registrator) ServiceStart(ctx context.Context, in *machineapi.ServiceStartRequest) (reply *machineapi.ServiceStartResponse, err error) {
	if err = system.Services(r.config).APIStart(ctx, in.Id); err != nil {
		return &machineapi.ServiceStartResponse{}, err
	}

	reply = &machineapi.ServiceStartResponse{
		Messages: []*machineapi.ServiceStart{
			{
				Resp: fmt.Sprintf("Service %q started", in.Id),
			},
		},
	}

	return reply, err
}

// Start implements deprecated Start method which forwards to 'ServiceStart'.
//nolint: staticcheck
func (r *Registrator) Start(ctx context.Context, in *machineapi.StartRequest) (reply *machineapi.StartResponse, err error) {
	var rep *machineapi.ServiceStartResponse

	rep, err = r.ServiceStart(ctx, &machineapi.ServiceStartRequest{Id: in.Id})
	if rep != nil {
		reply = &machineapi.StartResponse{
			Resp: rep.Messages[0].Resp,
		}
	}

	return
}

// Stop implements deprecated Stop method which forwards to 'ServiceStop'.
//nolint: staticcheck
func (r *Registrator) Stop(ctx context.Context, in *machineapi.StopRequest) (reply *machineapi.StopResponse, err error) {
	var rep *machineapi.ServiceStopResponse

	rep, err = r.ServiceStop(ctx, &machineapi.ServiceStopRequest{Id: in.Id})
	if rep != nil {
		reply = &machineapi.StopResponse{
			Resp: rep.Messages[0].Resp,
		}
	}

	return
}

// ServiceStop implements the machineapi.MachineServer interface and stops a
// service running on Talos.
func (r *Registrator) ServiceStop(ctx context.Context, in *machineapi.ServiceStopRequest) (reply *machineapi.ServiceStopResponse, err error) {
	if err = system.Services(r.config).APIStop(ctx, in.Id); err != nil {
		return &machineapi.ServiceStopResponse{}, err
	}

	reply = &machineapi.ServiceStopResponse{
		Messages: []*machineapi.ServiceStop{
			{
				Resp: fmt.Sprintf("Service %q stopped", in.Id),
			},
		},
	}

	return reply, err
}

// ServiceRestart implements the machineapi.MachineServer interface and stops a
// service running on Talos.
func (r *Registrator) ServiceRestart(ctx context.Context, in *machineapi.ServiceRestartRequest) (reply *machineapi.ServiceRestartResponse, err error) {
	if err = system.Services(r.config).APIRestart(ctx, in.Id); err != nil {
		return &machineapi.ServiceRestartResponse{}, err
	}

	reply = &machineapi.ServiceRestartResponse{
		Messages: []*machineapi.ServiceRestart{
			{
				Resp: fmt.Sprintf("Service %q restarted", in.Id),
			},
		},
	}

	return reply, err
}

// Copy implements the machineapi.MachineServer interface and copies data out of Talos node
func (r *Registrator) Copy(req *machineapi.CopyRequest, s machineapi.MachineService_CopyServer) error {
	path := req.RootPath
	path = filepath.Clean(path)

	if !filepath.IsAbs(path) {
		return fmt.Errorf("path is not absolute %v", path)
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
		err := s.SendMsg(&common.Data{Bytes: data})
		if err != nil {
			ctxCancel()
		}
	}

	archiveErr := <-errCh
	if archiveErr != nil {
		return s.SendMsg(&common.Data{
			Metadata: &common.Metadata{
				Error: archiveErr.Error(),
			},
		})
	}

	return nil
}

// List implements the machineapi.MachineServer interface.
func (r *Registrator) List(req *machineapi.ListRequest, s machineapi.MachineService_ListServer) error {
	if req == nil {
		req = new(machineapi.ListRequest)
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
			err = s.Send(&machineapi.FileInfo{
				Name:         fi.FullPath,
				RelativeName: fi.RelPath,
				Error:        fi.Error.Error(),
			})
		} else {
			err = s.Send(&machineapi.FileInfo{
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

// Mounts implements the machineapi.OSDServer interface.
func (r *Registrator) Mounts(ctx context.Context, in *empty.Empty) (reply *machineapi.MountsResponse, err error) {
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

	stats := []*machineapi.MountStat{}
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

		stat := &machineapi.MountStat{
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

	reply = &machineapi.MountsResponse{
		Messages: []*machineapi.Mounts{
			{
				Stats: stats,
			},
		},
	}

	return reply, multiErr.ErrorOrNil()
}

// Version implements the machineapi.MachineServer interface.
func (r *Registrator) Version(ctx context.Context, in *empty.Empty) (reply *machineapi.VersionResponse, err error) {
	var platform *machineapi.PlatformInfo

	if r.platform != nil {
		platform = &machineapi.PlatformInfo{
			Name: r.platform.Name(),
			Mode: r.platform.Mode().String(),
		}
	}

	return &machineapi.VersionResponse{
		Messages: []*machineapi.Version{
			{
				Version:  version.NewVersion(),
				Platform: platform,
			},
		},
	}, nil
}

// Kubeconfig implements the osapi.OSDServer interface.
func (r *Registrator) Kubeconfig(empty *empty.Empty, s machineapi.MachineService_KubeconfigServer) error {
	var kubeconfigBuf bytes.Buffer

	if err := kubeconfig.GenerateAdmin(r.config.Cluster(), &kubeconfigBuf); err != nil {
		return err
	}

	// wrap in .tar.gz to match Copy protocol
	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)

	tarW := tar.NewWriter(zw)

	err := tarW.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "kubeconfig",
		Size:     int64(kubeconfigBuf.Len()),
		ModTime:  time.Now(),
		Mode:     0600,
	})
	if err != nil {
		return err
	}

	_, err = io.Copy(tarW, &kubeconfigBuf)
	if err != nil {
		return err
	}

	if err = zw.Close(); err != nil {
		return err
	}

	return s.Send(&common.Data{
		Bytes: buf.Bytes(),
	})
}

// Logs provides a service or container logs can be requested and the contents of the
// log file are streamed in chunks.
// nolint: gocyclo
func (r *Registrator) Logs(req *machineapi.LogsRequest, l machineapi.MachineService_LogsServer) (err error) {
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

func k8slogs(ctx context.Context, req *machineapi.LogsRequest) (chunker.Chunker, io.Closer, error) {
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
func (r *Registrator) Read(in *machineapi.ReadRequest, srv machineapi.MachineService_ReadServer) (err error) {
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

		ctx, cancel := context.WithCancel(context.Background())

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

func pullAndValidateInstallerImage(ctx context.Context, config machinecfg.Registries, ref string) error {
	// Pull down specified installer image early so we can bail if it doesn't exist in the upstream registry
	containerdctx := namespaces.WithNamespace(ctx, constants.SystemContainerdNamespace)

	client, err := containerd.New(constants.SystemContainerdAddress)
	if err != nil {
		return err
	}

	img, err := image.Pull(containerdctx, config, client, ref)
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

	task, err := container.NewTask(containerdctx, cio.NewCreator(cio.WithStdio))
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
