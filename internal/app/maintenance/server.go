// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package maintenance

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"slices"
	"strings"

	cosiv1alpha1 "github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/protobuf/server"
	"github.com/google/uuid"
	"github.com/siderolabs/go-blockdevice/v2/block"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/resources"
	storaged "github.com/siderolabs/talos/internal/app/storaged"
	"github.com/siderolabs/talos/internal/pkg/configuration"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	v1alpha1machine "github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/role"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// Server implements [machine.MachineServiceServer], network.NetworkService, and [storage.StorageServiceServer].
type Server struct {
	machine.UnimplementedMachineServiceServer

	controller runtime.Controller
	cfgCh      chan<- config.Provider
	server     *grpc.Server

	mode runtime.Mode
}

// New initializes and returns a [Server].
func New(cfgCh chan<- config.Provider, mode runtime.Mode) *Server {
	if runtimeController == nil {
		panic("runtime controller is not set")
	}

	return &Server{
		controller: runtimeController,
		cfgCh:      cfgCh,
		mode:       mode,
	}
}

// Register implements the factory.Registrator interface.
func (s *Server) Register(obj *grpc.Server) {
	s.server = obj

	// wrap resources with access filter
	resourceState := s.controller.Runtime().State().V1Alpha2().Resources()
	resourceState = state.WrapCore(state.Filter(resourceState, resources.AccessPolicy(resourceState)))

	storage.RegisterStorageServiceServer(obj,
		&storaged.Server{
			Controller:      s.controller,
			MaintenanceMode: true,
		},
	)
	machine.RegisterMachineServiceServer(obj, s)
	cosiv1alpha1.RegisterStateServer(obj, server.NewState(resourceState))
}

// ApplyConfiguration implements [machine.MachineServiceServer].
func (s *Server) ApplyConfiguration(ctx context.Context, in *machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
	if s.mode.IsAgent() {
		return nil, status.Error(codes.Unimplemented, "API is not implemented in agent mode")
	}

	//nolint:exhaustive
	switch in.Mode {
	case machine.ApplyConfigurationRequest_TRY:
		fallthrough
	case machine.ApplyConfigurationRequest_REBOOT:
		fallthrough
	case machine.ApplyConfigurationRequest_AUTO:
	default:
		return nil, status.Errorf(codes.Unimplemented, "apply configuration --mode='%s' is not supported in maintenance mode",
			strings.ReplaceAll(strings.ToLower(in.Mode.String()), "_", "-"))
	}

	cfgProvider, err := configloader.NewFromBytes(in.GetData())
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	warnings, err := cfgProvider.Validate(s.controller.Runtime().State().Platform().Mode())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "configuration validation failed: %s", err)
	}

	warningsRuntime, err := cfgProvider.RuntimeValidate(ctx, s.controller.Runtime().State().V1Alpha2().Resources(), s.controller.Runtime().State().Platform().Mode())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "runtime configuration validation failed: %s", err)
	}

	reply := &machine.ApplyConfigurationResponse{
		Messages: []*machine.ApplyConfiguration{
			{
				Warnings: slices.Concat(warnings, warningsRuntime),
			},
		},
	}

	if in.DryRun {
		reply.Messages[0].ModeDetails = `Dry run summary:
Node is running in maintenance mode and does not have a config yet.`

		return reply, nil
	}

	s.cfgCh <- cfgProvider

	return reply, nil
}

// GenerateConfiguration implements the [machine.MachineServiceServer] interface.
func (s *Server) GenerateConfiguration(ctx context.Context, in *machine.GenerateConfigurationRequest) (*machine.GenerateConfigurationResponse, error) {
	if s.mode.IsAgent() {
		return nil, status.Error(codes.Unimplemented, "API is not implemented in agent mode")
	}

	if in.MachineConfig == nil {
		return nil, errors.New("invalid generate request")
	}

	machineType := v1alpha1machine.Type(in.MachineConfig.Type)

	if machineType == v1alpha1machine.TypeWorker {
		return nil, errors.New("join config can't be generated in the maintenance mode")
	}

	return configuration.Generate(ctx, in)
}

// GenerateClientConfiguration implements the [machine.MachineServiceServer] interface.
func (s *Server) GenerateClientConfiguration(context.Context, *machine.GenerateClientConfigurationRequest) (*machine.GenerateClientConfigurationResponse, error) {
	return nil, status.Error(codes.Unimplemented, "client configuration (talosconfig) can't be generated in the maintenance mode")
}

