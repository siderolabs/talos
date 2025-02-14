// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"io"
	"os"

	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services/registry"
	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/logging"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

type registryD struct{}

// RegistryID is the ID of the registry service.
const RegistryID = "registryd"

// NewRegistryD returns a new docker mirror registry service.
func NewRegistryD() system.Service                                       { return &registryD{} }
func (r *registryD) ID(runtime.Runtime) string                           { return RegistryID }
func (r *registryD) HealthSettings(runtime.Runtime) *health.Settings     { return &health.DefaultSettings }
func (r *registryD) PreFunc(context.Context, runtime.Runtime) error      { return nil }
func (r *registryD) PostFunc(runtime.Runtime, events.ServiceState) error { return nil }
func (r *registryD) Condition(runtime.Runtime) conditions.Condition      { return nil }
func (r *registryD) DependsOn(runtime.Runtime) []string                  { return nil }
func (r *registryD) Volumes() []string                                   { return nil }

func (r *registryD) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error {
		return simpleHealthCheck(ctx, "http://"+constants.RegistrydListenAddress+"/healthz")
	}
}

func (r *registryD) Runner(rt runtime.Runtime) (runner.Runner, error) {
	return goroutine.NewRunner(rt, "registryd", func(ctx context.Context, r runtime.Runtime, logOutput io.Writer) error {
		logger := logging.ZapLogger(
			logging.NewLogDestination(logOutput, zapcore.DebugLevel, logging.WithColoredLevels()),
		)

		st := r.State().V1Alpha2().Resources()
		it := func(yield func(string) bool) {
			imageCacheConfig, err := safe.StateGetByID[*cri.ImageCacheConfig](ctx, st, cri.ImageCacheConfigID)
			if err != nil {
				logger.Error("failed to get image cache config", zap.Error(err))

				return
			}

			for _, root := range imageCacheConfig.TypedSpec().Roots {
				if _, err = os.Stat(root); err != nil {
					logger.Error("failed to stat image cache root", zap.String("root", root), zap.Error(err))

					continue
				}

				if !yield(root) {
					return
				}
			}
		}

		return registry.NewService(registry.NewMultiPathFS(it), logger).Run(ctx)
	}, runner.WithLoggingManager(rt.Logging())), nil
}
