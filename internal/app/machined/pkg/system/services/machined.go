// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/siderolabs/go-debug"
	"google.golang.org/grpc"

	v1alpha1server "github.com/siderolabs/talos/internal/app/machined/internal/server/v1alpha1"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/grpc/factory"
	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

const machinedServiceID = "machined"

var rules = map[string]role.Set{
	"/cluster.ClusterService/HealthCheck": role.MakeSet(role.Admin, role.Operator, role.Reader),

	"/inspect.InspectService/ControllerRuntimeDependencies": role.MakeSet(role.Admin, role.Operator, role.Reader),

	"/machine.MachineService/ApplyConfiguration":          role.MakeSet(role.Admin),
	"/machine.MachineService/Bootstrap":                   role.MakeSet(role.Admin),
	"/machine.MachineService/CPUInfo":                     role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/CPUFreqStats":                role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/Containers":                  role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/Copy":                        role.MakeSet(role.Admin),
	"/machine.MachineService/DiskStats":                   role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/DiskUsage":                   role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/Dmesg":                       role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/EtcdAlarmList":               role.MakeSet(role.Admin, role.Operator, role.Reader, role.EtcdBackup),
	"/machine.MachineService/EtcdAlarmDisarm":             role.MakeSet(role.Admin, role.Operator),
	"/machine.MachineService/EtcdDefragment":              role.MakeSet(role.Admin, role.Operator),
	"/machine.MachineService/EtcdForfeitLeadership":       role.MakeSet(role.Admin),
	"/machine.MachineService/EtcdLeaveCluster":            role.MakeSet(role.Admin),
	"/machine.MachineService/EtcdMemberList":              role.MakeSet(role.Admin, role.Operator, role.Reader, role.EtcdBackup),
	"/machine.MachineService/EtcdRecover":                 role.MakeSet(role.Admin),
	"/machine.MachineService/EtcdRemoveMemberByID":        role.MakeSet(role.Admin),
	"/machine.MachineService/EtcdSnapshot":                role.MakeSet(role.Admin, role.Operator, role.EtcdBackup),
	"/machine.MachineService/EtcdStatus":                  role.MakeSet(role.Admin, role.Operator, role.Reader, role.EtcdBackup),
	"/machine.MachineService/Events":                      role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/GenerateClientConfiguration": role.MakeSet(role.Admin),
	"/machine.MachineService/GenerateConfiguration":       role.MakeSet(role.Admin),
	"/machine.MachineService/Hostname":                    role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/ImageList":                   role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/ImagePull":                   role.MakeSet(role.Admin, role.Operator),
	"/machine.MachineService/Kubeconfig":                  role.MakeSet(role.Admin),
	"/machine.MachineService/List":                        role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/LoadAvg":                     role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/Logs":                        role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/LogsContainers":              role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/Memory":                      role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/MetaWrite":                   role.MakeSet(role.Admin),
	"/machine.MachineService/MetaDelete":                  role.MakeSet(role.Admin),
	"/machine.MachineService/Mounts":                      role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/NetworkDeviceStats":          role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/Netstat":                     role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/PacketCapture":               role.MakeSet(role.Admin, role.Operator),
	"/machine.MachineService/Processes":                   role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/Read":                        role.MakeSet(role.Admin),
	"/machine.MachineService/Reboot":                      role.MakeSet(role.Admin, role.Operator),
	"/machine.MachineService/Reset":                       role.MakeSet(role.Admin),
	"/machine.MachineService/Restart":                     role.MakeSet(role.Admin, role.Operator),
	"/machine.MachineService/Rollback":                    role.MakeSet(role.Admin),
	"/machine.MachineService/ServiceList":                 role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/ServiceRestart":              role.MakeSet(role.Admin, role.Operator),
	"/machine.MachineService/ServiceStart":                role.MakeSet(role.Admin, role.Operator),
	"/machine.MachineService/ServiceStop":                 role.MakeSet(role.Admin, role.Operator),
	"/machine.MachineService/Shutdown":                    role.MakeSet(role.Admin, role.Operator),
	"/machine.MachineService/Stats":                       role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/SystemStat":                  role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/machine.MachineService/Upgrade":                     role.MakeSet(role.Admin),
	"/machine.MachineService/Version":                     role.MakeSet(role.Admin, role.Operator, role.Reader),

	// per-type authorization is handled by the service itself
	"/cosi.resource.State/Create":  role.MakeSet(role.Admin),
	"/cosi.resource.State/Destroy": role.MakeSet(role.Admin),
	"/cosi.resource.State/Get":     role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/cosi.resource.State/List":    role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/cosi.resource.State/Update":  role.MakeSet(role.Admin),
	"/cosi.resource.State/Watch":   role.MakeSet(role.Admin, role.Operator, role.Reader),

	"/storage.StorageService/Disks":           role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/storage.StorageService/BlockDeviceWipe": role.MakeSet(role.Admin),

	"/time.TimeService/Time":      role.MakeSet(role.Admin, role.Operator, role.Reader),
	"/time.TimeService/TimeCheck": role.MakeSet(role.Admin, role.Operator, role.Reader),
}

