/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/containerd/cgroups"
	"github.com/containerd/containerd"
	tasks "github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	"github.com/containerd/typeurl"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/go-multierror"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	containerdrunner "github.com/talos-systems/talos/internal/app/init/pkg/system/runner/containerd"
	initproto "github.com/talos-systems/talos/internal/app/init/proto"
	"github.com/talos-systems/talos/internal/app/osd/proto"
	bullyproto "github.com/talos-systems/talos/internal/pkg/bully/proto"
	bully "github.com/talos-systems/talos/internal/pkg/bully/server"
	filechunker "github.com/talos-systems/talos/internal/pkg/chunker/file"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/proc"
	"github.com/talos-systems/talos/internal/pkg/version"
	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

// Registrator is the concrete type that implements the factory.Registrator and
// proto.OSDServer interfaces.
type Registrator struct {
	// Every Init service API is proxied via OSD.
	*InitServiceClient
	// Leadership election.
	*bully.Bully

	Data *userdata.UserData
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	proto.RegisterOSDServer(s, r)
	initproto.RegisterInitServer(s, r)
	bullyproto.RegisterBullyServer(s, r)
}

// Kubeconfig implements the proto.OSDServer interface. The admin kubeconfig is
// generated by kubeadm and placed at /etc/kubernetes/admin.conf. This method
// returns the contents of the generated admin.conf in the response.
func (r *Registrator) Kubeconfig(ctx context.Context, in *empty.Empty) (data *proto.Data, err error) {
	fileBytes, err := ioutil.ReadFile("/etc/kubernetes/admin.conf")
	if err != nil {
		return
	}
	data = &proto.Data{
		Bytes: fileBytes,
	}

	return data, err
}

// Processes implements the proto.OSDServer interface.
func (r *Registrator) Processes(ctx context.Context, in *proto.ProcessesRequest) (reply *proto.ProcessesReply, err error) {
	pods, err := podInfo(in.Namespace)
	if err != nil {
		return nil, err
	}

	processes := []*proto.Process{}

	for _, containers := range pods {
		for _, container := range containers.Containers {
			process := &proto.Process{
				Namespace: in.Namespace,
				Id:        container.Display,
				Image:     container.Image,
				Pid:       container.Pid,
				Status:    container.Status,
			}
			processes = append(processes, process)
		}
	}

	return &proto.ProcessesReply{Processes: processes}, nil

}

// Stats implements the proto.OSDServer interface.
// nolint: gocyclo
func (r *Registrator) Stats(ctx context.Context, in *proto.StatsRequest) (reply *proto.StatsReply, err error) {
	client, _, err := connect(in.Namespace)
	if err != nil {
		return nil, err
	}
	// nolint: errcheck
	defer client.Close()

	containers, err := client.Containers(ctx)
	if err != nil {
		return nil, err
	}

	stats := []*proto.Stat{}

	for _, container := range containers {
		task, err := container.Task(ctx, nil)
		if err != nil {
			log.Println(err)
			continue
		}

		status, err := task.Status(ctx)
		if err != nil {
			log.Println(err)
			continue
		}

		if status.Status == containerd.Running {
			metrics, err := task.Metrics(ctx)
			if err != nil {
				log.Println(err)
				continue
			}

			anydata, err := typeurl.UnmarshalAny(metrics.Data)
			if err != nil {
				log.Println(err)
				continue
			}

			data, ok := anydata.(*cgroups.Metrics)
			if !ok {
				log.Println(errors.New("failed to convert metric data to cgroups.Metrics"))
				continue
			}

			var used uint64
			mem := data.Memory
			if mem.Usage != nil {
				if mem.TotalInactiveFile < mem.Usage.Usage {
					used = mem.Usage.Usage - mem.TotalInactiveFile
				}
			}

			stat := &proto.Stat{
				Namespace:   in.Namespace,
				Id:          container.ID(),
				MemoryUsage: used,
				CpuUsage:    data.CPU.Usage.Total,
			}

			stats = append(stats, stat)
		}

	}

	reply = &proto.StatsReply{Stats: stats}

	return reply, nil
}

// Restart implements the proto.OSDServer interface.
func (r *Registrator) Restart(ctx context.Context, in *proto.RestartRequest) (reply *proto.RestartReply, err error) {
	ctx = namespaces.WithNamespace(ctx, in.Namespace)
	client, err := containerd.New(constants.ContainerdAddress)
	if err != nil {
		return nil, err
	}
	// nolint: errcheck
	defer client.Close()
	task := client.TaskService()
	_, err = task.Kill(ctx, &tasks.KillRequest{ContainerID: in.Id, Signal: uint32(unix.SIGTERM)})
	if err != nil {
		return nil, err
	}

	reply = &proto.RestartReply{}

	return
}

