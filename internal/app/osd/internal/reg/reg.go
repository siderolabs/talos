/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	criconstants "github.com/containerd/cri/pkg/constants"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/prometheus/procfs"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"

	machineapi "github.com/talos-systems/talos/api/machine"
	networkapi "github.com/talos-systems/talos/api/network"
	osapi "github.com/talos-systems/talos/api/os"
	timeapi "github.com/talos-systems/talos/api/time"
	"github.com/talos-systems/talos/internal/pkg/containers"
	"github.com/talos-systems/talos/internal/pkg/containers/containerd"
	"github.com/talos-systems/talos/internal/pkg/containers/cri"
	"github.com/talos-systems/talos/pkg/chunker"
	filechunker "github.com/talos-systems/talos/pkg/chunker/file"
	"github.com/talos-systems/talos/pkg/constants"
)

// Registrator is the concrete type that implements the factory.Registrator and
// osapi.OSDServer interfaces.
type Registrator struct {
	// every Init service API is proxied via OSD
	*MachineClient
	*TimeClient
	*NetworkClient
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	osapi.RegisterOSServer(s, r)
	machineapi.RegisterMachineServer(s, r)
	timeapi.RegisterTimeServer(s, r)
	networkapi.RegisterNetworkServer(s, r)
}

// Kubeconfig implements the osapi.OSDServer interface.
func (r *Registrator) Kubeconfig(ctx context.Context, in *empty.Empty) (data *osapi.DataReply, err error) {
	fileBytes, err := ioutil.ReadFile(constants.AdminKubeconfig)
	if err != nil {
		return
	}

	data = &osapi.DataReply{
		Response: []*osapi.DataResponse{
			{
				Bytes: &osapi.Data{Bytes: fileBytes},
			},
		},
	}

	return data, err
}

// Containers implements the osapi.OSDServer interface.
func (r *Registrator) Containers(ctx context.Context, in *osapi.ContainersRequest) (reply *osapi.ContainersReply, err error) {
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

	containers := []*osapi.Container{}

	for _, pod := range pods {
		for _, container := range pod.Containers {
			container := &osapi.Container{
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

	reply = &osapi.ContainersReply{
		Response: []*osapi.ContainerResponse{
			{
				Containers: containers,
			},
		},
	}

	return reply, nil
}

// Stats implements the osapi.OSDServer interface.
// nolint: gocyclo
func (r *Registrator) Stats(ctx context.Context, in *osapi.StatsRequest) (reply *osapi.StatsReply, err error) {
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

	stats := []*osapi.Stat{}

	for _, pod := range pods {
		for _, container := range pod.Containers {
			if container.Metrics == nil {
				continue
			}

			stat := &osapi.Stat{
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

	reply = &osapi.StatsReply{
		Response: []*osapi.StatsResponse{
			{
				Stats: stats,
			},
		},
	}

	return reply, nil
}

// Restart implements the osapi.OSDServer interface.
func (r *Registrator) Restart(ctx context.Context, in *osapi.RestartRequest) (*osapi.RestartReply, error) {
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

	return &osapi.RestartReply{}, nil
}

// Dmesg implements the osapi.OSDServer interface. The klogctl syscall is used
// to read from the ring buffer at /proc/kmsg by taking the
// SYSLOG_ACTION_READ_ALL action. This action reads all messages remaining in
// the ring buffer non-destructively.
func (r *Registrator) Dmesg(ctx context.Context, in *empty.Empty) (data *osapi.DataReply, err error) {
	// Return the size of the kernel ring buffer
	size, err := unix.Klogctl(constants.SYSLOG_ACTION_SIZE_BUFFER, nil)
	if err != nil {
		return
	}
	// Read all messages from the log (non-destructively)
	buf := make([]byte, size)

	n, err := unix.Klogctl(constants.SYSLOG_ACTION_READ_ALL, buf)
	if err != nil {
		return
	}

	data = &osapi.DataReply{
		Response: []*osapi.DataResponse{
			{
				Bytes: &osapi.Data{Bytes: buf[:n]},
			},
		},
	}

	return data, err
}

// Logs implements the osapi.OSDServer interface. Service or container logs can
// be requested and the contents of the log file are streamed in chunks.
// nolint: gocyclo
func (r *Registrator) Logs(req *osapi.LogsRequest, l osapi.OS_LogsServer) (err error) {
	var chunk chunker.Chunker

	switch {
	case req.Namespace == "system" || req.Id == "kubelet" || req.Id == "kubeadm":
		filename := filepath.Join(constants.DefaultLogPath, filepath.Base(req.Id)+".log")

		var file *os.File

		file, err = os.OpenFile(filename, os.O_RDONLY, 0)
		if err != nil {
			return
		}
		// nolint: errcheck
		defer file.Close()

		chunk = filechunker.NewChunker(file)
	default:
		var file io.Closer

		if chunk, file, err = k8slogs(l.Context(), req); err != nil {
			return err
		}
		// nolint: errcheck
		defer file.Close()
	}

	for data := range chunk.Read(l.Context()) {
		if err = l.Send(&osapi.Data{Bytes: data}); err != nil {
			return
		}
	}

	return nil
}

// Version implements the osapi.OSDServer interface.
func (r *Registrator) Version(ctx context.Context, in *empty.Empty) (reply *machineapi.VersionReply, err error) {
	return r.MachineClient.Version(ctx, in)
}

// Processes implements the osapi.OSDServer interface
func (r *Registrator) Processes(ctx context.Context, in *empty.Empty) (reply *osapi.ProcessesReply, err error) {
	procs, err := procfs.AllProcs()
	if err != nil {
		return nil, err
	}

	processes := make([]*osapi.Process, 0, len(procs))

	var (
		command    string
		executable string
		args       []string
		stats      procfs.ProcStat
	)

	for _, proc := range procs {
		command, err = proc.Comm()
		if err != nil {
			return nil, err
		}

		executable, err = proc.Executable()
		if err != nil {
			return nil, err
		}

		args, err = proc.CmdLine()
		if err != nil {
			return nil, err
		}

		stats, err = proc.Stat()
		if err != nil {
			return nil, err
		}

		p := &osapi.Process{
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

	reply = &osapi.ProcessesReply{
		Response: []*osapi.ProcessResponse{
			{
				Processes: processes,
			},
		},
	}

	return reply, nil
}

func getContainerInspector(ctx context.Context, namespace string, driver osapi.ContainerDriver) (containers.Inspector, error) {
	switch driver {
	case osapi.ContainerDriver_CRI:
		if namespace != criconstants.K8sContainerdNamespace {
			return nil, errors.New("CRI inspector is supported only for K8s namespace")
		}

		return cri.NewInspector(ctx)
	case osapi.ContainerDriver_CONTAINERD:
		addr := constants.ContainerdAddress
		if namespace == constants.SystemContainerdNamespace {
			addr = constants.SystemContainerdAddress
		}

		return containerd.NewInspector(ctx, namespace, containerd.WithContainerdAddress(addr))
	default:
		return nil, fmt.Errorf("unsupported driver %q", driver)
	}
}

func k8slogs(ctx context.Context, req *osapi.LogsRequest) (chunker.Chunker, io.Closer, error) {
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

	return container.GetLogChunker()
}
