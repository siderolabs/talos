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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
	criconstants "github.com/containerd/cri/pkg/constants"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/prometheus/procfs"
	"github.com/rs/xid"
	"github.com/talos-systems/go-blockdevice/blockdevice/partition/gpt"
	"github.com/talos-systems/go-kmsg"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	installer "github.com/talos-systems/talos/cmd/installer/pkg/install"
	"github.com/talos-systems/talos/internal/app/machined/internal/install"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/disk"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/adv"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/app/resources"
	storaged "github.com/talos-systems/talos/internal/app/storaged"
	"github.com/talos-systems/talos/internal/pkg/configuration"
	"github.com/talos-systems/talos/internal/pkg/containers"
	taloscontainerd "github.com/talos-systems/talos/internal/pkg/containers/containerd"
	"github.com/talos-systems/talos/internal/pkg/containers/cri"
	"github.com/talos-systems/talos/internal/pkg/containers/image"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/internal/pkg/kubeconfig"
	"github.com/talos-systems/talos/internal/pkg/miniprocfs"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/archiver"
	"github.com/talos-systems/talos/pkg/chunker"
	"github.com/talos-systems/talos/pkg/chunker/stream"
	"github.com/talos-systems/talos/pkg/machinery/api/cluster"
	"github.com/talos-systems/talos/pkg/machinery/api/common"
	"github.com/talos-systems/talos/pkg/machinery/api/inspect"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/api/resource"
	"github.com/talos-systems/talos/pkg/machinery/api/storage"
	timeapi "github.com/talos-systems/talos/pkg/machinery/api/time"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	machinetype "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	timeresource "github.com/talos-systems/talos/pkg/machinery/resources/time"
	"github.com/talos-systems/talos/pkg/machinery/role"
	"github.com/talos-systems/talos/pkg/version"
)

// MinimumEtcdUpgradeLeaseLockSeconds indicates the minimum number of seconds for which we open a lease lock for upgrading Etcd nodes.
// This is not intended to lock for the duration of an upgrade.
// Rather, it is intended to make sure only one node processes the various pre-upgrade checks at a time.
// Thus, this timeout should be reflective of the expected time for the pre-upgrade checks, NOT the time to perform the upgrade itself.
const MinimumEtcdUpgradeLeaseLockSeconds = 60

// OSPathSeparator is the string version of the os.PathSeparator.
const OSPathSeparator = string(os.PathSeparator)

// Server implements ClusterService and MachineService APIs
// and is also responsible for registering ResourceServer and InspectServer.
type Server struct {
	cluster.UnimplementedClusterServiceServer
	machine.UnimplementedMachineServiceServer

	Controller runtime.Controller

	server *grpc.Server
}

func (s *Server) checkSupported(feature runtime.ModeCapability) error {
	mode := s.Controller.Runtime().State().Platform().Mode()

	if !mode.Supports(feature) {
		return fmt.Errorf("method is not supported in %s mode", mode.String())
	}

	return nil
}

func (s *Server) checkControlplane(apiName string) error {
	switch s.Controller.Runtime().Config().Machine().Type() { //nolint:exhaustive
	case machinetype.TypeControlPlane:
		fallthrough
	case machinetype.TypeInit:
		return nil
	}

	return status.Errorf(codes.Unimplemented, "%s is only available on control plane nodes", apiName)
}

// Register implements the factory.Registrator interface.
func (s *Server) Register(obj *grpc.Server) {
	s.server = obj

	machine.RegisterMachineServiceServer(obj, s)
	cluster.RegisterClusterServiceServer(obj, s)
	resource.RegisterResourceServiceServer(obj, &resources.Server{Resources: s.Controller.Runtime().State().V1Alpha2().Resources()})
	inspect.RegisterInspectServiceServer(obj, &InspectServer{server: s})
	storage.RegisterStorageServiceServer(obj, &storaged.Server{})
	timeapi.RegisterTimeServiceServer(obj, &TimeServer{ConfigProvider: s.Controller.Runtime()})
}

// ApplyConfiguration implements machine.MachineService.
//
//nolint:gocyclo
func (s *Server) ApplyConfiguration(ctx context.Context, in *machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
	log.Printf("apply config request: immediate %v, on reboot %v", in.Immediate, in.OnReboot)

	cfgProvider, err := s.Controller.Runtime().LoadAndValidateConfig(in.GetData())
	if err != nil {
		return nil, err
	}

	// --immediate
	if in.Immediate {
		if err = s.Controller.Runtime().CanApplyImmediate(cfgProvider); err != nil {
			return nil, err
		}
	}

	cfg, err := cfgProvider.Bytes()
	if err != nil {
		return nil, err
	}

	if err := ioutil.WriteFile(constants.ConfigPath, cfg, 0o600); err != nil {
		return nil, err
	}

	switch {
	// --immediate
	case in.Immediate:
		if err := s.Controller.Runtime().SetConfig(cfgProvider); err != nil {
			return nil, err
		}
	// default, no `--on-reboot`
	case !in.OnReboot:
		go func() {
			if err := s.Controller.Run(context.Background(), runtime.SequenceReboot, nil, runtime.WithTakeover()); err != nil {
				if !runtime.IsRebootError(err) {
					log.Println("apply configuration failed:", err)
				}

				if err != runtime.ErrLocked {
					s.server.GracefulStop()
				}
			}
		}()
	}

	return &machine.ApplyConfigurationResponse{
		Messages: []*machine.ApplyConfiguration{
			{},
		},
	}, nil
}

