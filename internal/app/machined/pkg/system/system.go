// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/conditions"
)

// singleton the system services API interface.
type singleton struct {
	runtime runtime.Runtime

	// State of running services by ID
	state map[string]*ServiceRunner

	// List of running services at the moment.
	//
	// Service might be in any state, but service ID in the map
	// implies ServiceRunner.Start() method is running at the momemnt
	runningMu sync.Mutex
	running   map[string]struct{}

	mu          sync.Mutex
	wg          sync.WaitGroup
	terminating bool
}

var (
	instance *singleton
	once     sync.Once
)

// Services returns the instance of the system services API.
//nolint:revive,golint
func Services(runtime runtime.Runtime) *singleton {
	once.Do(func() {
		instance = &singleton{
			runtime: runtime,
			state:   make(map[string]*ServiceRunner),
			running: make(map[string]struct{}),
		}
	})

	return instance
}

// Load adds service to the list of services managed by the runner.
//
// Load returns service IDs for each of the services.
func (s *singleton) Load(services ...Service) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.terminating {
		return nil
	}

	ids := make([]string, 0, len(services))

	for _, service := range services {
		id := service.ID(s.runtime)
		ids = append(ids, id)

		if _, exists := s.state[id]; exists {
			// service already loaded, ignore
			continue
		}

		svcrunner := NewServiceRunner(service, s.runtime)
		s.state[id] = svcrunner
	}

	return ids
}

// Unload stops the service and removes it from the list of running services.
//
// It is not an error to unload a service which was already removed or stopped.
func (s *singleton) Unload(ctx context.Context, serviceIDs ...string) error {
	s.mu.Lock()
	if s.terminating {
		s.mu.Unlock()

		return nil
	}

	servicesToRemove := []string{}

	for _, id := range serviceIDs {
		if _, exists := s.state[id]; exists {
			servicesToRemove = append(servicesToRemove, id)
		}
	}
	s.mu.Unlock()

	if err := s.Stop(ctx, servicesToRemove...); err != nil {
		return fmt.Errorf("error stopping services %v: %w", servicesToRemove, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	for _, id := range servicesToRemove {
		delete(s.state, id)
		delete(s.running, id) // this fixes an edge case when defer() in Start() doesn't have time to remove stopped service from running
	}

	return nil
}

// Start will invoke the service's Pre, Condition, and Type funcs. If the any
// error occurs in the Pre or Condition invocations, it is up to the caller to
// to restart the service.
func (s *singleton) Start(serviceIDs ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.terminating {
		return nil
	}

	var multiErr *multierror.Error

	for _, id := range serviceIDs {
		svcrunner := s.state[id]
		if svcrunner == nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("service %q not defined", id))
		}

		s.runningMu.Lock()

		_, running := s.running[id]
		if !running {
			s.running[id] = struct{}{}
		}

		s.runningMu.Unlock()

		if running {
			// service already running, skip
			continue
		}

		s.wg.Add(1)

		go func(id string, svcrunner *ServiceRunner) {
			defer func() {
				s.runningMu.Lock()
				delete(s.running, id)
				s.runningMu.Unlock()
			}()
			defer s.wg.Done()

			svcrunner.Start()
		}(id, svcrunner)
	}

	return multiErr.ErrorOrNil()
}

// StartAll starts all the services.
func (s *singleton) StartAll() {
	s.mu.Lock()
	serviceIDs := make([]string, 0, len(s.state))

	for id := range s.state {
		serviceIDs = append(serviceIDs, id)
	}

	s.mu.Unlock()

	//nolint:errcheck
	s.Start(serviceIDs...)
}

// LoadAndStart combines Load and Start into single call.
func (s *singleton) LoadAndStart(services ...Service) {
	err := s.Start(s.Load(services...)...)
	if err != nil {
		// should never happen
		panic(err)
	}
}

// Shutdown all the services.
func (s *singleton) Shutdown(ctx context.Context) {
	s.mu.Lock()
	if s.terminating {
		s.mu.Unlock()

		return
	}

	s.terminating = true

	_ = s.stopServices(ctx, nil, true) //nolint:errcheck
}

// Stop will initiate a shutdown of the specified service.
func (s *singleton) Stop(ctx context.Context, serviceIDs ...string) (err error) {
	if len(serviceIDs) == 0 {
		return
	}

	s.mu.Lock()
	if s.terminating {
		s.mu.Unlock()

		return nil
	}

	return s.stopServices(ctx, serviceIDs, false)
}

// StopWithRevDepenencies will initiate a shutdown of the specified services waiting for reverse dependencies to finish first.
//
// If reverse dependency is not stopped, this method might block waiting on it being stopped forever.
func (s *singleton) StopWithRevDepenencies(ctx context.Context, serviceIDs ...string) (err error) {
	if len(serviceIDs) == 0 {
		return
	}

	s.mu.Lock()
	if s.terminating {
		s.mu.Unlock()

		return nil
	}

	return s.stopServices(ctx, serviceIDs, true)
}

