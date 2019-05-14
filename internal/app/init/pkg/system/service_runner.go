/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package system

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/pkg/userdata"
)

// ServiceRunner wraps the state of the service (running, stopped, ...)
type ServiceRunner struct {
	mu sync.Mutex

	userData *userdata.UserData
	service  Service
	id       string

	state  events.ServiceState
	events events.ServiceEvents

	healthState health.State

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// NewServiceRunner creates new ServiceRunner around Service instance
func NewServiceRunner(service Service, userData *userdata.UserData) *ServiceRunner {
	ctx, ctxCancel := context.WithCancel(context.Background())

	return &ServiceRunner{
		service:   service,
		userData:  userData,
		id:        service.ID(userData),
		ctx:       ctx,
		ctxCancel: ctxCancel,
		state:     events.StateInitialized,
	}
}

// UpdateState implements events.Recorder
func (svcrunner *ServiceRunner) UpdateState(newstate events.ServiceState, message string, args ...interface{}) {
	svcrunner.mu.Lock()
	defer svcrunner.mu.Unlock()

	event := events.ServiceEvent{
		Message:   fmt.Sprintf(message, args...),
		State:     newstate,
		Timestamp: time.Now(),
	}

	svcrunner.state = newstate
	svcrunner.events.Push(event)

	log.Printf("service[%s](%s): %s", svcrunner.id, svcrunner.state, event.Message)
}

func (svcrunner *ServiceRunner) healthUpdate(change health.StateChange) {
	svcrunner.mu.Lock()
	defer svcrunner.mu.Unlock()

	// service not running, suppress event
	if svcrunner.state != events.StateRunning {
		return
	}

	var message string
	if *change.New.Healthy {
		message = "Health check successful"
	} else {
		message = fmt.Sprintf("Health check failed: %s", change.New.LastMessage)
	}

	event := events.ServiceEvent{
		Message:   message,
		State:     svcrunner.state,
		Timestamp: time.Now(),
	}
	svcrunner.events.Push(event)

	log.Printf("service[%s](%s): %s", svcrunner.id, svcrunner.state, event.Message)
}

// GetEventHistory returns history of events for this service
func (svcrunner *ServiceRunner) GetEventHistory(count int) []events.ServiceEvent {
	svcrunner.mu.Lock()
	defer svcrunner.mu.Unlock()

	return svcrunner.events.Get(count)
}

// Start initializes the service and runs it
//
// Start should be run in a goroutine.
func (svcrunner *ServiceRunner) Start() {
	svcrunner.UpdateState(events.StatePreparing, "Running pre state")
	if err := svcrunner.service.PreFunc(svcrunner.userData); err != nil {
		svcrunner.UpdateState(events.StateFailed, "Failed to run pre stage: %v", err)
		return
	}

	svcrunner.UpdateState(events.StateWaiting, "Waiting for conditions")
	_, err := svcrunner.service.ConditionFunc(svcrunner.userData)(svcrunner.ctx)
	if err != nil {
		svcrunner.UpdateState(events.StateFailed, "Condition failed: %v", err)
		return
	}

	svcrunner.UpdateState(events.StatePreparing, "Creating service runner")
	runnr, err := svcrunner.service.Runner(svcrunner.userData)
	if err != nil {
		svcrunner.UpdateState(events.StateFailed, "Failed to create runner: %v", err)
		return
	}

	if err := svcrunner.run(runnr); err != nil {
		svcrunner.UpdateState(events.StateFailed, "Failed running service: %v", err)
	} else {
		svcrunner.UpdateState(events.StateFinished, "Service finished successfully")
	}

	if err := svcrunner.service.PostFunc(svcrunner.userData); err != nil {
		svcrunner.UpdateState(events.StateFailed, "Failed to run post stage: %v", err)
		return
	}
}

// nolint: gocyclo
func (svcrunner *ServiceRunner) run(runnr runner.Runner) error {
	if runnr == nil {
		// special case - run nothing (TODO: we should handle it better, e.g. in PreFunc)
		return nil
	}

	if err := runnr.Open(svcrunner.ctx); err != nil {
		return errors.Wrap(err, "error opening runner")
	}

	// nolint: errcheck
	defer runnr.Close()

	errCh := make(chan error)

	go func() {
		errCh <- runnr.Run(svcrunner.UpdateState)
	}()

	if healthSvc, ok := svcrunner.service.(HealthcheckedService); ok {
		var healthWg sync.WaitGroup
		defer healthWg.Wait()

		healthWg.Add(1)
		go func() {
			defer healthWg.Done()

			// nolint: errcheck
			health.Run(svcrunner.ctx, healthSvc.HealthSettings(svcrunner.userData), &svcrunner.healthState, healthSvc.HealthFunc(svcrunner.userData))
		}()

		notifyCh := make(chan health.StateChange, 2)
		svcrunner.healthState.Subscribe(notifyCh)
		defer svcrunner.healthState.Unsubscribe(notifyCh)

		healthWg.Add(1)
		go func() {
			defer healthWg.Done()

			for {
				select {
				case <-svcrunner.ctx.Done():
					return
				case change := <-notifyCh:
					svcrunner.healthUpdate(change)
				}
			}
		}()

	}

	// when service run finishes, cancel context, this is important if service
	// terminates on its own before being terminated by Stop()
	defer svcrunner.ctxCancel()

	select {
	case <-svcrunner.ctx.Done():
		err := runnr.Stop()
		<-errCh
		if err != nil {
			return errors.Wrap(err, "error stopping service")
		}
	case err := <-errCh:
		if err != nil {
			return errors.Wrap(err, "error running service")
		}
	}

	return nil
}

// Shutdown initiates shutdown of the service runner
//
// Shutdown completes when Start() returns
func (svcrunner *ServiceRunner) Shutdown() {
	svcrunner.ctxCancel()
}
