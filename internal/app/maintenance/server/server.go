// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/resources"
	storaged "github.com/talos-systems/talos/internal/app/storaged"
	"github.com/talos-systems/talos/internal/pkg/configuration"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/api/resource"
	"github.com/talos-systems/talos/pkg/machinery/api/storage"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	v1alpha1machine "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/version"
)

// Server implements machine.MachineService, network.NetworkService, and storage.StorageService.
type Server struct {
	machine.UnimplementedMachineServiceServer

	runtime runtime.Runtime
	logger  *log.Logger
	cfgCh   chan []byte
	server  *grpc.Server
}

// New initializes and returns a `Server`.
func New(r runtime.Runtime, logger *log.Logger, cfgCh chan []byte) *Server {
	return &Server{
		runtime: r,
		logger:  logger,
		cfgCh:   cfgCh,
	}
}

// Register implements the factory.Registrator interface.
func (s *Server) Register(obj *grpc.Server) {
	s.server = obj

	// wrap resources with access filter
	resourceState := s.runtime.State().V1Alpha2().Resources()
	resourceState = state.WrapCore(state.Filter(resourceState, resources.AccessPolicy(resourceState)))

	storage.RegisterStorageServiceServer(obj, &storaged.Server{})
	machine.RegisterMachineServiceServer(obj, s)
	resource.RegisterResourceServiceServer(obj, &resources.Server{Resources: resourceState})
}

// ApplyConfiguration implements machine.MachineService.
func (s *Server) ApplyConfiguration(ctx context.Context, in *machine.ApplyConfigurationRequest) (*machine.ApplyConfigurationResponse, error) {
	//nolint:exhaustive
	switch in.Mode {
	case machine.ApplyConfigurationRequest_TRY:
		fallthrough
	case machine.ApplyConfigurationRequest_REBOOT:
		fallthrough
	case machine.ApplyConfigurationRequest_AUTO:
	default:
		return nil, fmt.Errorf("apply configuration --mode='%s' is not supported in maintenance mode",
			strings.ReplaceAll(strings.ToLower(in.Mode.String()), "_", "-"))
	}

	cfgProvider, err := configloader.NewFromBytes(in.GetData())
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	warnings, err := cfgProvider.Validate(s.runtime.State().Platform().Mode())
	if err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	reply := &machine.ApplyConfigurationResponse{
		Messages: []*machine.ApplyConfiguration{
			{
				Warnings: warnings,
			},
		},
	}

	if in.DryRun {
		reply.Messages[0].ModeDetails = `Dry run summary:
Node is running in maintenance mode and does not have a config yet.`

		return reply, nil
	}

	s.cfgCh <- in.GetData()

	return reply, nil
}

// GenerateConfiguration implements the machine.MachineServer interface.
func (s *Server) GenerateConfiguration(ctx context.Context, in *machine.GenerateConfigurationRequest) (*machine.GenerateConfigurationResponse, error) {
	if in.MachineConfig == nil {
		return nil, fmt.Errorf("invalid generate request")
	}

	machineType := v1alpha1machine.Type(in.MachineConfig.Type)

	if machineType == v1alpha1machine.TypeWorker {
		return nil, fmt.Errorf("join config can't be generated in the maintenance mode")
	}

	return configuration.Generate(ctx, in)
}

// GenerateClientConfiguration implements the machine.MachineServer interface.
func (s *Server) GenerateClientConfiguration(ctx context.Context, in *machine.GenerateClientConfigurationRequest) (*machine.GenerateClientConfigurationResponse, error) {
	return nil, status.Error(codes.Unimplemented, "client configuration (talosconfig) can't be generated in the maintenance mode")
}

func verifyPeer(ctx context.Context, condition func(netaddr.IP) bool) bool {
	remotePeer, ok := peer.FromContext(ctx)
	if !ok {
		return false
	}

	if remotePeer.Addr.Network() != "tcp" {
		return false
	}

	ip, _, err := net.SplitHostPort(remotePeer.Addr.String())
	if err != nil {
		return false
	}

	addr, err := netaddr.ParseIP(ip)
	if err != nil {
		return false
	}

	return condition(addr)
}

// Version implements the machine.MachineServer interface.
func (s *Server) Version(ctx context.Context, in *emptypb.Empty) (*machine.VersionResponse, error) {
	if !verifyPeer(ctx, func(addr netaddr.IP) bool {
		return network.IsULA(addr, network.ULASideroLink)
	}) {
		return nil, status.Error(codes.Unimplemented, "Version API is not implemented in maintenance mode")
	}

	var platform *machine.PlatformInfo

	if s.runtime.State().Platform() != nil {
		platform = &machine.PlatformInfo{
			Name: s.runtime.State().Platform().Name(),
			Mode: s.runtime.State().Platform().Mode().String(),
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
