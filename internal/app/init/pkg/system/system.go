/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package system

import (
	"log"
	"sync"

	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
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
	// PreFunc is invoked before a command is executed.
	PreFunc(*userdata.UserData) error
	// Start
	Start(*userdata.UserData) error
	// PostFunc is invoked after a command is executed.
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
			if err := service.Start(s.UserData); err != nil {
				log.Printf("failed to start service %q: %v", id, err)
				return
			}

			if err := service.PostFunc(s.UserData); err != nil {
				log.Printf("failed to run post stage of service %q: %v", id, err)
				return
			}
		}(service)
	}
}
