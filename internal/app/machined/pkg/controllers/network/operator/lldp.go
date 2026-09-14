// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/siderolabs/gen/channel"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/lldp"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// lldpRetryInterval bounds how often a failing socket is reopened.
const lldpRetryInterval = 5 * time.Second

type lldpCachedNeighbor struct {
	spec network.LLDPNeighborSpec
	// frame is the advertisement this neighbor was decoded from, kept to recognize
	// a re-advertisement of unchanged information.
	frame     []byte
	ttl       time.Duration
	expiresAt time.Time
}

// LLDP implements the LLDP network operator.
//
// It owns the link's listener and the cache of neighbors observed on it: neighbors are aged out on
// their advertised TTL and withdrawn immediately on a zero-TTL advertisement.
type LLDP struct {
	logger  *zap.Logger
	factory lldp.ListenerFactory

	linkName  string
	linkIndex uint32

	mu        sync.Mutex
	neighbors map[lldp.NeighborKey]lldpCachedNeighbor
}

// NewLLDP creates the LLDP operator listening on one device instance. A nil factory uses packet
// sockets. The link index pins the operator to that instance, so a replaced device is a new
// operator rather than a socket bound to an index which no longer exists.
func NewLLDP(logger *zap.Logger, linkName string, linkIndex uint32, factory lldp.ListenerFactory) *LLDP {
	if factory == nil {
		factory = lldp.NewListener
	}

	return &LLDP{
		logger:    logger,
		factory:   factory,
		linkName:  linkName,
		linkIndex: linkIndex,
		neighbors: map[lldp.NeighborKey]lldpCachedNeighbor{},
	}
}

// Prefix returns unique operator prefix which gets prepended to each spec.
func (o *LLDP) Prefix() string {
	return fmt.Sprintf("lldp/%s", o.linkName)
}

// Run the operator loop.
//
// The operator only returns when it is stopped: a link which is down or gone stops the operator via
// the operator spec, and any other socket failure is retried.
func (o *LLDP) Run(ctx context.Context, notifyCh chan<- struct{}) {
	// A stopped operator cannot expose stale neighbors: the controller reads the specs once more
	// while tearing the operator's outputs down.
	defer o.forget()

	for ctx.Err() == nil {
		listener, err := o.factory(o.linkIndex)
		if err == nil {
			err = o.receive(ctx, listener, notifyCh)
		}

		if ctx.Err() != nil {
			return
		}

		o.logger.Warn("LLDP listener unavailable, retrying", zap.String("link", o.linkName), zap.Error(err))

		o.wait(ctx, notifyCh, time.Now().Add(lldpRetryInterval))
	}
}

// receive reads until the socket or the context fails. The read deadline is the next expiry, so a
// single goroutine both receives and ages out neighbors.
func (o *LLDP) receive(ctx context.Context, listener lldp.Listener, notifyCh chan<- struct{}) error {
	// A read waiting for the next expiry is minutes away from returning, so
	// shutdown closes the socket instead of waiting for the deadline.
	stop := context.AfterFunc(ctx, func() { listener.Close() }) //nolint:errcheck

	defer func() {
		stop()
		listener.Close() //nolint:errcheck
	}()

	for ctx.Err() == nil {
		if err := listener.SetReadDeadline(o.expire(ctx, notifyCh)); err != nil {
			return err
		}

		frame, err := listener.ReadFrame()
		if err != nil {
			if errors.Is(err, os.ErrDeadlineExceeded) {
				continue
			}

			return err
		}

		// A neighbor re-advertises the same information every tx interval, so the common frame
		// carries nothing new: extend its lifetime without decoding it or waking the controller.
		if o.refresh(frame) {
			continue
		}

		neighbor, err := lldp.DecodeFrame(frame)
		if err != nil {
			o.logger.Debug("dropping invalid LLDP frame", zap.Error(err), zap.String("link", o.linkName))

			continue
		}

		ttl := time.Duration(neighbor.TTL) * time.Second

		o.mu.Lock()

		if neighbor.TTL == 0 {
			delete(o.neighbors, neighbor.Key)
		} else {
			o.neighbors[neighbor.Key] = lldpCachedNeighbor{
				spec:      neighbor.Spec,
				frame:     bytes.Clone(frame),
				ttl:       ttl,
				expiresAt: time.Now().Add(ttl),
			}
		}

		o.mu.Unlock()

		if !channel.SendWithContext(ctx, notifyCh, struct{}{}) {
			return ctx.Err()
		}
	}

	return ctx.Err()
}

