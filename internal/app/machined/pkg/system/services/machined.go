// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:golint
package services

import (
	"context"
	"io"

	v1alpha1server "github.com/talos-systems/talos/internal/app/machined/internal/server/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

type machinedService struct {
	c runtime.Controller
}

// Main is an entrypoint the the API service.
func (s *machinedService) Main(ctx context.Context, r runtime.Runtime, logWriter io.Writer) error {
	// Start the API server.
	server := factory.NewServer(
		&v1alpha1server.Server{
			Controller: s.c,
		},
		factory.WithLog("machined ", logWriter),
	)

	listener, err := factory.NewListener(factory.Network("unix"), factory.SocketPath(constants.MachineSocketPath))
	if err != nil {
		return err
	}

	defer server.Stop()

	go func() {
		//nolint:errcheck
		server.Serve(listener)
	}()

	<-ctx.Done()

	return nil
}

// Machined implements the Service interface. It serves as the concrete type with
// the required methods.
type Machined struct {
	Controller runtime.Controller
}

// ID implements the Service interface.
func (m *Machined) ID(r runtime.Runtime) string {
	return "machined"
}

// PreFunc implements the Service interface.
func (m *Machined) PreFunc(ctx context.Context, r runtime.Runtime) error {
	return nil
}

// PostFunc implements the Service interface.
func (m *Machined) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (m *Machined) Condition(r runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (m *Machined) DependsOn(r runtime.Runtime) []string {
	return nil
}

// Runner implements the Service interface.
func (m *Machined) Runner(r runtime.Runtime) (runner.Runner, error) {
	svc := &machinedService{m.Controller}

	return goroutine.NewRunner(r, "machined", svc.Main, runner.WithLoggingManager(r.Logging())), nil
}
