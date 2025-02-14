// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/goroutine"
	"github.com/siderolabs/talos/pkg/conditions"
)

type TestCondition struct{}

func (TestCondition) String() string {
	return "test-condition"
}

func (TestCondition) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
		return nil
	}
}

type TestService struct{}

func (TestService) ID(runtime.Runtime) string {
	return "test-service"
}

func (TestService) PreFunc(ctx context.Context, r runtime.Runtime) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (TestService) Runner(r runtime.Runtime) (runner.Runner, error) {
	return goroutine.NewRunner(r, "test-service", func(ctx context.Context, r runtime.Runtime, logOutput io.Writer) error {
		<-ctx.Done()

		return nil
	}), nil
}

func (TestService) PostFunc(runtime.Runtime, events.ServiceState) error {
	return nil
}

func (TestService) Condition(runtime.Runtime) conditions.Condition {
	return TestCondition{}
}

func (TestService) DependsOn(runtime.Runtime) []string {
	return nil
}

func (TestService) Volumes() []string {
	return nil
}

func TestRestartService(t *testing.T) {
	deadline, ok := t.Deadline()
	if !ok {
		deadline = time.Now().Add(15 * time.Second)
	}

	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	defer cancel()

	services := system.NewServices(nil)

	services.Load(TestService{})

	for range 100 {
		require.NoError(t, services.Start("test-service"))

		require.NoError(t, system.WaitForServiceWithInstance(services, system.StateEventUp, "test-service").Wait(ctx))

		require.NoError(t, services.Stop(ctx, "test-service"))
	}
}
