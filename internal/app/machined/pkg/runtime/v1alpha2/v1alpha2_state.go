// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"

	talosconfig "github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/k8s"
	"github.com/talos-systems/talos/pkg/resources/secrets"
	"github.com/talos-systems/talos/pkg/resources/time"
	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
)

// State implements runtime.V1alpha2State interface.
type State struct {
	resources state.State

	namespaceRegistry *registry.NamespaceRegistry
	resourceRegistry  *registry.ResourceRegistry
}

// NewState creates State.
func NewState() (*State, error) {
	s := &State{}

	ctx := context.TODO()

	s.resources = state.WrapCore(namespaced.NewState(inmem.Build))
	s.namespaceRegistry = registry.NewNamespaceRegistry(s.resources)
	s.resourceRegistry = registry.NewResourceRegistry(s.resources)

	if err := s.namespaceRegistry.RegisterDefault(ctx); err != nil {
		return nil, err
	}

	if err := s.resourceRegistry.RegisterDefault(ctx); err != nil {
		return nil, err
	}

	// register Talos namespaces
	if err := s.namespaceRegistry.Register(ctx, v1alpha1.NamespaceName, "Talos v1alpha1 subsystems glue resources."); err != nil {
		return nil, err
	}

	if err := s.namespaceRegistry.Register(ctx, config.NamespaceName, "Talos node configuration."); err != nil {
		return nil, err
	}

	if err := s.namespaceRegistry.Register(ctx, k8s.ControlPlaneNamespaceName, "Kubernetes control plane resources."); err != nil {
		return nil, err
	}

	if err := s.namespaceRegistry.Register(ctx, secrets.NamespaceName, "Resources with secret material."); err != nil {
		return nil, err
	}

	// register Talos resources
	for _, r := range []resource.Resource{
		&v1alpha1.BootstrapStatus{},
		&v1alpha1.Service{},
		&config.MachineConfig{},
		&config.MachineType{},
		&config.K8sControlPlane{},
		&k8s.Manifest{},
		&k8s.ManifestStatus{},
		&k8s.StaticPod{},
		&k8s.StaticPodStatus{},
		&k8s.SecretsStatus{},
		&secrets.Etcd{},
		&secrets.Kubernetes{},
		&secrets.Root{},
		&time.Status{},
	} {
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

// SetConfig implements runtime.V1alpha2State interface.
func (s *State) SetConfig(cfg talosconfig.Provider) error {
	cfgResource := config.NewMachineConfig(cfg)
	ctx := context.TODO()

	oldCfg, err := s.resources.Get(ctx, cfgResource.Metadata())
	if err != nil {
		if state.IsNotFoundError(err) {
			return s.resources.Create(ctx, cfgResource)
		}

		return err
	}

	cfgResource.Metadata().SetVersion(oldCfg.Metadata().Version())
	cfgResource.Metadata().BumpVersion()

	return s.resources.Update(ctx, oldCfg.Metadata().Version(), cfgResource)
}
