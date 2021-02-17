// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package server

import (
	"context"
	"fmt"
	"log"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	storaged "github.com/talos-systems/talos/internal/app/storaged"
	"github.com/talos-systems/talos/internal/pkg/configuration"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/api/network"
	"github.com/talos-systems/talos/pkg/machinery/api/storage"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	v1alpha1machine "github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// Server implements machine.MachineService.
type Server struct {
	machine.UnimplementedMachineServiceServer
	network.UnimplementedNetworkServiceServer
	storaged.Server
	runtime runtime.Runtime
	cfgCh   chan []byte
	logger  *log.Logger
	server  *grpc.Server
}

// New initializes and returns a `Server`.
func New(r runtime.Runtime, logger *log.Logger, cfgCh chan []byte) *Server {
	return &Server{
		logger:  logger,
		runtime: r,
		cfgCh:   cfgCh,
	}
}

// Register implements the factory.Registrator interface.
func (s *Server) Register(obj *grpc.Server) {
	s.server = obj

	storage.RegisterStorageServiceServer(obj, s)
	machine.RegisterMachineServiceServer(obj, s)
	network.RegisterNetworkServiceServer(obj, s)
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

	if err = cfgProvider.Validate(s.runtime.State().Platform().Mode()); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	reply = &machine.ApplyConfigurationResponse{
		Messages: []*machine.ApplyConfiguration{
			{},
		},
	}

	s.cfgCh <- in.GetData()

	return reply, nil
}

// GenerateConfiguration implements the machine.MachineServer interface.
// nolint:gocyclo
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

// Interfaces implements the machine.NetworkService interface.
func (s *Server) Interfaces(ctx context.Context, in *empty.Empty) (reply *network.InterfacesResponse, err error) {
	return networkd.GetDevices()
}