// refresh extends the lifetime of the neighbor whose cached advertisement is byte-identical to
// frame, reporting whether one matched. Nothing published changes, so the controller is not woken.
func (o *LLDP) refresh(frame []byte) bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	for key, neighbor := range o.neighbors {
		if bytes.Equal(neighbor.frame, frame) {
			neighbor.expiresAt = time.Now().Add(neighbor.ttl)
			o.neighbors[key] = neighbor

			return true
		}
	}

	return false
}

// wait sleeps until deadline, aging out neighbors on the way so that a socket
// which is down does not keep stale neighbors published.
func (o *LLDP) wait(ctx context.Context, notifyCh chan<- struct{}, deadline time.Time) {
	for {
		next := o.expire(ctx, notifyCh)
		if next.IsZero() || deadline.Before(next) {
			next = deadline
		}

		timer := time.NewTimer(time.Until(next))

		select {
		case <-ctx.Done():
			timer.Stop()

			return
		case <-timer.C:
		}

		if !time.Now().Before(deadline) {
			return
		}
	}
}

// expire drops timed-out neighbors, notifying the controller if any went away, and reports when
// the next one is due. The zero time means nothing is cached, so no deadline is needed.
func (o *LLDP) expire(ctx context.Context, notifyCh chan<- struct{}) time.Time {
	o.mu.Lock()

	now := time.Now()
	changed := false

	var next time.Time

	for key, neighbor := range o.neighbors {
		switch {
		case !neighbor.expiresAt.After(now):
			delete(o.neighbors, key)

			changed = true
		case next.IsZero() || neighbor.expiresAt.Before(next):
			next = neighbor.expiresAt
		}
	}

	o.mu.Unlock()

	if changed {
		channel.SendWithContext(ctx, notifyCh, struct{}{})
	}

	return next
}

// forget discards all observations.
func (o *LLDP) forget() {
	o.mu.Lock()
	defer o.mu.Unlock()

	clear(o.neighbors)
}

// AddressSpecs implements Operator interface.
func (o *LLDP) AddressSpecs() []network.AddressSpecSpec {
	return nil
}

// LinkSpecs implements Operator interface.
func (o *LLDP) LinkSpecs() []network.LinkSpecSpec {
	return nil
}

// RouteSpecs implements Operator interface.
func (o *LLDP) RouteSpecs() []network.RouteSpecSpec {
	return nil
}

// HostnameSpecs implements Operator interface.
func (o *LLDP) HostnameSpecs() []network.HostnameSpecSpec {
	return nil
}

// ResolverSpecs implements Operator interface.
func (o *LLDP) ResolverSpecs() []network.ResolverSpecSpec {
	return nil
}

// TimeServerSpecs implements Operator interface.
func (o *LLDP) TimeServerSpecs() []network.TimeServerSpecSpec {
	return nil
}

// LLDPNeighborSpecs implements Operator interface.
//
// The result is independently owned and sorted by the neighbors' protocol identity.
func (o *LLDP) LLDPNeighborSpecs() []network.LLDPNeighborSpec {
	o.mu.Lock()
	defer o.mu.Unlock()

	items := make([]network.LLDPNeighborSpec, 0, len(o.neighbors))

	for _, key := range slices.Sorted(maps.Keys(o.neighbors)) {
		spec := o.neighbors[key].spec
		spec.ManagementAddresses = slices.Clone(spec.ManagementAddresses)
		spec.VLANs = slices.Clone(spec.VLANs)
		items = append(items, spec)
	}

	return items
}