// GenerateConfiguration implements the machine.MachineServer interface.
func (s *Server) GenerateConfiguration(ctx context.Context, in *machine.GenerateConfigurationRequest) (reply *machine.GenerateConfigurationResponse, err error) {
	if s.Controller.Runtime().Config().Machine().Type() == machinetype.TypeWorker {
		return nil, fmt.Errorf("config can't be generated on worker nodes")
	}

	return configuration.Generate(ctx, in)
}

// Reboot implements the machine.MachineServer interface.
//
//nolint:dupl
func (s *Server) Reboot(ctx context.Context, in *machine.RebootRequest) (reply *machine.RebootResponse, err error) {
	log.Printf("reboot via API received")

	if err := s.checkSupported(runtime.Reboot); err != nil {
		return nil, err
	}

	go func() {
		if err := s.Controller.Run(context.Background(), runtime.SequenceReboot, in, runtime.WithTakeover()); err != nil {
			if !runtime.IsRebootError(err) {
				log.Println("reboot failed:", err)
			}

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
//nolint:gocyclo
func (s *Server) Rollback(ctx context.Context, in *machine.RollbackRequest) (*machine.RollbackResponse, error) {
	log.Printf("rollback via API received")

	if err := s.checkSupported(runtime.Rollback); err != nil {
		return nil, err
	}

	if err := func() error {
		if err := mount.SystemPartitionMount(s.Controller.Runtime(), nil, constants.BootPartitionLabel); err != nil {
			return fmt.Errorf("error mounting boot partition: %w", err)
		}

		defer func() {
			if err := mount.SystemPartitionUnmount(s.Controller.Runtime(), nil, constants.BootPartitionLabel); err != nil {
				log.Printf("failed unmounting boot partition: %s", err)
			}
		}()

		disk := s.Controller.Runtime().State().Machine().Disk(disk.WithPartitionLabel(constants.BootPartitionLabel))
		if disk == nil {
			return fmt.Errorf("boot disk not found")
		}

		grub := &grub.Grub{
			BootDisk: disk.Device().Name(),
		}

		_, next, err := grub.Labels()
		if err != nil {
			return err
		}

		if _, err = os.Stat(filepath.Join(constants.BootMountPoint, next)); errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot rollback to %q, label does not exist", next)
		}

		if err := grub.Default(next); err != nil {
			return fmt.Errorf("failed to revert bootloader: %v", err)
		}

		return nil
	}(); err != nil {
		return nil, err
	}

	go func() {
		if err := s.Controller.Run(context.Background(), runtime.SequenceReboot, in, runtime.WithForce(), runtime.WithTakeover()); err != nil {
			if !runtime.IsRebootError(err) {
				log.Println("reboot failed:", err)
			}

			if err != runtime.ErrLocked {
				// NB: We stop the gRPC server since a failed sequence triggers a
				// reboot.
				s.server.GracefulStop()
			}
		}
	}()

	return &machine.RollbackResponse{
		Messages: []*machine.Rollback{
			{},
		},
	}, nil
}

// Bootstrap implements the machine.MachineServer interface.
func (s *Server) Bootstrap(ctx context.Context, in *machine.BootstrapRequest) (reply *machine.BootstrapResponse, err error) {
	log.Printf("bootstrap request received")

	if s.Controller.Runtime().Config().Machine().Type() == machinetype.TypeWorker {
		return nil, status.Error(codes.FailedPrecondition, "bootstrap can only be performed on a control plane node")
	}

	timeCtx, timeCtxCancel := context.WithTimeout(ctx, 5*time.Second)
	defer timeCtxCancel()

	if err := timeresource.NewSyncCondition(s.Controller.Runtime().State().V1Alpha2().Resources()).Wait(timeCtx); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "time is not in sync yet")
	}

	if entries, _ := os.ReadDir(constants.EtcdDataPath); len(entries) > 0 { //nolint:errcheck
		return nil, status.Error(codes.AlreadyExists, "etcd data directory is not empty")
	}

	go func() {
		if err := s.Controller.Run(context.Background(), runtime.SequenceBootstrap, in); err != nil {
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
//nolint:dupl
func (s *Server) Shutdown(ctx context.Context, in *emptypb.Empty) (reply *machine.ShutdownResponse, err error) {
	log.Printf("shutdown via API received")

	if err = s.checkSupported(runtime.Shutdown); err != nil {
		return nil, err
	}

	go func() {
		if err := s.Controller.Run(context.Background(), runtime.SequenceShutdown, in, runtime.WithTakeover()); err != nil {
			if !runtime.IsRebootError(err) {
				log.Println("shutdown failed:", err)
			}

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
//nolint:gocyclo,cyclop
func (s *Server) Upgrade(ctx context.Context, in *machine.UpgradeRequest) (reply *machine.UpgradeResponse, err error) {
	var mu *concurrency.Mutex

	if err = s.checkSupported(runtime.Upgrade); err != nil {
		return nil, err
	}

	log.Printf("upgrade request received: preserve %v, staged %v, force %v", in.GetPreserve(), in.GetStage(), in.GetForce())

	log.Printf("validating %q", in.GetImage())

	if err = pullAndValidateInstallerImage(ctx, s.Controller.Runtime().Config().Machine().Registries(), in.GetImage()); err != nil {
		return nil, fmt.Errorf("error validating installer image %q: %w", in.GetImage(), err)
	}

	if s.Controller.Runtime().Config().Machine().Type() != machinetype.TypeWorker && !in.GetForce() {
		client, err := etcd.NewClientFromControlPlaneIPs(ctx, s.Controller.Runtime().Config().Cluster().CA(), s.Controller.Runtime().Config().Cluster().Endpoint())
		if err != nil {
			return nil, fmt.Errorf("failed to create etcd client: %w", err)
		}

		// acquire the upgrade mutex
		if mu, err = upgradeMutex(client); err != nil {
			return nil, fmt.Errorf("failed to acquire upgrade mutex: %w", err)
		}

		if err = mu.TryLock(ctx); err != nil {
			return nil, fmt.Errorf("failed to acquire upgrade lock: %w", err)
		}

		if err = client.ValidateForUpgrade(ctx, s.Controller.Runtime().Config(), in.GetPreserve()); err != nil {
			mu.Unlock(ctx) //nolint:errcheck

			return nil, fmt.Errorf("error validating etcd for upgrade: %w", err)
		}
	}

	runCtx := context.Background()

	if in.GetStage() {
		meta, err := bootloader.NewMeta()
		if err != nil {
			return nil, fmt.Errorf("error reading meta: %w", err)
		}
		//nolint:errcheck
		defer meta.Close()

		if !meta.ADV.SetTag(adv.StagedUpgradeImageRef, in.GetImage()) {
			return nil, fmt.Errorf("error adding staged upgrade image ref tag")
		}

		opts := install.DefaultInstallOptions()
		if err = opts.Apply(install.OptionsFromUpgradeRequest(s.Controller.Runtime(), in)...); err != nil {
			return nil, fmt.Errorf("error applying install options: %w", err)
		}

		serialized, err := json.Marshal(opts)
		if err != nil {
			return nil, fmt.Errorf("error serializing install options: %s", err)
		}

		if !meta.ADV.SetTag(adv.StagedUpgradeInstallOptions, string(serialized)) {
			return nil, fmt.Errorf("error adding staged upgrade install options tag")
		}

		if err = meta.Write(); err != nil {
			return nil, fmt.Errorf("error writing meta: %w", err)
		}

		go func() {
			if mu != nil {
				defer mu.Unlock(ctx) //nolint:errcheck
			}

			if err := s.Controller.Run(runCtx, runtime.SequenceStageUpgrade, in); err != nil {
				if !runtime.IsRebootError(err) {
					log.Println("reboot for staged upgrade failed:", err)
				}

				if err != runtime.ErrLocked {
					// NB: We stop the gRPC server since a failed sequence triggers a
					// reboot.
					s.server.GracefulStop()
				}
			}
		}()
	} else {
		go func() {
			if mu != nil {
				defer mu.Unlock(ctx) //nolint:errcheck
			}

			if err := s.Controller.Run(runCtx, runtime.SequenceUpgrade, in); err != nil {
				if !runtime.IsRebootError(err) {
					log.Println("upgrade failed:", err)
				}

				if err != runtime.ErrLocked {
					// NB: We stop the gRPC server since a failed sequence triggers a
					// reboot.
					s.server.GracefulStop()
				}
			}
		}()
	}

	reply = &machine.UpgradeResponse{
		Messages: []*machine.Upgrade{
			{
				Ack: "Upgrade request received",
			},
		},
	}

	return reply, nil
}

// ResetOptions implements runtime.ResetOptions interface.
type ResetOptions struct {
	*machine.ResetRequest

	systemDiskTargets []*installer.Target
}

// GetSystemDiskTargets implements runtime.ResetOptions interface.
func (opt *ResetOptions) GetSystemDiskTargets() []runtime.PartitionTarget {
	if opt.systemDiskTargets == nil {
		return nil
	}

	result := make([]runtime.PartitionTarget, len(opt.systemDiskTargets))

	for i := range result {
		result[i] = opt.systemDiskTargets[i]
	}

	return result
}

// Reset resets the node.
//
//nolint:gocyclo
func (s *Server) Reset(ctx context.Context, in *machine.ResetRequest) (reply *machine.ResetResponse, err error) {
	log.Printf("reset request received")

	opts := ResetOptions{
		ResetRequest: in,
	}

	if len(in.GetSystemPartitionsToWipe()) > 0 {
		bd := s.Controller.Runtime().State().Machine().Disk().BlockDevice

		var pt *gpt.GPT

		pt, err = bd.PartitionTable()
		if err != nil {
			return nil, fmt.Errorf("error reading partition table: %w", err)
		}

		for _, spec := range in.GetSystemPartitionsToWipe() {
			var target *installer.Target

			switch spec.Label {
			case constants.EFIPartitionLabel:
				target = installer.EFITarget(bd.Device().Name(), nil)
			case constants.BIOSGrubPartitionLabel:
				target = installer.BIOSTarget(bd.Device().Name(), nil)
			case constants.BootPartitionLabel:
				target = installer.BootTarget(bd.Device().Name(), nil)
			case constants.MetaPartitionLabel:
				target = installer.MetaTarget(bd.Device().Name(), nil)
			case constants.StatePartitionLabel:
				target = installer.StateTarget(bd.Device().Name(), installer.NoFilesystem)
			case constants.EphemeralPartitionLabel:
				target = installer.EphemeralTarget(bd.Device().Name(), installer.NoFilesystem)
			default:
				return nil, fmt.Errorf("label %q is not supported", spec.Label)
			}

			_, err = target.Locate(pt)
			if err != nil {
				return nil, fmt.Errorf("failed location partition with label %q: %w", spec.Label, err)
			}

			if spec.Wipe {
				opts.systemDiskTargets = append(opts.systemDiskTargets, target)
			}
		}
	}

	go func() {
		if err := s.Controller.Run(context.Background(), runtime.SequenceReset, &opts); err != nil {
			if !runtime.IsRebootError(err) {
				log.Println("reset failed:", err)
			}

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

// ServiceList returns list of the registered services and their status.
func (s *Server) ServiceList(ctx context.Context, in *emptypb.Empty) (result *machine.ServiceListResponse, err error) {
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
		//nolint:errcheck
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
//
//nolint:gocyclo
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

	var recursionDepth int

	if req.Recurse {
		if req.RecursionDepth == 0 {
			recursionDepth = -1
		} else {
			recursionDepth = int(req.RecursionDepth)
		}
	}

	opts := []archiver.WalkerOption{
		archiver.WithMaxRecurseDepth(recursionDepth),
	}

	if len(req.Types) > 0 {
		types := make([]archiver.FileType, 0, len(req.Types))

		for _, t := range req.Types {
			switch t {
			case machine.ListRequest_REGULAR:
				types = append(types, archiver.RegularFileType)
			case machine.ListRequest_DIRECTORY:
				types = append(types, archiver.DirectoryFileType)
			case machine.ListRequest_SYMLINK:
				types = append(types, archiver.SymlinkFileType)
			}
		}

		opts = append(opts, archiver.WithFileTypes(types...))
	}

	files, err := archiver.Walker(obj.Context(), req.Root, opts...)
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
				Uid:          fi.FileInfo.Sys().(*syscall.Stat_t).Uid,
				Gid:          fi.FileInfo.Sys().(*syscall.Stat_t).Gid,
			})
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// DiskUsage implements the machine.MachineServer interface.
//nolint:cyclop
func (s *Server) DiskUsage(req *machine.DiskUsageRequest, obj machine.MachineService_DiskUsageServer) error { //nolint:gocyclo
	if req == nil {
		req = new(machine.DiskUsageRequest)
	}

	for _, path := range req.Paths {
		if !strings.HasPrefix(path, OSPathSeparator) {
			// Make sure we use complete paths
			path = OSPathSeparator + path
		}

		path = strings.TrimSuffix(path, OSPathSeparator)
		if path == "" {
			path = "/"
		}

		_, err := os.Stat(path)
		if err == os.ErrNotExist {
			err = obj.Send(
				&machine.DiskUsageInfo{
					Name:         path,
					RelativeName: path,
					Error:        err.Error(),
				},
			)
			if err != nil {
				return err
			}

			continue
		}

		files, err := archiver.Walker(obj.Context(), path, archiver.WithMaxRecurseDepth(-1))
		if err != nil {
			err = obj.Send(
				&machine.DiskUsageInfo{
					Name:         path,
					RelativeName: path,
					Error:        err.Error(),
				},
			)
			if err != nil {
				return err
			}

			continue
		}

		folders := map[string]*machine.DiskUsageInfo{}

		// send a record back to client if the message shouldn't be skipped
		// at the same time use record information for folder size estimation
		sendSize := func(info *machine.DiskUsageInfo, depth int32, isDir bool) error {
			prefix := strings.TrimRight(filepath.Dir(info.Name), "/")
			if folder, ok := folders[prefix]; ok {
				folder.Size += info.Size
			}

			// recursion depth check
			skip := depth >= req.RecursionDepth && req.RecursionDepth > 0
			// skip files check
			skip = skip || !isDir && !req.All
			// threshold check
			skip = skip || req.Threshold > 0 && info.Size < req.Threshold
			skip = skip || req.Threshold < 0 && info.Size > -req.Threshold

			if skip {
				return nil
			}

			return obj.Send(info)
		}

		var (
			depth     int32
			prefix    = path
			rootDepth = int32(strings.Count(path, archiver.OSPathSeparator))
		)

		// flush all folder sizes until we get to the common prefix
		flushFolders := func(prefix, nextPrefix string) error {
			for !strings.HasPrefix(nextPrefix, prefix) {
				currentDepth := int32(strings.Count(prefix, archiver.OSPathSeparator)) - rootDepth

				if folder, ok := folders[prefix]; ok {
					err = sendSize(folder, currentDepth, true)
					if err != nil {
						return err
					}

					delete(folders, prefix)
				}

				prefix = strings.TrimRight(filepath.Dir(prefix), "/")
			}

			return nil
		}

		for fi := range files {
			if fi.Error != nil {
				err = obj.Send(
					&machine.DiskUsageInfo{
						Name:         fi.FullPath,
						RelativeName: fi.RelPath,
						Error:        fi.Error.Error(),
					},
				)
			} else {
				currentDepth := int32(strings.Count(fi.FullPath, archiver.OSPathSeparator)) - rootDepth
				size := fi.FileInfo.Size()
				if size < 0 {
					size = 0
				}

				// kcore file size gives wrong value, this code should be smarter when it reads it
				// TODO: figure out better way to skip such file
				if fi.FullPath == "/proc/kcore" {
					size = 0
				}

				if fi.FileInfo.IsDir() {
					folders[strings.TrimRight(fi.FullPath, "/")] = &machine.DiskUsageInfo{
						Name:         fi.FullPath,
						RelativeName: fi.RelPath,
						Size:         size,
					}
				} else {
					err = sendSize(&machine.DiskUsageInfo{
						Name:         fi.FullPath,
						RelativeName: fi.RelPath,
						Size:         size,
					}, currentDepth, false)

					if err != nil {
						return err
					}
				}

				// depth goes down when walker gets to the next sibling folder
				if currentDepth < depth {
					nextPrefix := fi.FullPath
					err = flushFolders(prefix, nextPrefix)

					if err != nil {
						return err
					}

					prefix = nextPrefix
				}

				if fi.FileInfo.IsDir() {
					prefix = fi.FullPath
				}
				depth = currentDepth
			}
		}

		if path != "" {
			p := strings.TrimRight(path, "/")
			if folder, ok := folders[p]; ok {
				err = flushFolders(prefix, p)
				if err != nil {
					return err
				}

				err = sendSize(folder, 0, true)

				if err != nil {
					return err
				}
			}
		}

		return nil
	}

	return nil
}

// Mounts implements the machine.MachineServer interface.
func (s *Server) Mounts(ctx context.Context, in *emptypb.Empty) (reply *machine.MountsResponse, err error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	//nolint:errcheck
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
func (s *Server) Version(ctx context.Context, in *emptypb.Empty) (reply *machine.VersionResponse, err error) {
	var platform *machine.PlatformInfo

	if s.Controller.Runtime().State().Platform() != nil {
		platform = &machine.PlatformInfo{
			Name: s.Controller.Runtime().State().Platform().Name(),
			Mode: s.Controller.Runtime().State().Platform().Mode().String(),
		}
	}

	features := &machine.FeaturesInfo{
		Rbac: s.Controller.Runtime().Config().Machine().Features().RBACEnabled(),
	}

	return &machine.VersionResponse{
		Messages: []*machine.Version{
			{
				Version:  version.NewVersion(),
				Platform: platform,
				Features: features,
			},
		},
	}, nil
}

// Kubeconfig implements the machine.MachineServer interface.
func (s *Server) Kubeconfig(empty *emptypb.Empty, obj machine.MachineService_KubeconfigServer) error {
	if err := s.checkControlplane("kubeconfig"); err != nil {
		return err
	}

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

		//nolint:errcheck
		defer logR.Close()

		chunk = stream.NewChunker(l.Context(), logR)
	default:
		var file io.Closer

		if chunk, file, err = k8slogs(l.Context(), req); err != nil {
			return err
		}
		//nolint:errcheck
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
	//nolint:errcheck
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
		addr := constants.CRIContainerdAddress
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

		defer f.Close() //nolint:errcheck

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
//nolint:gocyclo
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

	if err := s.Controller.Runtime().Events().Watch(func(events <-chan runtime.EventInfo) {
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

	//nolint:errcheck
	defer container.Delete(containerdctx, containerd.WithSnapshotCleanup)

	task, err := container.NewTask(containerdctx, cio.NullIO)
	if err != nil {
		return err
	}

	//nolint:errcheck
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
	//nolint:errcheck
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
func (s *Server) Stats(ctx context.Context, in *machine.StatsRequest) (reply *machine.StatsResponse, err error) {
	inspector, err := getContainerInspector(ctx, in.Namespace, in.Driver)
	if err != nil {
		return nil, err
	}
	//nolint:errcheck
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
	//nolint:errcheck
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
//nolint:gocyclo
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
	defer reader.Close() //nolint:errcheck

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
func (s *Server) Processes(ctx context.Context, in *emptypb.Empty) (reply *machine.ProcessesResponse, err error) {
	var processes []*machine.ProcessInfo

	procs, err := miniprocfs.NewProcesses()
	if err != nil {
		return nil, err
	}

	for {
		info, err := procs.Next()
		if err != nil {
			return nil, err
		}

		if info == nil {
			break
		}

		processes = append(processes, info)
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
func (s *Server) Memory(ctx context.Context, in *emptypb.Empty) (reply *machine.MemoryResponse, err error) {
	proc, err := procfs.NewDefaultFS()
	if err != nil {
		return nil, err
	}

	info, err := proc.Meminfo()
	if err != nil {
		return nil, err
	}

	meminfo := &machine.MemInfo{
		Memtotal:          pointer.GetUint64(info.MemTotal),
		Memfree:           pointer.GetUint64(info.MemFree),
		Memavailable:      pointer.GetUint64(info.MemAvailable),
		Buffers:           pointer.GetUint64(info.Buffers),
		Cached:            pointer.GetUint64(info.Cached),
		Swapcached:        pointer.GetUint64(info.SwapCached),
		Active:            pointer.GetUint64(info.Active),
		Inactive:          pointer.GetUint64(info.Inactive),
		Activeanon:        pointer.GetUint64(info.ActiveAnon),
		Inactiveanon:      pointer.GetUint64(info.InactiveAnon),
		Activefile:        pointer.GetUint64(info.ActiveFile),
		Inactivefile:      pointer.GetUint64(info.InactiveFile),
		Unevictable:       pointer.GetUint64(info.Unevictable),
		Mlocked:           pointer.GetUint64(info.Mlocked),
		Swaptotal:         pointer.GetUint64(info.SwapTotal),
		Swapfree:          pointer.GetUint64(info.SwapFree),
		Dirty:             pointer.GetUint64(info.Dirty),
		Writeback:         pointer.GetUint64(info.Writeback),
		Anonpages:         pointer.GetUint64(info.AnonPages),
		Mapped:            pointer.GetUint64(info.Mapped),
		Shmem:             pointer.GetUint64(info.Shmem),
		Slab:              pointer.GetUint64(info.Slab),
		Sreclaimable:      pointer.GetUint64(info.SReclaimable),
		Sunreclaim:        pointer.GetUint64(info.SUnreclaim),
		Kernelstack:       pointer.GetUint64(info.KernelStack),
		Pagetables:        pointer.GetUint64(info.PageTables),
		Nfsunstable:       pointer.GetUint64(info.NFSUnstable),
		Bounce:            pointer.GetUint64(info.Bounce),
		Writebacktmp:      pointer.GetUint64(info.WritebackTmp),
		Commitlimit:       pointer.GetUint64(info.CommitLimit),
		Committedas:       pointer.GetUint64(info.CommittedAS),
		Vmalloctotal:      pointer.GetUint64(info.VmallocTotal),
		Vmallocused:       pointer.GetUint64(info.VmallocUsed),
		Vmallocchunk:      pointer.GetUint64(info.VmallocChunk),
		Hardwarecorrupted: pointer.GetUint64(info.HardwareCorrupted),
		Anonhugepages:     pointer.GetUint64(info.AnonHugePages),
		Shmemhugepages:    pointer.GetUint64(info.ShmemHugePages),
		Shmempmdmapped:    pointer.GetUint64(info.ShmemPmdMapped),
		Cmatotal:          pointer.GetUint64(info.CmaTotal),
		Cmafree:           pointer.GetUint64(info.CmaFree),
		Hugepagestotal:    pointer.GetUint64(info.HugePagesTotal),
		Hugepagesfree:     pointer.GetUint64(info.HugePagesFree),
		Hugepagesrsvd:     pointer.GetUint64(info.HugePagesRsvd),
		Hugepagessurp:     pointer.GetUint64(info.HugePagesSurp),
		Hugepagesize:      pointer.GetUint64(info.Hugepagesize),
		Directmap4K:       pointer.GetUint64(info.DirectMap4k),
		Directmap2M:       pointer.GetUint64(info.DirectMap2M),
		Directmap1G:       pointer.GetUint64(info.DirectMap1G),
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

// EtcdMemberList implements the machine.MachineServer interface.
func (s *Server) EtcdMemberList(ctx context.Context, in *machine.EtcdMemberListRequest) (reply *machine.EtcdMemberListResponse, err error) {
	if err = s.checkControlplane("member list"); err != nil {
		return nil, err
	}

	var client *etcd.Client

	if in.QueryLocal {
		client, err = etcd.NewLocalClient()
	} else {
		client, err = etcd.NewClientFromControlPlaneIPs(ctx, s.Controller.Runtime().Config().Cluster().CA(), s.Controller.Runtime().Config().Cluster().Endpoint())
	}

	if err != nil {
		return nil, err
	}

	//nolint:errcheck
	defer client.Close()

	ctx = clientv3.WithRequireLeader(ctx)

	resp, err := client.MemberList(ctx)
	if err != nil {
		return nil, err
	}

	members := make([]*machine.EtcdMember, 0, len(resp.Members))
	legacyMembers := make([]string, 0, len(resp.Members))

	for _, member := range resp.Members {
		members = append(members,
			&machine.EtcdMember{
				Id:         member.GetID(),
				Hostname:   member.GetName(),
				PeerUrls:   member.GetPeerURLs(),
				ClientUrls: member.GetClientURLs(),
				IsLearner:  member.GetIsLearner(),
			},
		)

		legacyMembers = append(legacyMembers, member.GetName())
	}

	reply = &machine.EtcdMemberListResponse{
		Messages: []*machine.EtcdMembers{
			{
				LegacyMembers: legacyMembers,
				Members:       members,
			},
		},
	}

	return reply, nil
}

// EtcdRemoveMember implements the machine.MachineServer interface.
func (s *Server) EtcdRemoveMember(ctx context.Context, in *machine.EtcdRemoveMemberRequest) (reply *machine.EtcdRemoveMemberResponse, err error) {
	if err = s.checkControlplane("etcd remove member"); err != nil {
		return nil, err
	}

	client, err := etcd.NewClientFromControlPlaneIPs(ctx, s.Controller.Runtime().Config().Cluster().CA(), s.Controller.Runtime().Config().Cluster().Endpoint())
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	//nolint:errcheck
	defer client.Close()

	ctx = clientv3.WithRequireLeader(ctx)

	if err = client.RemoveMember(ctx, in.Member); err != nil {
		return nil, fmt.Errorf("failed to remove member: %w", err)
	}

	reply = &machine.EtcdRemoveMemberResponse{
		Messages: []*machine.EtcdRemoveMember{
			{},
		},
	}

	return reply, nil
}

// EtcdLeaveCluster implements the machine.MachineServer interface.
func (s *Server) EtcdLeaveCluster(ctx context.Context, in *machine.EtcdLeaveClusterRequest) (reply *machine.EtcdLeaveClusterResponse, err error) {
	if err = s.checkControlplane("etcd leave"); err != nil {
		return nil, err
	}

	client, err := etcd.NewClientFromControlPlaneIPs(ctx, s.Controller.Runtime().Config().Cluster().CA(), s.Controller.Runtime().Config().Cluster().Endpoint())
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	//nolint:errcheck
	defer client.Close()

	ctx = clientv3.WithRequireLeader(ctx)

	if err = client.LeaveCluster(ctx); err != nil {
		return nil, fmt.Errorf("failed to leave cluster: %w", err)
	}

	reply = &machine.EtcdLeaveClusterResponse{
		Messages: []*machine.EtcdLeaveCluster{
			{},
		},
	}

	return reply, nil
}

// EtcdForfeitLeadership implements the machine.MachineServer interface.
func (s *Server) EtcdForfeitLeadership(ctx context.Context, in *machine.EtcdForfeitLeadershipRequest) (reply *machine.EtcdForfeitLeadershipResponse, err error) {
	if err = s.checkControlplane("etcd forfeit leadership"); err != nil {
		return nil, err
	}

	client, err := etcd.NewClientFromControlPlaneIPs(ctx, s.Controller.Runtime().Config().Cluster().CA(), s.Controller.Runtime().Config().Cluster().Endpoint())
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	//nolint:errcheck
	defer client.Close()

	ctx = clientv3.WithRequireLeader(ctx)

	leader, err := client.ForfeitLeadership(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to forfeit leadership: %w", err)
	}

	reply = &machine.EtcdForfeitLeadershipResponse{
		Messages: []*machine.EtcdForfeitLeadership{
			{
				Member: leader,
			},
		},
	}

	return reply, nil
}

// EtcdSnapshot implements the machine.MachineServer interface.
func (s *Server) EtcdSnapshot(in *machine.EtcdSnapshotRequest, srv machine.MachineService_EtcdSnapshotServer) error {
	if err := s.checkControlplane("etcd snapshot"); err != nil {
		return err
	}

	client, err := etcd.NewLocalClient()
	if err != nil {
		return fmt.Errorf("failed to create etcd client: %w", err)
	}

	//nolint:errcheck
	defer client.Close()

	rd, err := client.Snapshot(srv.Context())
	if err != nil {
		return fmt.Errorf("failed reading etcd snapshot: %w", err)
	}

	ctx, cancel := context.WithCancel(srv.Context())
	defer cancel()

	chunker := stream.NewChunker(ctx, rd)
	chunkCh := chunker.Read()

	for data := range chunkCh {
		err := srv.SendMsg(&common.Data{Bytes: data})
		if err != nil {
			cancel()

			return err
		}
	}

	return nil
}

// EtcdRecover implements the machine.MachineServer interface.
func (s *Server) EtcdRecover(srv machine.MachineService_EtcdRecoverServer) error {
	if err := s.checkControlplane("etcd recover"); err != nil {
		return err
	}

	snapshot, err := os.OpenFile(constants.EtcdRecoverySnapshotPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return fmt.Errorf("error creating etcd recovery snapshot: %w", err)
	}

	defer snapshot.Close() //nolint:errcheck

	successfulUpload := false

	defer func() {
		if !successfulUpload {
			os.Remove(snapshot.Name()) //nolint:errcheck
		}
	}()

	for {
		var msg *common.Data

		msg, err = srv.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		_, err = snapshot.Write(msg.Bytes)
		if err != nil {
			return fmt.Errorf("error writing snapshot: %w", err)
		}
	}

	if err = snapshot.Sync(); err != nil {
		return fmt.Errorf("error fsyncing snapshot: %w", err)
	}

	if err = snapshot.Close(); err != nil {
		return fmt.Errorf("error closing snapshot: %w", err)
	}

	successfulUpload = true

	return srv.SendAndClose(&machine.EtcdRecoverResponse{
		Messages: []*machine.EtcdRecover{
			{},
		},
	})
}

// GenerateClientConfiguration implements the machine.MachineServer interface.
func (s *Server) GenerateClientConfiguration(ctx context.Context, in *machine.GenerateClientConfigurationRequest) (*machine.GenerateClientConfigurationResponse, error) {
	if s.Controller.Runtime().Config().Machine().Type() == machinetype.TypeWorker {
		return nil, status.Error(codes.FailedPrecondition, "client configuration (talosconfig) can't be generated on worker nodes")
	}

	crtTTL := in.CrtTtl.AsDuration()
	if crtTTL <= 0 {
		return nil, status.Error(codes.InvalidArgument, "crt_ttl should be positive")
	}

	ca := s.Controller.Runtime().Config().Machine().Security().CA()

	roles, _ := role.Parse(in.Roles)

	cert, err := generate.NewAdminCertificateAndKey(time.Now(), ca, roles, crtTTL)
	if err != nil {
		return nil, err
	}

	// make a nice context name
	contextName := s.Controller.Runtime().Config().Cluster().Name()
	if r := roles.Strings(); len(r) == 1 {
		contextName = strings.TrimPrefix(r[0], role.Prefix) + "@" + contextName
	}

	talosconfig := clientconfig.NewConfig(contextName, nil, ca.Crt, cert)

	b, err := talosconfig.Bytes()
	if err != nil {
		return nil, err
	}

	reply := &machine.GenerateClientConfigurationResponse{
		Messages: []*machine.GenerateClientConfiguration{
			{
				Ca:          ca.Crt,
				Crt:         cert.Crt,
				Key:         cert.Key,
				Talosconfig: b,
			},
		},
	}

	return reply, nil
}

func upgradeMutex(c *etcd.Client) (*concurrency.Mutex, error) {
	sess, err := concurrency.NewSession(c.Client,
		concurrency.WithTTL(MinimumEtcdUpgradeLeaseLockSeconds),
	)
	if err != nil {
		return nil, err
	}

	mu := concurrency.NewMutex(sess, constants.EtcdTalosEtcdUpgradeMutex)

	return mu, nil
}
