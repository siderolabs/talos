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
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
)

// Server implements machine.MaintenanceService.
type Server struct {
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

	machine.RegisterMaintenanceServiceServer(obj, s)
}

// ApplyConfiguration implements machine.MaintenanceService.
func (s *Server) ApplyConfiguration(ctx context.Context, in *machine.ApplyConfigurationRequest) (reply *machine.ApplyConfigurationResponse, err error) {
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
