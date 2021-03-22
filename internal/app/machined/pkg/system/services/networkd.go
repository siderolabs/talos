// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:golint
package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner/restart"
	"github.com/talos-systems/talos/internal/app/networkd"
	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/grpc/dialer"
	healthapi "github.com/talos-systems/talos/pkg/machinery/api/health"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Networkd implements the Service interface. It serves as the concrete type with
// the required methods.
type Networkd struct{}

// ID implements the Service interface.
func (n *Networkd) ID(r runtime.Runtime) string {
	return "networkd"
}

// PreFunc implements the Service interface.
func (n *Networkd) PreFunc(ctx context.Context, r runtime.Runtime) error {
	return os.MkdirAll(filepath.Dir(constants.NetworkSocketPath), 0o750)
}

// PostFunc implements the Service interface.
func (n *Networkd) PostFunc(r runtime.Runtime, state events.ServiceState) (err error) {
	return nil
}

// Condition implements the Service interface.
func (n *Networkd) Condition(r runtime.Runtime) conditions.Condition {
	return nil
}

// DependsOn implements the Service interface.
func (n *Networkd) DependsOn(r runtime.Runtime) []string {
	return nil
}

func (n *Networkd) Runner(r runtime.Runtime) (runner.Runner, error) {
	return restart.New(goroutine.NewRunner(
		r,
		"networkd",
		networkd.Main,
		runner.WithLoggingManager(r.Logging()),
	),
		restart.WithType(restart.Forever),
	), nil
}

// HealthFunc implements the HealthcheckedService interface.
func (n *Networkd) HealthFunc(r runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		var (
			conn      *grpc.ClientConn
			err       error
			hcResp    *healthapi.HealthCheckResponse
			readyResp *healthapi.ReadyCheckResponse
		)

		conn, err = grpc.DialContext(
			ctx,
			fmt.Sprintf("%s://%s", "unix", constants.NetworkSocketPath),
			grpc.WithInsecure(),
			grpc.WithContextDialer(dialer.DialUnix()),
		)
		if err != nil {
			return err
		}
		defer conn.Close() //nolint:errcheck

		nClient := healthapi.NewHealthClient(conn)
		if readyResp, err = nClient.Ready(ctx, &empty.Empty{}); err != nil {
			return err
		}

		if readyResp.Messages[0].Status != healthapi.ReadyCheck_READY {
			return errors.New("networkd is not ready")
		}

		if hcResp, err = nClient.Check(ctx, &empty.Empty{}); err != nil {
			return err
		}

		if hcResp.Messages[0].Status == healthapi.HealthCheck_SERVING {
			return nil
		}

		msg := fmt.Sprintf("networkd is unhealthy: %s", hcResp.Messages[0].Status.String())

		if r.Config().Debug() {
			log.Printf("DEBUG: %s", msg)
		}

		return errors.New(msg)
	}
}

// HealthSettings implements the HealthcheckedService interface.
func (n *Networkd) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.DefaultSettings
}
