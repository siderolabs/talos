// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	stdmaps "maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/maps"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
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
	running   map[string]*serviceLaunch

	mu sync.Mutex
	wg sync.WaitGroup

	// denyNewServices is set on DenyNewServices, and it rejects any further Load/Start calls, used on Talos shutdown/reboot paths.
	denyNewServices bool
	// terminating is set on Shutdown, and it rejects any further Load/Start/Stop calls, and allows Shutdown to proceed without deadlock.
	terminating bool
}

// serviceLaunch identifies one invocation, including its terminal state publication.
// Cancellation and completion belong to this invocation, never to a later restart.
type serviceLaunch struct {
	cancel context.CancelFunc
	done   chan struct{}
}

var (
	instance *singleton
	once     sync.Once
)

func newServices(runtime runtime.Runtime) *singleton {
	return &singleton{
		runtime: runtime,
		state:   map[string]*ServiceRunner{},
		running: map[string]*serviceLaunch{},
	}
}

// Services returns the instance of the system services API.
//
//nolint:revive
func Services(runtime runtime.Runtime) *singleton {
	once.Do(func() {
		instance = newServices(runtime)
	})

	return instance
}

// Load adds service to the list of services managed by the runner.
//
// Load returns service IDs for each of the services.
func (s *singleton) Load(services ...Service) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.terminating || s.denyNewServices {
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

		svcrunner := NewServiceRunner(s, service, s.runtime)
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

	servicesToRemove := make([]string, 0, len(serviceIDs))
	original := make(map[string]*ServiceRunner, len(serviceIDs))

	for _, id := range serviceIDs {
		if service, exists := s.state[id]; exists {
			servicesToRemove = append(servicesToRemove, id)
			original[id] = service
		}
	}

	launches := s.captureLaunches(servicesToRemove)
	s.mu.Unlock()

	for _, launch := range launches {
		launch.cancel()
	}

	if err := waitForLaunches(ctx, launches); err != nil {
		return fmt.Errorf("error stopping services %v: %w", servicesToRemove, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	for _, id := range servicesToRemove {
		if s.state[id] != original[id] {
			continue
		}

		if s.running[id] != nil {
			return fmt.Errorf("service %q restarted while unloading", id)
		}

		delete(s.state, id)
	}

	return nil
}

// Start will invoke the service's Pre, Condition, and Type funcs. If any
// error occurs in the Pre or Condition invocations, it is up to the caller to
// restart the service.
func (s *singleton) Start(serviceIDs ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.terminating || s.denyNewServices {
		return nil
	}

	var multiErr *multierror.Error

	for _, id := range serviceIDs {
		svcrunner := s.state[id]
		if svcrunner == nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("service %q not defined", id))

			continue
		}

		s.runningMu.Lock()

		if _, running := s.running[id]; running {
			s.runningMu.Unlock()

			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		launch := &serviceLaunch{cancel: cancel, done: make(chan struct{})}
		s.running[id] = launch
		s.runningMu.Unlock()

		runNotify := make(chan struct{})

		s.wg.Add(1)

		go func(id string, svcrunner *ServiceRunner) {
			defer s.wg.Done()
			defer cancel()
			defer func() {
				s.runningMu.Lock()
				delete(s.running, id)
				close(launch.done)
				s.runningMu.Unlock()
			}()

			err := svcrunner.runWithContext(ctx, runNotify)

			switch {
			case err == nil:
				svcrunner.UpdateState(context.Background(), events.StateFinished, "Service finished successfully")
			case errors.Is(err, ErrSkip):
				svcrunner.UpdateState(context.Background(), events.StateSkipped, "Service skipped")
			default:
				msg := err.Error()
				if len(msg) > 0 {
					msg = strings.ToUpper(msg[:1]) + msg[1:]
				}

				svcrunner.UpdateState(context.Background(), events.StateFailed, "%s", msg)
			}
		}(id, svcrunner)

		// wait for svcrunner.Run to enter the running phase, and then return
		<-runNotify
	}

	return multiErr.ErrorOrNil()
}

// StartAll starts all the services.
func (s *singleton) StartAll() {
	s.mu.Lock()
	serviceIDs := maps.Keys(s.state)
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

// PreShutdown runs node shutdown hooks for all running services.
func (s *singleton) PreShutdown(ctx context.Context) error {
	var multiErr *multierror.Error

	for _, svcrunner := range s.List() {
		if svcrunner.GetState() != events.StateRunning {
			continue
		}

		service, ok := svcrunner.service.(PreShutdownService)
		if !ok {
			continue
		}

		if err := service.PreShutdownFunc(ctx, s.runtime); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("service %q pre-shutdown hook failed: %w", svcrunner.id, err))
		}
	}

	return multiErr.ErrorOrNil()
}

