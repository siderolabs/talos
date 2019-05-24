/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package system

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/pkg/userdata"
)

type singleton struct {
	UserData *userdata.UserData

	// State of running services by ID
	State map[string]*ServiceRunner

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
			State:    make(map[string]*ServiceRunner),
		}
	})
	return instance
}

// Start will invoke the service's Pre, Condition, and Type funcs. If the any
// error occurs in the Pre or Condition invocations, it is up to the caller to
// to restart the service.
func (s *singleton) Start(services ...Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.terminating {
		return
	}

	for _, service := range services {
		id := service.ID(s.UserData)

		if _, exists := s.State[id]; exists {
			// service already started?
			// TODO: it might be nice to handle case when service
			//       should be restarted (e.g. kubeadm after reset)
			continue
		}

		svcrunner := NewServiceRunner(service, s.UserData)
		s.State[id] = svcrunner

		s.wg.Add(1)
		go func(svcrunner *ServiceRunner) {
			defer s.wg.Done()

			svcrunner.Start()
		}(svcrunner)
	}
}

// Shutdown all the services
func (s *singleton) Shutdown() {
	s.mu.Lock()
	if s.terminating {
		s.mu.Unlock()
		return
	}
	s.terminating = true
	stateCopy := make(map[string]*ServiceRunner)
	for name, svcrunner := range s.State {
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

	result = make([]*ServiceRunner, 0, len(s.State))
	for _, svcrunner := range s.State {
		result = append(result, svcrunner)
	}

	// TODO: results should be sorted properly with topological sort on dependencies
	//       but, we don't have dependencies yet, so sort by service id for now to get stable order
	sort.Slice(result, func(i, j int) bool { return result[i].id < result[j].id })

	return
}
