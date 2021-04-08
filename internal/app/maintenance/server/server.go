// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	networkserver "github.com/talos-systems/talos/internal/app/networkd/pkg/server"
	storaged "github.com/talos-systems/talos/internal/app/storaged"
	"github.com/talos-systems/talos/internal/pkg/configuration"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/api/network"
	"github.com/talos-systems/talos/pkg/machinery/api/storage"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	v1alpha1machine "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
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

	storage.RegisterStorageServiceServer(obj, &storaged.Server{})
	machine.RegisterMachineServiceServer(obj, s)
	network.RegisterNetworkServiceServer(obj, &networkserver.NetworkServer{})
}

// ApplyConfiguration implements machine.MachineService.
func (s *Server) ApplyConfiguration(ctx context.Context, in *machine.ApplyConfigurationRequest) (reply *machine.ApplyConfigurationResponse, err error) {
	if in.OnReboot {
		return nil, fmt.Errorf("apply configuration on reboot is not supported in maintenance mode")
	}

	cfgProvider, err := configloader.NewFromBytes(in.GetData())
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	warnings, err := cfgProvider.Validate(s.runtime.State().Platform().Mode())
	if err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	reply = &machine.ApplyConfigurationResponse{
		Messages: []*machine.ApplyConfiguration{
			{
				Warnings: warnings,
			},
		},
	}

	s.cfgCh <- in.GetData()

	return reply, nil
}

// GenerateConfiguration implements the machine.MachineServer interface.
func (s *Server) GenerateConfiguration(ctx context.Context, in *machine.GenerateConfigurationRequest) (reply *machine.GenerateConfigurationResponse, err error) {
	if in.MachineConfig == nil {
		return nil, fmt.Errorf("invalid generate request")
	}

	machineType := v1alpha1machine.Type(in.MachineConfig.Type)

	if machineType == v1alpha1machine.TypeJoin {
		return nil, fmt.Errorf("join config cannot be generated in the maintenance mode")
	}

	return configuration.Generate(ctx, in)
}
