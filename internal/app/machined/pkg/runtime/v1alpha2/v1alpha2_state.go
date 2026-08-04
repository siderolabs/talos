// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/siderolabs/gen/panicsafe"

	"github.com/siderolabs/talos/internal/pkg/ctrltrace"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// State implements runtime.V1alpha2State interface.
type State struct {
	inmemState *inmem.State
	resources  state.State

	namespaceRegistry *registry.NamespaceRegistry
	resourceRegistry  *registry.ResourceRegistry
}

// NewState creates State.
func NewState() (*State, error) {
	s := &State{}

	ctx := context.TODO()

	s.inmemState = inmem.NewStateWithOptions(
		inmem.WithHistoryInitialCapacity(constants.InmemHistoryInitialCapacity),
		inmem.WithHistoryMaxCapacity(constants.InmemHistoryMaxCapacity),
		inmem.WithHistoryGap(constants.InmemHistoryGap),
	)

	s.resources = state.WrapCore(ctrltrace.WrapState(s.inmemState))
	s.namespaceRegistry = registry.NewNamespaceRegistry(s.resources)
	s.resourceRegistry = registry.NewResourceRegistry(s.resources)

	if err := s.namespaceRegistry.RegisterDefault(ctx); err != nil {
		return nil, err
	}

	if err := s.resourceRegistry.RegisterDefault(ctx); err != nil {
		return nil, err
	}

	// register Talos namespaces
	for _, ns := range resources.Namespaces() {
		if err := s.namespaceRegistry.Register(ctx, ns.Name, ns.Description); err != nil {
			return nil, err
		}
	}

	// register Talos resources
	for _, r := range resources.All() {
		if err := s.resourceRegistry.Register(ctx, r); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// Resources implements runtime.V1alpha2State interface.
func (s *State) Resources() state.State {
	return s.resources
}

// NamespaceRegistry implements runtime.V1alpha2State interface.
func (s *State) NamespaceRegistry() *registry.NamespaceRegistry {
	return s.namespaceRegistry
}

// ResourceRegistry implements runtime.V1alpha2State interface.
func (s *State) ResourceRegistry() *registry.ResourceRegistry {
	return s.resourceRegistry
}

// GetConfig implements runtime.V1alpha2State interface.
func (s *State) GetConfig(ctx context.Context) (talosconfig.Provider, error) {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, s.resources, config.ActiveID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, err
	}

	return cfg.Provider(), nil
}

// SetConfig implements runtime.V1alpha2State interface.
func (s *State) SetConfig(ctx context.Context, id string, cfg talosconfig.Provider) error {
	cfgResource := config.NewMachineConfigWithID(cfg, id)

	if cfg == nil {
		err := s.resources.Destroy(ctx, cfgResource.Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		return nil
	}

	oldCfg, err := s.resources.Get(ctx, cfgResource.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return s.resources.Create(ctx, cfgResource)
		}

		return err
	}

	cfgResource.Metadata().SetVersion(oldCfg.Metadata().Version())

	return s.resources.Update(ctx, cfgResource)
}

// RunBackgroundCleanup removes stale events from the event buffer.
func (s *State) RunBackgroundCleanup(ctx context.Context) error {
	if err := panicsafe.RunErr(func() error {
		return s.inmemState.RunHistoryCleanup(ctx, constants.InmemCleanupInterval)
	}); err != nil {
		return fmt.Errorf("failed to run history cleanup: %w", err)
	}

	return nil
}
