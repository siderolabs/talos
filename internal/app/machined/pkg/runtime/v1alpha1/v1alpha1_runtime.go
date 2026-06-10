// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/config"
	machineconfig "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// Runtime implements the Runtime interface.
type Runtime struct {
	s runtime.State
	e runtime.EventStream
	l runtime.LoggingManager

	rollbackTimerMu sync.Mutex
	rollbackTimer   *time.Timer
}

// NewRuntime initializes and returns the v1alpha1 runtime.
func NewRuntime(s runtime.State, e runtime.EventStream, l runtime.LoggingManager) *Runtime {
	return &Runtime{
		s: s,
		e: e,
		l: l,
	}
}

func (r *Runtime) configProvider() config.Provider {
	cfg, err := r.s.V1Alpha2().GetConfig(context.TODO())
	if err != nil {
		panic(err)
	}

	return cfg
}

// Config implements the Runtime interface.
func (r *Runtime) Config() config.Config {
	cfg := r.configProvider()

	if cfg == nil {
		return nil
	}

	return cfg
}

// ConfigCompleteForBoot implements the Runtime interface.
func (r *Runtime) ConfigCompleteForBoot() bool {
	cfg := r.configProvider()

	if cfg == nil {
		return false
	}

	return cfg.CompleteForBoot()
}

// ConfigContainer implements the Runtime interface.
func (r *Runtime) ConfigContainer() config.Container {
	cfg := r.configProvider()

	if cfg == nil {
		return nil
	}

	return cfg
}

// RollbackToConfigAfter implements the Runtime interface.
func (r *Runtime) RollbackToConfigAfter(timeout time.Duration) error {
	cfgProvider := r.configProvider()

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
	return r.s.V1Alpha2().SetConfig(context.TODO(), machineconfig.ActiveID, cfg)
}

// SetPersistedConfig implements the Runtime interface.
func (r *Runtime) SetPersistedConfig(cfg config.Provider) error {
	return r.s.V1Alpha2().SetConfig(context.TODO(), machineconfig.PersistentID, cfg)
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
	nodenameResource, err := safe.ReaderGet[*k8s.Nodename](
		context.Background(),
		r.s.V1Alpha2().Resources(),
		resource.NewMetadata(k8s.NamespaceName, k8s.NodenameType, k8s.NodenameID, resource.VersionUndefined),
	)
	if err != nil {
		return "", fmt.Errorf("error getting nodename resource: %w", err)
	}

	return nodenameResource.TypedSpec().Nodename, nil
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

// GetSystemInformation returns system information resource if it exists.
func (r *Runtime) GetSystemInformation(ctx context.Context) (*hardware.SystemInformation, error) {
	return safe.StateGet[*hardware.SystemInformation](ctx, r.State().V1Alpha2().Resources(), hardware.NewSystemInformation(hardware.SystemInformationID).Metadata())
}
