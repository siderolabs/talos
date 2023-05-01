// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/google/go-cmp/cmp"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// Runtime implements the Runtime interface.
type Runtime struct {
	c config.Provider
	s runtime.State
	e runtime.EventStream
	l runtime.LoggingManager

	rollbackTimerMu sync.Mutex
	rollbackTimer   *time.Timer
}

// NewRuntime initializes and returns the v1alpha1 runtime.
func NewRuntime(c config.Provider, s runtime.State, e runtime.EventStream, l runtime.LoggingManager) *Runtime {
	return &Runtime{
		c: c,
		s: s,
		e: e,
		l: l,
	}
}

// Config implements the Runtime interface.
func (r *Runtime) Config() config.Provider {
	return r.c
}

// LoadAndValidateConfig implements the Runtime interface.
func (r *Runtime) LoadAndValidateConfig(b []byte) (config.Provider, error) {
	cfg, err := configloader.NewFromBytes(b)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if _, err := cfg.Validate(r.State().Platform().Mode()); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	return cfg, nil
}

// RollbackToConfigAfter implements the Runtime interface.
func (r *Runtime) RollbackToConfigAfter(cfg []byte, timeout time.Duration) error {
	cfgProvider, err := r.LoadAndValidateConfig(cfg)
	if err != nil {
		return err
	}

	r.CancelConfigRollbackTimeout()

	r.rollbackTimer = time.AfterFunc(timeout, func() {
		log.Println("rolling back the configuration")

		if err := r.SetConfig(cfgProvider); err != nil {
			log.Printf("config rollback failed %s", err)
		}
	})

	return nil
}

// CancelConfigRollbackTimeout implements the Runtime interface.
func (r *Runtime) CancelConfigRollbackTimeout() {
	r.rollbackTimerMu.Lock()
	defer r.rollbackTimerMu.Unlock()

	if r.rollbackTimer != nil {
		r.rollbackTimer.Stop()
		r.rollbackTimer = nil
	}
}

// SetConfig implements the Runtime interface.
func (r *Runtime) SetConfig(cfg config.Provider) error {
	r.c = cfg

	return r.s.V1Alpha2().SetConfig(cfg)
}

// CanApplyImmediate implements the Runtime interface.
func (r *Runtime) CanApplyImmediate(cfg config.Provider) error {
	currentConfig, ok := r.Config().Raw().(*v1alpha1.Config)
	if !ok {
		return fmt.Errorf("current config is not v1alpha1")
	}

	newConfig, ok := cfg.Raw().(*v1alpha1.Config)
	if !ok {
		return fmt.Errorf("new config is not v1alpha1")
	}

	// copy the config as we're going to modify it
	newConfig = newConfig.DeepCopy()

	// the config changes allowed to be applied immediately are:
	// * .debug
	// * .cluster
	// * .machine.time
	// * .machine.certCANs
	// * .machine.install
	// * .machine.network
	// * .machine.sysfs
	// * .machine.sysctls
	// * .machine.logging
	// * .machine.controlplane
	// * .machine.kubelet
	// * .machine.kernel
	// * .machine.registries (note that auth is not applied immediately, containerd limitation)
	// * .machine.pods
	// * .machine.seccompProfiles
	// * .machine.nodeLabels
	// * .machine.features.kubernetesTalosAPIAccess
	newConfig.ConfigDebug = currentConfig.ConfigDebug
	newConfig.ClusterConfig = currentConfig.ClusterConfig

	if newConfig.MachineConfig != nil && currentConfig.MachineConfig != nil {
		newConfig.MachineConfig.MachineTime = currentConfig.MachineConfig.MachineTime
		newConfig.MachineConfig.MachineCertSANs = currentConfig.MachineConfig.MachineCertSANs
		newConfig.MachineConfig.MachineInstall = currentConfig.MachineConfig.MachineInstall
		newConfig.MachineConfig.MachineNetwork = currentConfig.MachineConfig.MachineNetwork
		newConfig.MachineConfig.MachineSysfs = currentConfig.MachineConfig.MachineSysfs
		newConfig.MachineConfig.MachineSysctls = currentConfig.MachineConfig.MachineSysctls
		newConfig.MachineConfig.MachineLogging = currentConfig.MachineConfig.MachineLogging
		newConfig.MachineConfig.MachineControlPlane = currentConfig.MachineConfig.MachineControlPlane
		newConfig.MachineConfig.MachineKubelet = currentConfig.MachineConfig.MachineKubelet
		newConfig.MachineConfig.MachineKernel = currentConfig.MachineConfig.MachineKernel
		newConfig.MachineConfig.MachineRegistries = currentConfig.MachineConfig.MachineRegistries
		newConfig.MachineConfig.MachinePods = currentConfig.MachineConfig.MachinePods
		newConfig.MachineConfig.MachineSeccompProfiles = currentConfig.MachineConfig.MachineSeccompProfiles
		newConfig.MachineConfig.MachineNodeLabels = currentConfig.MachineConfig.MachineNodeLabels
		newConfig.MachineConfig.MachineUdev = currentConfig.MachineConfig.MachineUdev

		if newConfig.MachineConfig.MachineFeatures != nil && currentConfig.MachineConfig.MachineFeatures != nil {
			newConfig.MachineConfig.MachineFeatures.KubernetesTalosAPIAccessConfig = currentConfig.MachineConfig.MachineFeatures.KubernetesTalosAPIAccessConfig
		}
	}

	if !reflect.DeepEqual(currentConfig, newConfig) {
		diff := cmp.Diff(currentConfig, newConfig, cmp.AllowUnexported(v1alpha1.InstallDiskSizeMatcher{}))

		return fmt.Errorf("this config change can't be applied in immediate mode\ndiff: %s", diff)
	}

	return nil
}

// State implements the Runtime interface.
func (r *Runtime) State() runtime.State {
	return r.s
}

// Events implements the Runtime interface.
func (r *Runtime) Events() runtime.EventStream {
	return r.e
}

// Logging implements the Runtime interface.
func (r *Runtime) Logging() runtime.LoggingManager {
	return r.l
}

// NodeName implements the Runtime interface.
func (r *Runtime) NodeName() (string, error) {
	nodenameResource, err := r.s.V1Alpha2().Resources().Get(context.Background(), resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, k8s.NodenameID, resource.VersionUndefined))
	if err != nil {
		return "", fmt.Errorf("error getting nodename resource: %w", err)
	}

	return nodenameResource.(*k8s.Nodename).TypedSpec().Nodename, nil
}

// IsBootstrapAllowed checks for CRI to be up, checked in the bootstrap method.
func (r *Runtime) IsBootstrapAllowed() bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	svc := &services.CRI{}
	if err := system.WaitForService(system.StateEventUp, svc.ID(r)).Wait(ctx); err != nil {
		return false
	}

	return true
}