type machinedService struct {
	c runtime.Controller
}

// Main is an entrypoint to the API service.
func (s *machinedService) Main(ctx context.Context, _ runtime.Runtime, logWriter io.Writer) error {
	injector := &authz.Injector{
		Mode: authz.MetadataOnly,
	}

	if debug.Enabled {
		injector.Logger = log.New(logWriter, "machined/authz/injector ", log.Flags()).Printf
	}

	authorizer := &authz.Authorizer{
		Rules:         rules,
		FallbackRoles: role.MakeSet(role.Admin),
		Logger:        log.New(logWriter, "machined/authz/authorizer ", log.Flags()).Printf,
	}

	// Start the API server.
	server := factory.NewServer( //nolint:contextcheck
		&v1alpha1server.Server{
			Controller: s.c,
			// breaking the import loop cycle between services/ package and v1alpha1_server.go
			EtcdBootstrapper: BootstrapEtcd,

			ShutdownCtx: ctx,
		},
		factory.WithLog("machined ", logWriter),

		factory.ServerOptions(
			grpc.MaxRecvMsgSize(constants.GRPCMaxMessageSize),
		),

		factory.WithUnaryInterceptor(injector.UnaryInterceptor()),
		factory.WithStreamInterceptor(injector.StreamInterceptor()), //nolint:contextcheck

		factory.WithUnaryInterceptor(authorizer.UnaryInterceptor()),
		factory.WithStreamInterceptor(authorizer.StreamInterceptor()), //nolint:contextcheck
	)

	// ensure socket dir exists
	if err := os.MkdirAll(filepath.Dir(constants.MachineSocketPath), 0o770); err != nil {
		return err
	}

	// set the final leaf to be world-executable to make apid connect to the socket
	if err := os.Chmod(filepath.Dir(constants.MachineSocketPath), 0o771); err != nil {
		return err
	}

	listener, err := factory.NewListener(factory.Network("unix"), factory.SocketPath(constants.MachineSocketPath)) //nolint:contextcheck
	if err != nil {
		return err
	}

	if err := selinux.SetLabel(constants.MachineSocketPath, constants.MachineSocketLabel); err != nil {
		return err
	}

	// chown the socket path to make it accessible to the apid
	if err := os.Chown(constants.MachineSocketPath, constants.ApidUserID, constants.ApidUserID); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	closed := make(chan struct{})

	context.AfterFunc(ctx, func() {
		defer close(closed)

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		factory.ServerGracefulStop(server, shutdownCtx) //nolint:contextcheck
	})

	err = server.Serve(listener)

	cancel()
	<-closed

	return err
}

var _ system.HealthcheckedService = (*Machined)(nil)

// Machined implements the Service interface. It serves as the concrete type with
// the required methods.
type Machined struct {
	Controller runtime.Controller
}

// ID implements the Service interface.
func (m *Machined) ID(runtime.Runtime) string {
	return machinedServiceID
}

// PreFunc implements the Service interface.
func (m *Machined) PreFunc(context.Context, runtime.Runtime) error {
	return nil
}

// PostFunc implements the Service interface.
func (m *Machined) PostFunc(runtime.Runtime, events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (m *Machined) Condition(runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (m *Machined) DependsOn(runtime.Runtime) []string {
	return nil
}

// Volumes implements the Service interface.
func (m *Machined) Volumes(runtime.Runtime) []string {
	return nil
}

// Runner implements the Service interface.
func (m *Machined) Runner(r runtime.Runtime) (runner.Runner, error) {
	svc := &machinedService{m.Controller}

	return goroutine.NewRunner(r, machinedServiceID, svc.Main, runner.WithLoggingManager(r.Logging())), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (m *Machined) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		var d net.Dialer

		conn, err := d.DialContext(ctx, "unix", constants.MachineSocketPath)
		if err != nil {
			return err
		}

		return conn.Close()
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (m *Machined) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
