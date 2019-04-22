/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package system

import (
	"log"
	"sync"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/pkg/userdata"
)

type singleton struct {
	UserData *userdata.UserData
}

var instance *singleton
var once sync.Once

// Service is an interface describing a system service.
type Service interface {
	// ID is the service id.
	ID(*userdata.UserData) string
	// PreFunc is invoked before a runner is created
	PreFunc(*userdata.UserData) error
	// Runner creates runner for the service
	Runner(*userdata.UserData) (runner.Runner, error)
	// PostFunc is invoked after a runner is closed.
	PostFunc(*userdata.UserData) error
	// ConditionFunc describes the conditions under which a service should
	// start.
	ConditionFunc(*userdata.UserData) conditions.ConditionFunc
}

// Services returns the instance of the system services API.
// TODO(andrewrynhard): This should be a gRPC based API availale on a local
// unix socket.
// nolint: golint
func Services(data *userdata.UserData) *singleton {
	once.Do(func() {
		instance = &singleton{UserData: data}
	})
	return instance
}

func runService(runnr runner.Runner) error {
	if runnr == nil {
		// special case - run nothing (TODO: we should handle it better, e.g. in PreFunc)
		return nil
	}

	if err := runnr.Open(); err != nil {
		return errors.Wrap(err, "error opening runner")
	}

	// nolint: errcheck
	defer runnr.Close()

	if err := runnr.Run(); err != nil {
		return errors.Wrap(err, "error running service")
	}

	return nil
}

// Start will invoke the service's Pre, Condition, and Type funcs. If the any
// error occurs in the Pre or Condition invocations, it is up to the caller to
// to restart the service.
func (s *singleton) Start(services ...Service) {
	for _, service := range services {
		go func(service Service) {
			id := service.ID(s.UserData)
			if err := service.PreFunc(s.UserData); err != nil {
				log.Printf("failed to run pre stage of service %q: %v", id, err)
				return
			}

			_, err := service.ConditionFunc(s.UserData)()
			if err != nil {
				log.Printf("service %q condition failed: %v", id, err)
				return
			}

			log.Printf("starting service %q", id)
			runnr, err := service.Runner(s.UserData)
			if err != nil {
				log.Printf("failed to create runner for service %q: %v", id, err)
				return
			}

			if err := runService(runnr); err != nil {
				log.Printf("failed running service %q: %v", id, err)
			}

			if err := service.PostFunc(s.UserData); err != nil {
				log.Printf("failed to run post stage of service %q: %v", id, err)
				return
			}
		}(service)
	}
}
