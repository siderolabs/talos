// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package system

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/conditions"
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
	mu              sync.Mutex
	waitingRegister bool
	instance        *singleton

	events  []StateEvent
	service string
}

func (sc *serviceCondition) Wait(ctx context.Context) error {
	sc.instance.mu.Lock()
	svcrunner := sc.instance.state[sc.service]
	sc.instance.mu.Unlock()

	if svcrunner == nil {
		return sc.waitRegister(ctx)
	}

	return sc.waitEvent(ctx, svcrunner)
}

func (sc *serviceCondition) waitEvent(ctx context.Context, svcrunner *ServiceRunner) error {
	notifyCh := make(chan struct{}, 1)

	for _, ev := range sc.events {
		svcrunner.Subscribe(ev, notifyCh)
		defer svcrunner.Unsubscribe(ev, notifyCh)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-notifyCh:
		return nil
	}
}

func (sc *serviceCondition) waitRegister(ctx context.Context) error {
	sc.mu.Lock()
	sc.waitingRegister = true
	sc.mu.Unlock()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var svcrunner *ServiceRunner

	for {
		sc.instance.mu.Lock()
		svcrunner = sc.instance.state[sc.service]
		sc.instance.mu.Unlock()

		if svcrunner != nil {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}

	sc.mu.Lock()
	sc.waitingRegister = false
	sc.mu.Unlock()

	return sc.waitEvent(ctx, svcrunner)
}

func (sc *serviceCondition) String() string {
	sc.mu.Lock()
	waitingRegister := sc.waitingRegister
	sc.mu.Unlock()

	if waitingRegister {
		return fmt.Sprintf("service %q to be registered", sc.service)
	}

	if len(sc.events) == 1 {
		return fmt.Sprintf("service %q to be %q", sc.service, string(sc.events[0]))
	}

	return fmt.Sprintf("service %q to be %s", sc.service, strings.Join(xslices.Map(sc.events, func(e StateEvent) string { return string(e) }), "/"))
}

// WaitForService waits for service to reach some state event.
func WaitForService(event StateEvent, service string) conditions.Condition {
	return waitForService(instance, []StateEvent{event}, service)
}

// WaitForServiceAnyEvent waits for service to reach some state event (one of).
func WaitForServiceAnyEvent(events []StateEvent, service string) conditions.Condition {
	return waitForService(instance, events, service)
}

func waitForService(instance *singleton, events []StateEvent, service string) conditions.Condition {
	return &serviceCondition{
		instance: instance,
		events:   events,
		service:  service,
	}
}