// Version implements the machine.MachineServer interface.
func (s *Server) Version(ctx context.Context, _ *emptypb.Empty) (*machine.VersionResponse, error) {
	if err := s.assertAdminRole(ctx); err != nil {
		return nil, err
	}

	var platform *machine.PlatformInfo

	if s.controller.Runtime().State().Platform() != nil {
		platform = &machine.PlatformInfo{
			Name: s.controller.Runtime().State().Platform().Name(),
			Mode: s.controller.Runtime().State().Platform().Mode().String(),
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

// Upgrade initiates an upgrade.
func (s *Server) Upgrade(ctx context.Context, in *machine.UpgradeRequest) (reply *machine.UpgradeResponse, err error) {
	if s.mode.IsAgent() {
		return nil, status.Error(codes.Unimplemented, "API is not implemented in agent mode")
	}

	if err = s.assertAdminRole(ctx); err != nil {
		return nil, err
	}

	if !s.controller.Runtime().State().Machine().Installed() {
		return nil, status.Errorf(codes.FailedPrecondition, "Talos is not installed")
	}

	actorID := uuid.New().String()

	mode := s.controller.Runtime().State().Platform().Mode()

	if !mode.Supports(runtime.Upgrade) {
		return nil, status.Errorf(codes.FailedPrecondition, "method is not supported in %s mode", mode.String())
	}

	// none of the options are supported in maintenance mode
	if in.GetPreserve() || in.GetStage() || in.GetForce() {
		return nil, status.Errorf(codes.Unimplemented, "upgrade --preserve, --stage, and --force are not supported in maintenance mode")
	}

	log.Printf("upgrade request received: %q", in.GetImage())

	runCtx := context.WithValue(context.Background(), runtime.ActorIDCtxKey{}, actorID)

	go func() {
		if err := s.controller.Run(runCtx, runtime.SequenceMaintenanceUpgrade, in); err != nil {
			if !runtime.IsRebootError(err) {
				log.Println("upgrade failed:", err)
			}
		}
	}()

	reply = &machine.UpgradeResponse{
		Messages: []*machine.Upgrade{
			{
				Ack:     "Upgrade request received",
				ActorId: actorID,
			},
		},
	}

	return reply, nil
}

// Reset resets the node.
//
//nolint:gocyclo
func (s *Server) Reset(ctx context.Context, in *machine.ResetRequest) (*machine.ResetResponse, error) {
	if s.mode.IsAgent() {
		return nil, status.Error(codes.Unimplemented, "API is not implemented in agent mode")
	}

	if err := s.assertAdminRole(ctx); err != nil {
		return nil, err
	}

	if in.UserDisksToWipe != nil && in.Mode == machine.ResetRequest_SYSTEM_DISK {
		return nil, errors.New("reset failed: invalid input, wipe mode SYSTEM_DISK doesn't support UserDisksToWipe parameter")
	}

	actorID := uuid.New().String()

	log.Printf("reset request received. actorID: %s", actorID)

	if len(in.GetSystemPartitionsToWipe()) > 0 {
		return nil, errors.New("system partitions to wipe params is not supported in the maintenance mode")
	}

	systemDisk, err := blockres.GetSystemDisk(ctx, s.controller.Runtime().State().V1Alpha2().Resources())
	if err != nil {
		return nil, err
	}

	if systemDisk == nil {
		return nil, errors.New("reset failed: Talos is not installed")
	}

	dev, err := block.NewFromPath(systemDisk.DevPath, block.OpenForWrite())
	if err != nil {
		return nil, err
	}

	defer dev.Close() //nolint:errcheck

	if err = dev.FastWipe(); err != nil {
		return nil, err
	}

	resetCtx := context.WithValue(context.Background(), runtime.ActorIDCtxKey{}, actorID)

	if in.Mode != machine.ResetRequest_SYSTEM_DISK {
		for _, deviceName := range in.UserDisksToWipe {
			dev, err = block.NewFromPath(deviceName, block.OpenForWrite())
			if err != nil {
				return nil, err
			}

			defer dev.Close() //nolint:errcheck

			log.Printf("wiping user disk %s", deviceName)

			err = dev.FastWipe()
			if err != nil {
				return nil, err
			}
		}
	}

	go func() {
		sequence := runtime.SequenceShutdown
		if in.Reboot {
			sequence = runtime.SequenceReboot
		}

		if err := s.controller.Run(resetCtx, sequence, in); err != nil {
			if !runtime.IsRebootError(err) {
				log.Println("reset failed:", err)
			}
		}
	}()

	return &machine.ResetResponse{
		Messages: []*machine.Reset{
			{
				ActorId: actorID,
			},
		},
	}, nil
}

// MetaWrite implements the [machine.MachineServiceServer] interface.
func (s *Server) MetaWrite(ctx context.Context, req *machine.MetaWriteRequest) (*machine.MetaWriteResponse, error) {
	if err := s.assertAdminRole(ctx); err != nil {
		return nil, err
	}

	if uint32(uint8(req.Key)) != req.Key {
		return nil, status.Errorf(codes.InvalidArgument, "key must be a uint8")
	}

	ok, err := s.controller.Runtime().State().Machine().Meta().SetTagBytes(ctx, uint8(req.Key), req.Value)
	if err != nil {
		return nil, err
	}

	if !ok {
		// META overflowed
		return nil, status.Errorf(codes.ResourceExhausted, "meta write failed")
	}

	err = s.controller.Runtime().State().Machine().Meta().Flush()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		// ignore not exist error, as it's possible that the meta partition is not created yet
		return nil, err
	}

	return &machine.MetaWriteResponse{
		Messages: []*machine.MetaWrite{{}},
	}, nil
}

// MetaDelete implements the [machine.MachineServiceServer] interface.
func (s *Server) MetaDelete(ctx context.Context, req *machine.MetaDeleteRequest) (*machine.MetaDeleteResponse, error) {
	if err := s.assertAdminRole(ctx); err != nil {
		return nil, err
	}

	if uint32(uint8(req.Key)) != req.Key {
		return nil, status.Errorf(codes.InvalidArgument, "key must be a uint8")
	}

	ok, err := s.controller.Runtime().State().Machine().Meta().DeleteTag(ctx, uint8(req.Key))
	if err != nil {
		return nil, err
	}

	if !ok {
		// META key not found
		return nil, status.Errorf(codes.NotFound, "meta key not found")
	}

	err = s.controller.Runtime().State().Machine().Meta().Flush()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		// ignore not exist error, as it's possible that the meta partition is not created yet
		return nil, err
	}

	return &machine.MetaDeleteResponse{
		Messages: []*machine.MetaDelete{{}},
	}, nil
}

func (s *Server) assertAdminRole(ctx context.Context) error {
	if !authz.HasRole(ctx, role.Admin) {
		return status.Error(codes.Unimplemented, "API is not implemented in maintenance mode")
	}

	return nil
}
