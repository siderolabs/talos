// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system_test

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/pkg/conditions"
)

type MockService struct {
	name         string
	preError     error
	runnerError  error
	nilRunner    bool
	runner       runner.Runner
	condition    conditions.Condition
	postError    error
	dependencies []string
}

func (m *MockService) ID(runtime.Runtime) string {
	if m.name != "" {
		return m.name
	}

	return "MockRunner"
}

func (m *MockService) PreFunc(context.Context, runtime.Runtime) error {
	return m.preError
}

func (m *MockService) Runner(runtime.Runtime) (runner.Runner, error) {
	if m.runner != nil {
		return m.runner, m.runnerError
	}

	if m.nilRunner {
		return nil, nil
	}

	return &MockRunner{exitCh: make(chan error)}, m.runnerError
}

func (m *MockService) PostFunc(runtime.Runtime, events.ServiceState) error {
	return m.postError
}

func (m *MockService) Condition(runtime.Runtime) conditions.Condition {
	return m.condition
}

func (m *MockService) DependsOn(runtime.Runtime) []string {
	return m.dependencies
}

type MockHealthcheckedService struct {
	MockService

	notHealthy uint32
}

func (m *MockHealthcheckedService) SetHealthy(healthy bool) {
	if healthy {
		atomic.StoreUint32(&m.notHealthy, 0)
	} else {
		atomic.StoreUint32(&m.notHealthy, 1)
	}
}

func (m *MockHealthcheckedService) HealthFunc(runtime.Runtime) health.Check {
	return func(context.Context) error {
		if atomic.LoadUint32(&m.notHealthy) == 0 {
			return nil
		}

		return errors.New("not healthy")
	}
}

func (m *MockHealthcheckedService) HealthSettings(runtime.Runtime) *health.Settings {
	return &health.Settings{
		InitialDelay: time.Millisecond,
		Timeout:      time.Second,
		Period:       time.Millisecond,
	}
}

type MockRunner struct {
	exitCh chan error
}

func (m *MockRunner) Open(ctx context.Context) error {
	return nil
}

func (m *MockRunner) Close() error {
	return nil
}

func (m *MockRunner) Run(eventSink events.Recorder) error {
	eventSink(events.StateRunning, "Running")

	return <-m.exitCh
}

func (m *MockRunner) Stop() error {
	close(m.exitCh)

	return nil
}

func (m *MockRunner) String() string {
	return "MockRunner()"
}

type MockCondition struct {
	done chan struct{}
	desc string
}

func NewMockCondition(desc string) *MockCondition {
	return &MockCondition{
		done: make(chan struct{}),
		desc: desc,
	}
}

func (m *MockCondition) String() string {
	return m.desc
}

func (m *MockCondition) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.done:
		return nil
	}
}
