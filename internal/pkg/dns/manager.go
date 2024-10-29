// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dns

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/netip"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin/pkg/proxy"
	"github.com/hashicorp/go-multierror"
	dnssrv "github.com/miekg/dns"
	"github.com/siderolabs/gen/xiter"
	"github.com/thejerf/suture/v4"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

// ErrCreatingRunner is an error that occurs when creating a runner.
var ErrCreatingRunner = errors.New("error creating runner")

// Manager manages DNS runners.
type Manager struct {
	originalCtx  context.Context //nolint:containedctx
	handler      *Handler
	nodeHandler  *NodeHandler
	rootHandler  *Cache
	s            *suture.Supervisor
	supervisorCh <-chan error
	logger       *zap.Logger
	runners      map[AddressPair]suture.ServiceToken
}

// NewManager creates a new manager.
func NewManager(mr MemberReader, hook suture.EventHook, logger *zap.Logger) *Manager {
	handler := NewHandler(logger)
	nodeHandler := NewNodeHandler(handler, &addrResolver{mr: mr}, logger)
	rootHandler := NewCache(nodeHandler, logger)

	m := &Manager{
		handler:     handler,
		nodeHandler: nodeHandler,
		rootHandler: rootHandler,
		s:           suture.New("dns-resolve-cache-runners", suture.Spec{EventHook: hook}),
		logger:      logger,
		runners:     map[AddressPair]suture.ServiceToken{},
	}

	// If we lost ref to the manager. Ensure finalizer is called and all upstreams are collected.
	runtime.SetFinalizer(m, (*Manager).finalize)

	return m
}

// ServeBackground starts the manager in the background. It panics if the manager is not initialized or if it's called
// more than once.
func (m *Manager) ServeBackground(ctx context.Context) {
	switch {
	case m.originalCtx == nil:
		m.originalCtx = ctx
	case m.originalCtx != ctx:
		panic("Manager.ServeBackground is called with a different context")
	case m.originalCtx == ctx:
		return
	}

	m.supervisorCh = m.s.ServeBackground(ctx)
}

// AddressPair represents a network and address with port.
type AddressPair struct {
	Network string
	Addr    netip.AddrPort
}

// String returns a string representation of the address pair.
func (a AddressPair) String() string { return "Network: " + a.Network + ", Addr: " + a.Addr.String() }

// RunAll updates and run the runners managed by the manager. It returns an iterator which yields the address pairs for
// all running and attempted ro run configurations. It's mandatory to range over the iterator to ensure all runners are updated.
func (m *Manager) RunAll(pairs iter.Seq[AddressPair], forwardEnabled bool) iter.Seq2[RunResult, error] {
	return func(yield func(RunResult, error) bool) {
		preserve := make(map[AddressPair]struct{}, len(m.runners))

		for cfg := range pairs {
			preserve[cfg] = struct{}{}

			if _, ok := m.runners[cfg]; ok {
				if !yield(makeResult(cfg, StatusRunning), nil) {
					return
				}

				continue
			}

			opts, err := newDNSRunnerOpts(cfg, m.rootHandler, forwardEnabled)
			if err != nil {
				err = fmt.Errorf("%w: %w", ErrCreatingRunner, err)
			} else {
				m.runners[cfg] = m.s.Add(NewRunner(opts, m.logger))
			}

			if !yield(makeResult(cfg, StatusNew), err) {
				return
			}
		}

		for cfg, token := range m.runners {
			if _, ok := preserve[cfg]; ok {
				continue
			}

			err := m.s.RemoveAndWait(token, 0)
			if err != nil {
				err = fmt.Errorf("error removing runner: %w", err)
			}

			if !yield(makeResult(cfg, StatusRemoved), err) {
				return
			}

			delete(m.runners, cfg)
		}
	}
}

func makeResult(cfg AddressPair, s Status) RunResult { return RunResult{AddressPair: cfg, Status: s} }

// AllowNodeResolving enables or disables the node resolving feature.
func (m *Manager) AllowNodeResolving(enabled bool) { m.nodeHandler.SetEnabled(enabled) }

// SetUpstreams sets the upstreams for the DNS handler. It returns true if the upstreams were updated, false otherwise.
func (m *Manager) SetUpstreams(prxs iter.Seq[*proxy.Proxy]) bool { return m.handler.SetProxy(prxs) }

// ClearAll stops and removes all runners. It returns an iterator which yields the address pairs that were removed
// and/or errors that occurred during the removal process. It's mandatory to range over the iterator to ensure all
// runners are stopped.
func (m *Manager) ClearAll(dry bool) error {
	if dry {
		return nil
	}

	var multiErr *multierror.Error

	for _, err := range m.clearAll() {
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
		}
	}

	return multiErr.ErrorOrNil()
}

