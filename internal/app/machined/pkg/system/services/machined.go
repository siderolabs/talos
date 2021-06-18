// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"io"
	"log"

	v1alpha1server "github.com/talos-systems/talos/internal/app/machined/internal/server/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/grpc/middleware/authz"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/role"
)

var rules = map[string]role.Set{
	"/cluster.ClusterService/HealthCheck": role.MakeSet(role.Admin, role.Reader),

	"/inspect.InspectService/ControllerRuntimeDependencies": role.MakeSet(role.Admin, role.Reader),

	"/machine.MachineService/ApplyConfiguration":           role.MakeSet(role.Admin),
	"/machine.MachineService/Bootstrap":                    role.MakeSet(role.Admin),
	"/machine.MachineService/CPUInfo":                      role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/Containers":                   role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/Copy":                         role.MakeSet(role.Admin),
	"/machine.MachineService/DiskStats":                    role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/DiskUsage":                    role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/Dmesg":                        role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/EtcdForfeitLeadership":        role.MakeSet(role.Admin),
	"/machine.MachineService/EtcdLeaveCluster":             role.MakeSet(role.Admin),
	"/machine.MachineService/EtcdMemberList":               role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/EtcdRecover":                  role.MakeSet(role.Admin),
	"/machine.MachineService/EtcdRemoveMember":             role.MakeSet(role.Admin),
	"/machine.MachineService/EtcdSnapshot":                 role.MakeSet(role.Admin, role.EtcdBackup),
	"/machine.MachineService/Events":                       role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/GenerateClientConfiguration":  role.MakeSet(role.Admin),
	"/machine.MachineService/GenerateConfiguration":        role.MakeSet(role.Admin),
	"/machine.MachineService/Hostname":                     role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/Kubeconfig":                   role.MakeSet(role.Admin),
	"/machine.MachineService/List":                         role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/LoadAvg":                      role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/Logs":                         role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/Memory":                       role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/Mounts":                       role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/NetworkDeviceStats":           role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/Processes":                    role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/Read":                         role.MakeSet(role.Admin),
	"/machine.MachineService/Reboot":                       role.MakeSet(role.Admin),
	"/machine.MachineService/RemoveBootkubeInitializedKey": role.MakeSet(role.Admin),
	"/machine.MachineService/Reset":                        role.MakeSet(role.Admin),
	"/machine.MachineService/Restart":                      role.MakeSet(role.Admin),
	"/machine.MachineService/Rollback":                     role.MakeSet(role.Admin),
	"/machine.MachineService/ServiceList":                  role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/ServiceRestart":               role.MakeSet(role.Admin),
	"/machine.MachineService/ServiceStart":                 role.MakeSet(role.Admin),
	"/machine.MachineService/ServiceStop":                  role.MakeSet(role.Admin),
	"/machine.MachineService/Shutdown":                     role.MakeSet(role.Admin),
	"/machine.MachineService/Stats":                        role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/SystemStat":                   role.MakeSet(role.Admin, role.Reader),
	"/machine.MachineService/Upgrade":                      role.MakeSet(role.Admin),
	"/machine.MachineService/Version":                      role.MakeSet(role.Admin, role.Reader),

	"/network.NetworkService/Interfaces": role.MakeSet(role.Admin, role.Reader),
	"/network.NetworkService/Routes":     role.MakeSet(role.Admin, role.Reader),

	// per-type authorization is handled by the service itself
	"/resource.ResourceService": role.MakeSet(role.Admin, role.Reader),

	"/storage.StorageService/Disks": role.MakeSet(role.Admin, role.Reader),

	"/time.TimeService/Time":      role.MakeSet(role.Admin, role.Reader),
	"/time.TimeService/TimeCheck": role.MakeSet(role.Admin, role.Reader),
}

type machinedService struct {
	c runtime.Controller
}

// Main is an entrypoint the the API service.
func (s *machinedService) Main(ctx context.Context, r runtime.Runtime, logWriter io.Writer) error {
	injector := &authz.Injector{
		Mode:   authz.MetadataOnly,
		Logger: log.New(logWriter, "machined/authz/injector ", log.Flags()).Printf,
	}

	authorizer := &authz.Authorizer{
		Rules:         rules,
		FallbackRoles: role.MakeSet(role.Admin),
		Logger:        log.New(logWriter, "machined/authz/authorizer ", log.Flags()).Printf,
	}

	// Start the API server.
	server := factory.NewServer(
		&v1alpha1server.Server{
			Controller: s.c,
		},
		factory.WithLog("machined ", logWriter),

		factory.WithUnaryInterceptor(injector.UnaryInterceptor()),
		factory.WithStreamInterceptor(injector.StreamInterceptor()),

		factory.WithUnaryInterceptor(authorizer.UnaryInterceptor()),
		factory.WithStreamInterceptor(authorizer.StreamInterceptor()),
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
