/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package api

import (
	"context"
	"io"

	"github.com/talos-systems/talos/internal/app/machined/internal/api/reg"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/factory"
)

// Service wraps machined API server
type Service struct{}

// NewService creates new Service
func NewService() *Service {
	return &Service{}
}

// Main is an entrypoint the the API service
func (s *Service) Main(ctx context.Context, config runtime.Configurator, logWriter io.Writer) error {
	api := reg.NewRegistrator(config)
	server := factory.NewServer(api)

	listener, err := factory.NewListener(factory.Network("unix"), factory.SocketPath(constants.InitSocketPath))
	if err != nil {
		return err
	}

	defer server.Stop()

	go func() {
		// nolint: errcheck
		server.Serve(listener)
	}()

	<-ctx.Done()

	return nil
}
