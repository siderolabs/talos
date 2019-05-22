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
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/init/proto"
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

	stateSubscribers map[StateEvent][]chan<- struct{}

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// NewServiceRunner creates new ServiceRunner around Service instance
func NewServiceRunner(service Service, userData *userdata.UserData) *ServiceRunner {
	ctx, ctxCancel := context.WithCancel(context.Background())

	return &ServiceRunner{
		service:          service,
		userData:         userData,
		id:               service.ID(userData),
		ctx:              ctx,
		ctxCancel:        ctxCancel,
		state:            events.StateInitialized,
		stateSubscribers: make(map[StateEvent][]chan<- struct{}),
	}
}

// UpdateState implements events.Recorder
func (svcrunner *ServiceRunner) UpdateState(newstate events.ServiceState, message string, args ...interface{}) {
	svcrunner.mu.Lock()

	event := events.ServiceEvent{
		Message:   fmt.Sprintf(message, args...),
		State:     newstate,
		Timestamp: time.Now(),
	}

	svcrunner.state = newstate
	svcrunner.events.Push(event)

	log.Printf("service[%s](%s): %s", svcrunner.id, svcrunner.state, event.Message)

	isUp := svcrunner.inStateLocked(StateEventUp)
	isDown := svcrunner.inStateLocked(StateEventDown)
	svcrunner.mu.Unlock()

	if isUp {
		svcrunner.notifyEvent(StateEventUp)
	}
	if isDown {
		svcrunner.notifyEvent(StateEventDown)
	}
}

func (svcrunner *ServiceRunner) healthUpdate(change health.StateChange) {
	svcrunner.mu.Lock()

	// service not running, suppress event
	if svcrunner.state != events.StateRunning {
		svcrunner.mu.Unlock()
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

	isUp := svcrunner.inStateLocked(StateEventUp)
	svcrunner.mu.Unlock()

	if isUp {
		svcrunner.notifyEvent(StateEventUp)
	}
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
// nolint: gocyclo
func (svcrunner *ServiceRunner) Start() {
	condition := svcrunner.service.Condition(svcrunner.userData)
	dependencies := svcrunner.service.DependsOn(svcrunner.userData)
	if len(dependencies) > 0 {
		serviceConditions := make([]conditions.Condition, len(dependencies))
		for i := range dependencies {
			serviceConditions[i] = WaitForService(StateEventUp, dependencies[i])
		}
		serviceDependencies := conditions.WaitForAll(serviceConditions...)
		if condition != nil {
			condition = conditions.WaitForAll(serviceDependencies, condition)
		} else {
			condition = serviceDependencies
		}
	}

	if condition != nil {
		svcrunner.UpdateState(events.StateWaiting, "Waiting for %s", condition)
		err := condition.Wait(svcrunner.ctx)
		if err != nil {
			svcrunner.UpdateState(events.StateFailed, "Condition failed: %v", err)
			return
		}
	}

	svcrunner.UpdateState(events.StatePreparing, "Running pre state")
	if err := svcrunner.service.PreFunc(svcrunner.userData); err != nil {
		svcrunner.UpdateState(events.StateFailed, "Failed to run pre stage: %v", err)
		return
	}

	svcrunner.UpdateState(events.StatePreparing, "Creating service runner")
	runnr, err := svcrunner.service.Runner(svcrunner.userData)
	if err != nil {
		svcrunner.UpdateState(events.StateFailed, "Failed to create runner: %v", err)
		return
	}

	if runnr == nil {
		svcrunner.UpdateState(events.StateSkipped, "Service skipped")
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

// AsProto returns protobuf struct with the state of the service runner
func (svcrunner *ServiceRunner) AsProto() *proto.ServiceInfo {
	svcrunner.mu.Lock()
	defer svcrunner.mu.Unlock()

	return &proto.ServiceInfo{
		Id:     svcrunner.id,
		State:  svcrunner.state.String(),
		Events: svcrunner.events.AsProto(events.MaxEventsToKeep),
		Health: svcrunner.healthState.AsProto(),
	}
}

// Subscribe to a specific event for this service.
//
// Channel `ch` should be buffered or it should have listener attached to it,
// as event might be delivered before Subscribe() returns.
func (svcrunner *ServiceRunner) Subscribe(event StateEvent, ch chan<- struct{}) {
	svcrunner.mu.Lock()

	if svcrunner.inStateLocked(event) {
		svcrunner.mu.Unlock()

		// svcrunner is already in expected state, notify immediately
		select {
		case ch <- struct{}{}:
		default:
		}

		return
	}

	svcrunner.stateSubscribers[event] = append(svcrunner.stateSubscribers[event], ch)
	svcrunner.mu.Unlock()
}

// Unsubscribe cancels subscription established with Subscribe.
func (svcrunner *ServiceRunner) Unsubscribe(event StateEvent, ch chan<- struct{}) {
	svcrunner.mu.Lock()
	defer svcrunner.mu.Unlock()

	channels := svcrunner.stateSubscribers[event]

	for i := 0; i < len(channels); {
		if channels[i] == ch {
			channels[i], channels[len(channels)-1] = channels[len(channels)-1], nil
			channels = channels[:len(channels)-1]
		} else {
			i++
		}
	}

	svcrunner.stateSubscribers[event] = channels
}

func (svcrunner *ServiceRunner) notifyEvent(event StateEvent) {
	svcrunner.mu.Lock()
	channels := append([]chan<- struct{}(nil), svcrunner.stateSubscribers[event]...)
	svcrunner.mu.Unlock()

	for _, ch := range channels {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (svcrunner *ServiceRunner) inStateLocked(event StateEvent) bool {
	switch event {
	case StateEventUp:
		// check if service supports health checks
		_, supportsHealth := svcrunner.service.(HealthcheckedService)
		health := svcrunner.healthState.Get()

		// up when:
		//   a) either skipped
		//   b) or running and healthy (if supports health checks)
		return svcrunner.state == events.StateSkipped || svcrunner.state == events.StateRunning && (!supportsHealth || (health.Healthy != nil && *health.Healthy))
	case StateEventDown:
		// down when in any of the terminal states
		return svcrunner.state == events.StateFailed || svcrunner.state == events.StateFinished || svcrunner.state == events.StateSkipped
	default:
		panic("unsupported event")
	}
}
