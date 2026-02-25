// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lifecycle implements machine.LifecycleService.
package lifecycle

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/install"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
	crires "github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// Service implements machine.LifecycleService.
type Service struct {
	machine.UnimplementedLifecycleServiceServer

	lock    sync.Mutex
	runtime runtime.Runtime
}

// NewService creates a new instance of the lifecycle service.
func NewService(runtime runtime.Runtime) *Service {
	return &Service{
		lock:    sync.Mutex{},
		runtime: runtime,
	}
}

// Install handles the installation of the machine.
// It ensures that only one installation or upgrade can occur at a time by using a mutex lock.
func (s *Service) Install(req *machine.LifecycleServiceInstallRequest, ss grpc.ServerStreamingServer[machine.LifecycleServiceInstallResponse]) error {
	ctx := ss.Context()

	if !s.lock.TryLock() {
		return status.Error(codes.FailedPrecondition, "another installation/upgrade is already in progress")
	}
	defer s.lock.Unlock()

	if s.runtime.State().Platform().Mode().InContainer() {
		return status.Error(codes.FailedPrecondition, "installation is not supported in container mode")
	}

	if s.runtime.State().Machine().Installed() {
		return status.Error(codes.AlreadyExists, "machine is already installed")
	}

	if err := crires.WaitForImageCache(ctx, s.runtime.State().V1Alpha2().Resources()); err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("failed to wait for the image cache: %v", err))
	}

	installerImage := req.GetSource().GetImageName()
	if installerImage == "" {
		return status.Error(codes.InvalidArgument, "installer image name is required")
	}

	disk := req.GetDestination().GetDisk()
	if disk == "" {
		return status.Error(codes.InvalidArgument, "destination disk is required")
	}

	targetDisk, err := filepath.EvalSymlinks(disk)
	if err != nil {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("invalid disk path: %v", err))
	}

	log.Printf("starting installation: installer_image=%s, disk=%s", installerImage, targetDisk)

	//nolint:dupl
	err = runInstallerContainer(ctx, &containerRunConfig{
		containerdInst: req.GetContainerd(),
		imageRef:       installerImage,
		disk:           targetDisk,
		platform:       s.runtime.State().Platform().Name(),
		cfgContainer:   s.runtime.ConfigContainer(),
		opts: []install.Option{
			install.WithForce(true),
			install.WithPull(false),
			install.WithZero(false),
		},
		send: func(msg string) error {
			return ss.Send(&machine.LifecycleServiceInstallResponse{
				Progress: &machine.LifecycleServiceInstallProgress{
					Response: &machine.LifecycleServiceInstallProgress_Message{
						Message: msg,
					},
				},
			})
		},
		sendExitCode: func(exitCode int32) error {
			return ss.Send(&machine.LifecycleServiceInstallResponse{
				Progress: &machine.LifecycleServiceInstallProgress{
					Response: &machine.LifecycleServiceInstallProgress_ExitCode{
						ExitCode: exitCode,
					},
				},
			})
		},
	})
	if err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("installation failed: %v", err))
	}

	return nil
}

// Upgrade handles the upgrade of the machine.
// It ensures that only one installation or upgrade can occur at a time by using a mutex lock.
func (s *Service) Upgrade(req *machine.LifecycleServiceUpgradeRequest, ss grpc.ServerStreamingServer[machine.LifecycleServiceUpgradeResponse]) error {
	ctx := ss.Context()

	if !s.lock.TryLock() {
		return status.Error(codes.FailedPrecondition, "another installation/upgrade is already in progress")
	}
	defer s.lock.Unlock()

	if s.runtime.State().Platform().Mode().InContainer() {
		return status.Error(codes.FailedPrecondition, "upgrade is not supported in container mode")
	}

	if !s.runtime.State().Machine().Installed() {
		return status.Error(codes.FailedPrecondition, "machine is not installed")
	}

	installerImage := req.GetSource().GetImageName()
	if installerImage == "" {
		return status.Error(codes.InvalidArgument, "installer image name is required")
	}

	systemDisk, err := blockres.GetSystemDisk(ctx, s.runtime.State().V1Alpha2().Resources())
	if err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("failed to get system disk: %v", err))
	}

	if systemDisk == nil {
		return status.Error(codes.Internal, "system disk not found")
	}

	devname := systemDisk.DevPath

	log.Printf("starting upgrade: installer_image=%s, system_disk=%s", installerImage, devname)

	//nolint:dupl
	err = runInstallerContainer(ctx, &containerRunConfig{
		containerdInst: req.GetContainerd(),
		imageRef:       installerImage,
		disk:           devname,
		platform:       s.runtime.State().Platform().Name(),
		cfgContainer:   s.runtime.ConfigContainer(),
		opts: []install.Option{
			install.WithPull(false),
			install.WithUpgrade(true),
			install.WithForce(false),
		},
		send: func(msg string) error {
			return ss.Send(&machine.LifecycleServiceUpgradeResponse{
				Progress: &machine.LifecycleServiceInstallProgress{
					Response: &machine.LifecycleServiceInstallProgress_Message{
						Message: msg,
					},
				},
			})
		},
		sendExitCode: func(exitCode int32) error {
			return ss.Send(&machine.LifecycleServiceUpgradeResponse{
				Progress: &machine.LifecycleServiceInstallProgress{
					Response: &machine.LifecycleServiceInstallProgress_ExitCode{
						ExitCode: exitCode,
					},
				},
			})
		},
	})
	if err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("upgrade failed: %v", err))
	}

	return nil
}