// Reset implements the proto.OSDServer interface.
func (r *Registrator) Reset(ctx context.Context, in *empty.Empty) (reply *proto.ResetReply, err error) {
	// TODO(andrewrynhard): Delete all system tasks and containers.

	// Set the process arguments.
	args := runner.Args{
		ID:          "reset",
		ProcessArgs: []string{"/bin/kubeadm", "reset", "--force"},
	}

	// Set the mounts.
	// nolint: dupl
	mounts := []specs.Mount{
		{Type: "cgroup", Destination: "/sys/fs/cgroup", Options: []string{"ro"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/docker", Source: "/var/lib/docker", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/crictl", Source: "/bin/crictl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/kubeadm", Source: "/bin/kubeadm", Options: []string{"bind", "ro"}},
	}

	cr := containerdrunner.NewRunner(
		r.Data,
		&args,
		runner.WithContainerImage(constants.KubernetesImage),
		runner.WithOCISpecOpts(
			containerdrunner.WithMemoryLimit(int64(1000000*512)),
			containerdrunner.WithRootfsPropagation("slave"),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithPrivileged,
		),
	)

	err = cr.Open(context.Background())
	if err != nil {
		return nil, err
	}

	// nolint: errcheck
	defer cr.Close()

	// TODO: should this go through system.Services?
	err = cr.Run(events.NullRecorder)
	if err != nil {
		return nil, err
	}

	reply = &proto.ResetReply{}

	return reply, nil
}

// Dmesg implements the proto.OSDServer interface. The klogctl syscall is used
// to read from the ring buffer at /proc/kmsg by taking the
// SYSLOG_ACTION_READ_ALL action. This action reads all messages remaining in
// the ring buffer non-destructively.
func (r *Registrator) Dmesg(ctx context.Context, in *empty.Empty) (data *proto.Data, err error) {
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

	data = &proto.Data{Bytes: buf[:n]}

	return data, err
}

// Logs implements the proto.OSDServer interface. Service or container logs can
// be requested and the contents of the log file are streamed in chunks.
// nolint: gocyclo
func (r *Registrator) Logs(req *proto.LogsRequest, l proto.OSD_LogsServer) (err error) {
	var (
		client *containerd.Client
		ctx    context.Context
		pods   []*pod
		task   *tasks.GetResponse
	)

	pods, err = podInfo(req.Namespace)
	if err != nil {
		return err
	}
	client, ctx, err = connect(req.Namespace)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer client.Close()

	for _, containers := range pods {
		for _, container := range containers.Containers {
			if container.Display != req.Id {
				continue
			}

			if container.LogFile == "" {
				task, err = client.TaskService().Get(ctx, &tasks.GetRequest{ContainerID: container.ID})
				if err != nil {
					return
				}

				container.LogFile = task.Process.Stdout
			}

			var file *os.File
			file, err = os.OpenFile(container.LogFile, os.O_RDONLY, 0)
			if err != nil {
				return
			}

			chunk := filechunker.NewChunker(file)

			if chunk == nil {
				err = errors.New("no log reader found")
				return
			}

			for data := range chunk.Read(l.Context()) {
				if err = l.Send(&proto.Data{Bytes: data}); err != nil {
					return
				}
			}
		}
	}

	return nil
}

// Routes implements the proto.OSDServer interface.
func (r *Registrator) Routes(ctx context.Context, in *empty.Empty) (data *proto.RoutesReply, err error) {
	routeList, err := netlink.RouteList(nil, 2)
	if err != nil {
		return nil, err
	}

	routes := []*proto.Route{}

	for _, route := range routeList {
		link, _err := netlink.LinkByIndex(route.LinkIndex)
		if _err != nil {
			err = _err
			return nil, err
		}

		destination := "0.0.0.0"
		if route.Dst != nil {
			destination = route.Dst.String()
		}

		gateway := "0.0.0.0"
		if route.Gw != nil {
			gateway = route.Gw.String()
		}

		routeMessage := &proto.Route{
			Interface:   link.Attrs().Name,
			Destination: destination,
			Gateway:     gateway,
		}
		routes = append(routes, routeMessage)
	}

	data = &proto.RoutesReply{Routes: routes}
	return data, err
}

// Version implements the proto.OSDServer interface.
func (r *Registrator) Version(ctx context.Context, in *empty.Empty) (data *proto.Data, err error) {
	v, err := version.NewVersion()
	if err != nil {
		return
	}

	data = &proto.Data{Bytes: []byte(v)}

	return data, err
}

// Top implements the proto.OSDServer interface
func (r *Registrator) Top(ctx context.Context, in *empty.Empty) (reply *proto.TopReply, err error) {
	var procs []proc.ProcessList
	procs, err = proc.List()
	if err != nil {
		return
	}

	var plist bytes.Buffer
	enc := gob.NewEncoder(&plist)
	err = enc.Encode(procs)
	if err != nil {
		return
	}

	p := &proto.ProcessList{Bytes: plist.Bytes()}
	reply = &proto.TopReply{ProcessList: p}
	return
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
