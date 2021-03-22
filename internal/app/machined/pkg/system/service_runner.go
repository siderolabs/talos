// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/events"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/health"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system/runner"
	"github.com/talos-systems/talos/pkg/conditions"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
)

// WaitConditionCheckInterval is time between checking for wait condition
// description changes.
//
// Exposed here for unit-tests to override.
var WaitConditionCheckInterval = time.Second

// ServiceRunner wraps the state of the service (running, stopped, ...).
type ServiceRunner struct {
	mu sync.Mutex

	runtime runtime.Runtime
	service Service
	id      string

	state  events.ServiceState
	events events.ServiceEvents

	healthState health.State

	stateSubscribers map[StateEvent][]chan<- struct{}

	ctxMu     sync.Mutex
	ctx       context.Context
	ctxCancel context.CancelFunc
}

// NewServiceRunner creates new ServiceRunner around Service instance.
func NewServiceRunner(service Service, runtime runtime.Runtime) *ServiceRunner {
	ctx, ctxCancel := context.WithCancel(context.Background())

	return &ServiceRunner{
		service:          service,
		runtime:          runtime,
		id:               service.ID(runtime),
		state:            events.StateInitialized,
		stateSubscribers: make(map[StateEvent][]chan<- struct{}),
		ctx:              ctx,
		ctxCancel:        ctxCancel,
	}
}

// GetState implements events.Recorder.
func (svcrunner *ServiceRunner) GetState() events.ServiceState {
	svcrunner.mu.Lock()
	defer svcrunner.mu.Unlock()

	return svcrunner.state
}

// UpdateState implements events.Recorder.
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
	isFinished := svcrunner.inStateLocked(StateEventFinished)
	svcrunner.mu.Unlock()

	if svcrunner.runtime != nil {
		svcrunner.runtime.Events().Publish(event.AsProto(svcrunner.id))
	}

	if isUp {
		svcrunner.notifyEvent(StateEventUp)
	}

	if isDown {
		svcrunner.notifyEvent(StateEventDown)
	}

	if isFinished {
		svcrunner.notifyEvent(StateEventFinished)
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
		Health:    change.New,
		Timestamp: time.Now(),
	}
	svcrunner.events.Push(event)

	log.Printf("service[%s](%s): %s", svcrunner.id, svcrunner.state, event.Message)

	isUp := svcrunner.inStateLocked(StateEventUp)
	svcrunner.mu.Unlock()

	if isUp {
		svcrunner.notifyEvent(StateEventUp)
	}

	if svcrunner.runtime != nil {
		svcrunner.runtime.Events().Publish(event.AsProto(svcrunner.id))
	}
}

// GetEventHistory returns history of events for this service.
func (svcrunner *ServiceRunner) GetEventHistory(count int) []events.ServiceEvent {
	svcrunner.mu.Lock()
	defer svcrunner.mu.Unlock()

	return svcrunner.events.Get(count)
}

func (svcrunner *ServiceRunner) waitFor(ctx context.Context, condition conditions.Condition) error {
	description := condition.String()
	svcrunner.UpdateState(events.StateWaiting, "Waiting for %s", description)

	errCh := make(chan error)

	go func() {
		errCh <- condition.Wait(ctx)
	}()

	ticker := time.NewTicker(WaitConditionCheckInterval)
	defer ticker.Stop()

	// update state if condition description changes (some conditions are satisfied)
	for {
		select {
		case err := <-errCh:
			return err
		case <-ticker.C:
			newDescription := condition.String()
			if newDescription != description && newDescription != "" {
				description = newDescription
				svcrunner.UpdateState(events.StateWaiting, "Waiting for %s", description)
			}
		}
	}
}

