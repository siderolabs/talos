/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package system_test

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/pkg/userdata"
)

type MockService struct {
	name        string
	preError    error
	runnerError error
	runner      runner.Runner
	condition   conditions.ConditionFunc
	postError   error
}

func (m *MockService) ID(*userdata.UserData) string {
	if m.name != "" {
		return m.name
	}
	return "MockRunner"
}
func (m *MockService) PreFunc(*userdata.UserData) error {
	return m.preError
}

func (m *MockService) Runner(*userdata.UserData) (runner.Runner, error) {
	if m.runner != nil {
		return m.runner, m.runnerError
	}

	return &MockRunner{exitCh: make(chan error)}, m.runnerError
}

func (m *MockService) PostFunc(*userdata.UserData) error {
	return m.postError
}

func (m *MockService) ConditionFunc(*userdata.UserData) conditions.ConditionFunc {
	if m.condition != nil {
		return m.condition
	}

	return conditions.None()
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

func (m *MockHealthcheckedService) HealthFunc(*userdata.UserData) health.Check {
	return func(context.Context) error {
		if atomic.LoadUint32(&m.notHealthy) == 0 {
			return nil
		}

		return errors.New("not healthy")
	}
}

func (m *MockHealthcheckedService) HealthSettings(*userdata.UserData) *health.Settings {
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