// Stop will initiate a shutdown of the specified service.
func (s *singleton) Stop(ctx context.Context, serviceIDs ...string) (err error) {
	if len(serviceIDs) == 0 {
		return err
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
// If reverse dependency is not stopped, this method might block waiting on it being stopped for up to 30 seconds.
func (s *singleton) StopWithRevDepenencies(ctx context.Context, serviceIDs ...string) (err error) {
	if len(serviceIDs) == 0 {
		return err
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
	servicesToStop := map[string]*ServiceRunner{}

	if services == nil {
		stdmaps.Copy(servicesToStop, s.state)
	} else {
		for _, name := range services {
			if _, ok := s.state[name]; !ok {
				continue
			}

			servicesToStop[name] = s.state[name]
		}
	}

	// build reverse dependencies, and expand the list of services to stop
	// with services which depend on the one being stopped
	reverseDependencies := map[string][]string{}

	if waitForRevDependencies {
		// expand the list of services to stop with the list of services which depend
		// on the ones being stopped
		// the loop is run as long as more dependencies are added to the list
		for {
			expanded := false

			for name, svcrunner := range s.state {
				if _, scheduledToStop := servicesToStop[name]; scheduledToStop {
					continue
				}

				dependencies := svcrunner.service.DependsOn(s.runtime)

				shouldStopService := false

				for _, dependency := range dependencies {
					for scheduledService := range servicesToStop {
						if scheduledService == dependency {
							shouldStopService = true

							break
						}
					}

					if shouldStopService {
						break
					}
				}

				if shouldStopService {
					servicesToStop[name] = svcrunner
					expanded = true
				}
			}

			if !expanded {
				break
			}
		}

		// build a list of dependencies to wait for before stopping each of the services
		for name, svcrunner := range servicesToStop {
			for _, dependency := range svcrunner.service.DependsOn(s.runtime) {
				reverseDependencies[dependency] = append(reverseDependencies[dependency], name)
			}
		}
	}

	launches := s.captureLaunches(maps.Keys(servicesToStop))
	s.mu.Unlock()

	// shutdown all the services waiting for rev deps
	var shutdownWg sync.WaitGroup

	// wait max 30 seconds for reverse deps to shut down
	shutdownCtx, shutdownCtxCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCtxCancel()

	for name, launch := range launches {
		shutdownWg.Go(func() {
			for _, dependency := range reverseDependencies[name] {
				if err := waitForLaunch(shutdownCtx, launches[dependency]); err != nil {
					log.Printf("gave up on %q while stopping %q", dependency, name)
				}
			}

			launch.cancel()
		})
	}

	shutdownWg.Wait()

	return waitForLaunches(ctx, launches)
}

// captureLaunches is called while s.mu prevents concurrent registration and start.
func (s *singleton) captureLaunches(ids []string) map[string]*serviceLaunch {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	launches := make(map[string]*serviceLaunch, len(ids))
	for _, id := range ids {
		if launch := s.running[id]; launch != nil {
			launches[id] = launch
		}
	}

	return launches
}

func waitForLaunches(ctx context.Context, launches map[string]*serviceLaunch) error {
	for _, launch := range launches {
		if err := waitForLaunch(ctx, launch); err != nil {
			return err
		}
	}

	return nil
}

func waitForLaunch(ctx context.Context, launch *serviceLaunch) error {
	if launch == nil {
		return nil
	}

	select {
	case <-launch.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// List returns snapshot of ServiceRunner instances.
func (s *singleton) List() (result []*ServiceRunner) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result = maps.Values(s.state)

	// TODO: results should be sorted properly with topological sort on dependencies
	//       but, we don't have dependencies yet, so sort by service id for now to get stable order
	slices.SortFunc(result, func(a, b *ServiceRunner) int { return cmp.Compare(a.id, b.id) })

	return result
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

// DenyNewServices sets the denyNewServices flag, which prevents any new services from being loaded or started.
func (s *singleton) DenyNewServices() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.denyNewServices = true
}
