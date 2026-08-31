// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system_test

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func (TestService) Volumes(runtime.Runtime) []string {
	return nil
}

type preShutdownTestService struct {
	TestService

	id      string
	started chan struct{}
	running atomic.Bool
	hook    func() error
}

func (service *preShutdownTestService) ID(runtime.Runtime) string {
	return service.id
}

func (service *preShutdownTestService) Runner(r runtime.Runtime) (runner.Runner, error) {
	return goroutine.NewRunner(r, service.id, func(ctx context.Context, _ runtime.Runtime, _ io.Writer) error {
		service.running.Store(true)

		close(service.started)
		defer service.running.Store(false)

		<-ctx.Done()

		return nil
	}), nil
}

func (service *preShutdownTestService) Condition(runtime.Runtime) conditions.Condition {
	return nil
}

func (service *preShutdownTestService) PreShutdownFunc(context.Context, runtime.Runtime) error {
	if !service.running.Load() {
		return errors.New("main service stopped before pre-shutdown hook")
	}

	return service.hook()
}

func TestPreShutdownServices(t *testing.T) {
	ctx := t.Context()
	services := system.NewServices(newRuntime(t))

	var (
		mu    sync.Mutex
		calls []string
	)

	record := func(id string, err error) func() error {
		return func() error {
			mu.Lock()
			defer mu.Unlock()

			calls = append(calls, id)

			return err
		}
	}

	serviceA := &preShutdownTestService{id: "a", started: make(chan struct{}), hook: record("a", errors.New("a failed"))}
	serviceZ := &preShutdownTestService{id: "z", started: make(chan struct{}), hook: record("z", nil)}

	services.Load(serviceZ, serviceA)
	require.NoError(t, services.Start("z", "a"))
	require.NoError(t, system.WaitForServiceWithInstance(services, system.StateEventUp, "z").Wait(ctx))
	require.NoError(t, system.WaitForServiceWithInstance(services, system.StateEventUp, "a").Wait(ctx))

	for _, service := range []*preShutdownTestService{serviceA, serviceZ} {
		select {
		case <-service.started:
		case <-ctx.Done():
			t.Fatal("service did not start before test context expired")
		}
	}

	err := services.PreShutdown(ctx)
	require.ErrorContains(t, err, `service "a" pre-shutdown hook failed: a failed`)
	assert.Equal(t, []string{"a", "z"}, calls)
	assert.True(t, serviceA.running.Load())
	assert.True(t, serviceZ.running.Load())

	require.Error(t, services.PreShutdown(ctx))
	assert.Equal(t, []string{"a", "z", "a", "z"}, calls, "hooks should be retryable")

	require.NoError(t, services.Stop(ctx, "a"))
	assert.Equal(t, []string{"a", "z", "a", "z"}, calls, "service stop must not run node-shutdown hooks")

	require.NoError(t, services.PreShutdown(ctx))
	assert.Equal(t, []string{"a", "z", "a", "z", "z"}, calls, "stopped services must be skipped")

	services.Shutdown(ctx)
}

func TestRestartService(t *testing.T) {
	deadline, ok := t.Deadline()
	if !ok {
		deadline = time.Now().Add(15 * time.Second)
	}

	ctx, cancel := context.WithDeadline(t.Context(), deadline)
	defer cancel()

	services := system.NewServices(newRuntime(t))

	services.Load(TestService{})

	for range 100 {
		require.NoError(t, services.Start("test-service"))

		require.NoError(t, system.WaitForServiceWithInstance(services, system.StateEventUp, "test-service").Wait(ctx))

		require.NoError(t, services.Stop(ctx, "test-service"))
	}
}