func (m *Manager) clearAll() iter.Seq2[AddressPair, error] {
	return func(yield func(AddressPair, error) bool) {
		if len(m.runners) == 0 {
			return
		}

		defer m.handler.Stop()

		removeAndWait := m.s.RemoveAndWait
		if m.originalCtx.Err() != nil {
			// ctx canceled, no reason to remove runners from Supervisor since they are already dropped
			removeAndWait = func(id suture.ServiceToken, timeout time.Duration) error { return nil }
		}

		for runData, token := range m.runners {
			err := removeAndWait(token, 0)
			if err != nil {
				err = fmt.Errorf("error removing runner: %w", err)
			}

			if !yield(runData, err) {
				return
			}

			delete(m.runners, runData)
		}
	}
}

func (m *Manager) finalize() {
	for data, err := range m.clearAll() {
		if err != nil {
			m.logger.Error("error stopping dns runner", zap.Error(err))
		}

		m.logger.Info(
			"dns runner stopped from finalizer!",
			zap.String("address", data.Addr.String()),
			zap.String("network", data.Network),
		)
	}
}

// Done reports if superwisor finished execution.
func (m *Manager) Done() <-chan error {
	return m.supervisorCh
}

type addrResolver struct {
	mr MemberReader
}

func (s *addrResolver) ResolveAddr(ctx context.Context, qType uint16, name string) (iter.Seq[netip.Addr], bool) {
	name = strings.TrimRight(name, ".")

	items, err := s.mr.ReadMembers(ctx)
	if err != nil {
		return nil, false
	}

	found, ok := xiter.Find(func(res *cluster.Member) bool {
		return fqdnMatch(name, res.TypedSpec().Hostname) || fqdnMatch(name, res.Metadata().ID())
	}, items)
	if !ok {
		return nil, false
	}

	return xiter.Filter(
		func(addr netip.Addr) bool {
			return (qType == dnssrv.TypeA && addr.Is4()) || (qType == dnssrv.TypeAAAA && addr.Is6())
		},
		slices.Values(found.TypedSpec().Addresses),
	), true
}

func fqdnMatch(what, where string) bool {
	what = strings.TrimRight(what, ".")
	where = strings.TrimRight(where, ".")

	if what == where {
		return true
	}

	first, _, found := strings.Cut(where, ".")
	if !found {
		return false
	}

	return what == first
}

// MemberReader is an interface to read members.
type MemberReader interface {
	ReadMembers(ctx context.Context) (iter.Seq[*cluster.Member], error)
}

func newDNSRunnerOpts(cfg AddressPair, rootHandler dnssrv.Handler, forwardEnabled bool) (RunnerOptions, error) {
	if cfg.Addr.Addr().Is6() && !strings.HasSuffix(cfg.Network, "6") {
		cfg.Network += "6"
	}

	var serverOpts RunnerOptions

	controlFn, ctrlErr := MakeControl(cfg.Network, forwardEnabled)
	if ctrlErr != nil {
		return serverOpts, fmt.Errorf("error creating %q control function: %w", cfg.Network, ctrlErr)
	}

	switch cfg.Network {
	case "udp", "udp6":
		packetConn, err := NewUDPPacketConn(cfg.Network, cfg.Addr.String(), controlFn)
		if err != nil {
			return serverOpts, fmt.Errorf("error creating %q packet conn: %w", cfg.Network, err)
		}

		serverOpts = RunnerOptions{
			PacketConn: packetConn,
			Handler:    rootHandler,
		}

	case "tcp", "tcp6":
		listener, err := NewTCPListener(cfg.Network, cfg.Addr.String(), controlFn)
		if err != nil {
			return serverOpts, fmt.Errorf("error creating %q listener: %w", cfg.Network, err)
		}

		serverOpts = RunnerOptions{
			Listener:      listener,
			Handler:       rootHandler,
			ReadTimeout:   3 * time.Second,
			WriteTimeout:  5 * time.Second,
			IdleTimeout:   func() time.Duration { return 10 * time.Second },
			MaxTCPQueries: -1,
		}
	}

	return serverOpts, nil
}

// RunResult represents the result of a RunAll iteration.
type RunResult struct {
	AddressPair
	Status Status
}

// Status represents the status of a runner.
type Status int

const (
	// StatusNew represents a new runner.
	StatusNew Status = iota
	// StatusRunning represents a already running runner.
	StatusRunning
	// StatusRemoved represents a removed runner.
	StatusRemoved
)
