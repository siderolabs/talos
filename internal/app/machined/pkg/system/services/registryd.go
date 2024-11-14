// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"
	"io"
	"io/fs"
	"os"

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
)

type registryD struct{}

// NewRegistryD returns a new docker mirror registry service.
func NewRegistryD() system.Service                                       { return &registryD{} }
func (r *registryD) ID(runtime.Runtime) string                           { return "registryd" }
func (r *registryD) HealthSettings(runtime.Runtime) *health.Settings     { return &health.DefaultSettings }
func (r *registryD) PreFunc(context.Context, runtime.Runtime) error      { return nil }
func (r *registryD) PostFunc(runtime.Runtime, events.ServiceState) error { return nil }
func (r *registryD) Condition(runtime.Runtime) conditions.Condition      { return nil }
func (r *registryD) DependsOn(runtime.Runtime) []string                  { return nil }

func (r *registryD) HealthFunc(runtime.Runtime) health.Check {
	return func(ctx context.Context) error { return simpleHealthCheck(ctx, "http://127.0.0.1:3172/healthz") }
}

func (r *registryD) Runner(rt runtime.Runtime) (runner.Runner, error) {
	it := func(yield func(fs.StatFS) bool) {
		// TODO: Replace the code below with reads from `runtime.Runtime`.
		for _, root := range []string{"/imagecache", "/var/lib/registry-cache"} {
			if !yield(os.DirFS(root).(fs.StatFS)) {
				return
			}
		}
	}

	return goroutine.NewRunner(rt, "registryd", func(ctx context.Context, r runtime.Runtime, logOutput io.Writer) error {
		return registry.NewService(
			registry.NewMultiPathFS(it),
			logging.ZapLogger(logging.NewLogDestination(
				logOutput,
				zapcore.DebugLevel,
				logging.WithColoredLevels(),
			)),
		).Run(ctx)
	}, runner.WithLoggingManager(rt.Logging())), nil
}
