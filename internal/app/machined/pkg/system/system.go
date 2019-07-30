/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package system

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/conditions"
	"github.com/talos-systems/talos/pkg/userdata"
)

type singleton struct {
	UserData *userdata.UserData

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

var instance *singleton
var once sync.Once

// Services returns the instance of the system services API.
// nolint: golint
func Services(data *userdata.UserData) *singleton {
	once.Do(func() {
		instance = &singleton{
			UserData: data,
			state:    make(map[string]*ServiceRunner),
			running:  make(map[string]struct{}),
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
		id := service.ID(s.UserData)
		ids = append(ids, id)

		if _, exists := s.state[id]; exists {
			// service already loaded, ignore
			continue
		}

		svcrunner := NewServiceRunner(service, s.UserData)
		s.state[id] = svcrunner
	}

	return ids
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
			multiErr = multierror.Append(multiErr, errors.Errorf("service %q not defined", id))
		}

		s.runningMu.Lock()
		_, running := s.running[id]
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

			s.runningMu.Lock()
			s.running[id] = struct{}{}
			s.runningMu.Unlock()

			svcrunner.Start()
		}(id, svcrunner)
	}

	return multiErr.ErrorOrNil()
}

// LoadAndStart combines Load and Start into single call.
func (s *singleton) LoadAndStart(services ...Service) {
	err := s.Start(s.Load(services...)...)
	if err != nil {
		// should never happen
		panic(err)
	}
}

// Shutdown all the services
func (s *singleton) Shutdown() {
	s.mu.Lock()
	if s.terminating {
		s.mu.Unlock()
		return
	}
	stateCopy := make(map[string]*ServiceRunner)
	s.terminating = true
	for name, svcrunner := range s.state {
		stateCopy[name] = svcrunner
	}
	s.mu.Unlock()

	// build reverse dependencies
	reverseDependencies := make(map[string][]string)

	for name, svcrunner := range stateCopy {
		for _, dependency := range svcrunner.service.DependsOn(s.UserData) {
			reverseDependencies[dependency] = append(reverseDependencies[dependency], name)
		}
	}

	// shutdown all the services waiting for rev deps
	var shutdownWg sync.WaitGroup

	// wait max 30 seconds for reverse deps to shut down
	shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCtxCancel()

	for name, svcrunner := range stateCopy {
		shutdownWg.Add(1)
		go func(svcrunner *ServiceRunner, reverseDeps []string) {
			defer shutdownWg.Done()
			conds := make([]conditions.Condition, len(reverseDeps))
			for i := range reverseDeps {
				conds[i] = WaitForService(StateEventDown, reverseDeps[i])
			}

			// nolint: errcheck
			_ = conditions.WaitForAll(conds...).Wait(shutdownCtx)

			svcrunner.Shutdown()
		}(svcrunner, reverseDependencies[name])
	}
	shutdownWg.Wait()

	s.wg.Wait()
}

// List returns snapshot of ServiceRunner instances
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

// Stop will initiate a shutdown of the specified service.
func (s *singleton) Stop(ctx context.Context, serviceIDs ...string) (err error) {
	if len(serviceIDs) == 0 {
		return
	}

	s.mu.Lock()
	if s.terminating {
		s.mu.Unlock()
		return
	}

	// Copy current service state
	stateCopy := make(map[string]*ServiceRunner)
	for _, id := range serviceIDs {
		if _, ok := s.state[id]; !ok {
			return fmt.Errorf("service not found: %s", id)
		}
		stateCopy[id] = s.state[id]
	}
	s.mu.Unlock()

	conds := make([]conditions.Condition, 0, len(stateCopy))

	// Initiate a shutdown on the specific service
	for id, svcrunner := range stateCopy {
		svcrunner.Shutdown()
		conds = append(conds, WaitForService(StateEventDown, id))
	}

	// Wait for service to actually shut down
	return conditions.WaitForAll(conds...).Wait(ctx)
}
