// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"context"
	"fmt"

	"github.com/talos-systems/talos/pkg/conditions"
)

// StateEvent is a service event (e.g. 'up', 'down').
type StateEvent string

// Service event list.
const (
	StateEventUp       = StateEvent("up")
	StateEventDown     = StateEvent("down")
	StateEventFinished = StateEvent("finished")
)

type serviceCondition struct {
	event   StateEvent
	service string
}

func (sc *serviceCondition) Wait(ctx context.Context) error {
	instance.mu.Lock()
	svcrunner := instance.state[sc.service]
	instance.mu.Unlock()

	if svcrunner == nil {
		return fmt.Errorf("service %q is not registered", sc.service)
	}

	notifyCh := make(chan struct{}, 1)

	svcrunner.Subscribe(sc.event, notifyCh)
	defer svcrunner.Unsubscribe(sc.event, notifyCh)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-notifyCh:
		return nil
	}
}

func (sc *serviceCondition) String() string {
	return fmt.Sprintf("service %q to be %q", sc.service, string(sc.event))
}

// WaitForService waits for service to reach some state event.
func WaitForService(event StateEvent, service string) conditions.Condition {
	return &serviceCondition{event, service}
}