//nolint:gocyclo
func (s *singleton) stopServices(ctx context.Context, services []string, waitForRevDependencies bool) error {
	stateCopy := make(map[string]*ServiceRunner)

	if services == nil {
		for name, svcrunner := range s.state {
			stateCopy[name] = svcrunner
		}
	} else {
		for _, name := range services {
			if _, ok := s.state[name]; !ok {
				continue
			}

			stateCopy[name] = s.state[name]
		}
	}

	s.mu.Unlock()

	// build reverse dependencies
	reverseDependencies := make(map[string][]string)

	if waitForRevDependencies {
		for name, svcrunner := range stateCopy {
			for _, dependency := range svcrunner.service.DependsOn(s.runtime) {
				reverseDependencies[dependency] = append(reverseDependencies[dependency], name)
			}
		}
	}

	// shutdown all the services waiting for rev deps
	var shutdownWg sync.WaitGroup

	// wait max 30 seconds for reverse deps to shut down
	shutdownCtx, shutdownCtxCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCtxCancel()

	stoppedConds := []conditions.Condition{}

	for name, svcrunner := range stateCopy {
		shutdownWg.Add(1)

		stoppedConds = append(stoppedConds, WaitForService(StateEventDown, name))

		go func(svcrunner *ServiceRunner, reverseDeps []string) {
			defer shutdownWg.Done()

			conds := make([]conditions.Condition, len(reverseDeps))

			for i := range reverseDeps {
				conds[i] = WaitForService(StateEventDown, reverseDeps[i])
			}

			allDeps := conditions.WaitForAll(conds...)
			if err := allDeps.Wait(shutdownCtx); err != nil {
				log.Printf("gave up on %s while stopping %q", allDeps, svcrunner.id)
			}

			svcrunner.Shutdown()
		}(svcrunner, reverseDependencies[name])
	}

	shutdownWg.Wait()

	return conditions.WaitForAll(stoppedConds...).Wait(ctx)
}

// List returns snapshot of ServiceRunner instances.
func (s *singleton) List() (result []*ServiceRunner) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result = make([]*ServiceRunner, 0, len(s.state))
	for _, svcrunner := range s.state {
		result = append(result, svcrunner)
	}

	// TODO: results should be sorted properly with topological sort on dependencies
	//       but, we don't have dependencies yet, so sort by service id for now to get stable order
	sort.Slice(result, func(i, j int) bool { return result[i].id < result[j].id })

	return
}

// IsRunning checks service status (started/stopped).
//
// It doesn't check if service runner was started or not, just pure
// check for service status in terms of start/stop.
func (s *singleton) IsRunning(id string) (Service, bool, error) {
	s.mu.Lock()
	runner, exists := s.state[id]
	s.mu.Unlock()

	if !exists {
		return nil, false, fmt.Errorf("service %q not defined", id)
	}

	s.runningMu.Lock()
	_, running := s.running[id]
	s.runningMu.Unlock()

	return runner.service, running, nil
}

// APIStart processes service start request from the API.
func (s *singleton) APIStart(ctx context.Context, id string) error {
	service, running, err := s.IsRunning(id)
	if err != nil {
		return err
	}

	if running {
		// already started, skip
		return nil
	}

	if svc, ok := service.(APIStartableService); ok && svc.APIStartAllowed(s.runtime) {
		return s.Start(id)
	}

	return fmt.Errorf("service %q doesn't support start operation via API", id)
}

// APIStop processes services stop request from the API.
func (s *singleton) APIStop(ctx context.Context, id string) error {
	service, running, err := s.IsRunning(id)
	if err != nil {
		return err
	}

	if !running {
		// already stopped, skip
		return nil
	}

	if svc, ok := service.(APIStoppableService); ok && svc.APIStopAllowed(s.runtime) {
		return s.Stop(ctx, id)
	}

	return fmt.Errorf("service %q doesn't support stop operation via API", id)
}

// APIRestart processes services restart request from the API.
func (s *singleton) APIRestart(ctx context.Context, id string) error {
	service, running, err := s.IsRunning(id)
	if err != nil {
		return err
	}

	if !running {
		// restart for not running service is equivalent to Start()
		return s.APIStart(ctx, id)
	}

	if svc, ok := service.(APIRestartableService); ok && svc.APIRestartAllowed(s.runtime) {
		if err := s.Stop(ctx, id); err != nil {
			return err
		}

		return s.Start(id)
	}

	return fmt.Errorf("service %q doesn't support restart operation via API", id)
}