// Start initializes the service and runs it
//
// Start should be run in a goroutine.
//nolint:gocyclo
func (svcrunner *ServiceRunner) Start() {
	defer func() {
		// reset context for the next run
		svcrunner.ctxMu.Lock()
		svcrunner.ctx, svcrunner.ctxCancel = context.WithCancel(context.Background())
		svcrunner.ctxMu.Unlock()
	}()

	svcrunner.ctxMu.Lock()
	ctx := svcrunner.ctx
	svcrunner.ctxMu.Unlock()

	condition := svcrunner.service.Condition(svcrunner.runtime)

	dependencies := svcrunner.service.DependsOn(svcrunner.runtime)
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
		if err := svcrunner.waitFor(ctx, condition); err != nil {
			svcrunner.UpdateState(events.StateFailed, "Condition failed: %v", err)

			return
		}
	}

	svcrunner.UpdateState(events.StatePreparing, "Running pre state")

	if err := svcrunner.service.PreFunc(ctx, svcrunner.runtime); err != nil {
		svcrunner.UpdateState(events.StateFailed, "Failed to run pre stage: %v", err)

		return
	}

	svcrunner.UpdateState(events.StatePreparing, "Creating service runner")

	runnr, err := svcrunner.service.Runner(svcrunner.runtime)
	if err != nil {
		svcrunner.UpdateState(events.StateFailed, "Failed to create runner: %v", err)

		return
	}

	if runnr == nil {
		svcrunner.UpdateState(events.StateSkipped, "Service skipped")

		return
	}

	if err := svcrunner.run(ctx, runnr); err != nil {
		svcrunner.UpdateState(events.StateFailed, "Failed running service: %v", err)
	} else {
		svcrunner.UpdateState(events.StateFinished, "Service finished successfully")
	}

	// PostFunc passes in the state so that we can take actions that depend on the outcome of the run
	state := svcrunner.GetState()

	if err := svcrunner.service.PostFunc(svcrunner.runtime, state); err != nil {
		svcrunner.UpdateState(events.StateFailed, "Failed to run post stage: %v", err)

		return
	}
}

//nolint:gocyclo
func (svcrunner *ServiceRunner) run(ctx context.Context, runnr runner.Runner) error {
	if runnr == nil {
		// special case - run nothing (TODO: we should handle it better, e.g. in PreFunc)
		return nil
	}

	if err := runnr.Open(ctx); err != nil {
		return fmt.Errorf("error opening runner: %w", err)
	}

	//nolint:errcheck
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

			//nolint:errcheck
			health.Run(ctx, healthSvc.HealthSettings(svcrunner.runtime), &svcrunner.healthState, healthSvc.HealthFunc(svcrunner.runtime))
		}()

		notifyCh := make(chan health.StateChange, 2)

		svcrunner.healthState.Subscribe(notifyCh)
		defer svcrunner.healthState.Unsubscribe(notifyCh)

		healthWg.Add(1)

		go func() {
			defer healthWg.Done()

			for {
				select {
				case <-ctx.Done():
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
	case <-ctx.Done():
		err := runnr.Stop()

		<-errCh

		if err != nil {
			return fmt.Errorf("error stopping service: %w", err)
		}
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error running service: %w", err)
		}
	}

	return nil
}

// Shutdown initiates shutdown of the service runner
//
// Shutdown completes when Start() returns.
func (svcrunner *ServiceRunner) Shutdown() {
	svcrunner.ctxMu.Lock()
	defer svcrunner.ctxMu.Unlock()
	svcrunner.ctxCancel()
}

// AsProto returns protobuf struct with the state of the service runner.
func (svcrunner *ServiceRunner) AsProto() *machineapi.ServiceInfo {
	svcrunner.mu.Lock()
	defer svcrunner.mu.Unlock()

	return &machineapi.ServiceInfo{
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
		// up when:
		//   a) either skipped or already finished
		//   b) or running and healthy (if supports health checks)
		switch svcrunner.state { //nolint:exhaustive
		case events.StateSkipped, events.StateFinished:
			return true
		case events.StateRunning:
			// check if service supports health checks
			_, supportsHealth := svcrunner.service.(HealthcheckedService)
			health := svcrunner.healthState.Get()

			return !supportsHealth || (health.Healthy != nil && *health.Healthy)
		default:
			return false
		}
	case StateEventDown:
		// down when in any of the terminal states
		switch svcrunner.state { //nolint:exhaustive
		case events.StateFailed, events.StateFinished, events.StateSkipped:
			return true
		default:
			return false
		}
	case StateEventFinished:
		if svcrunner.state == events.StateFinished {
			return true
		}

		return false
	default:
		panic("unsupported event")
	}
}
