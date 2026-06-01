// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	extservices "github.com/siderolabs/talos/pkg/machinery/extensions/services"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ServiceManager is the interface to the v1alpha1 services subsystems.
type ServiceManager interface {
	IsRunning(id string) (system.Service, bool, error)
	Load(services ...system.Service) []string
	Stop(ctx context.Context, serviceIDs ...string) (err error)
	Start(serviceIDs ...string) error
}

// ExtensionServiceController creates extension services based on the extension service configuration found in the rootfs.
type ExtensionServiceController struct {
	V1Alpha1Services ServiceManager
	ConfigPath       string

	configStatusCache map[string]string
}

// Name implements controller.Controller interface.
func (ctrl *ExtensionServiceController) Name() string {
	return "runtime.ExtensionServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ExtensionServiceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.ExtensionServiceConfigStatusType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ExtensionServiceController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *ExtensionServiceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// wait for controller runtime to be ready
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	// extensions loading only needs to run once, as services are static
	serviceFiles, err := os.ReadDir(ctrl.ConfigPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// directory not present, skip completely
			logger.Debug("extension service directory is not found")

			return nil
		}

		return err
	}

	// load initial state of configStatuses
	if ctrl.configStatusCache == nil {
		configStatuses, err := safe.ReaderListAll[*runtime.ExtensionServiceConfigStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing extension services config: %w", err)
		}

		ctrl.configStatusCache = make(map[string]string, configStatuses.Len())

		for res := range configStatuses.All() {
			ctrl.configStatusCache[res.Metadata().ID()] = res.TypedSpec().SpecVersion
		}
	}

	// load services from definitions into the service runner framework
	extServices := map[string]struct{}{}

	for _, serviceFile := range serviceFiles {
		if filepath.Ext(serviceFile.Name()) != ".yaml" {
			logger.Debug("skipping config file", zap.String("filename", serviceFile.Name()))

			continue
		}

		spec, err := ctrl.loadSpec(filepath.Join(ctrl.ConfigPath, serviceFile.Name()))
		if err != nil {
			logger.Error("error loading extension service spec", zap.String("filename", serviceFile.Name()), zap.Error(err))

			continue
		}

		if err = spec.Validate(); err != nil {
			logger.Error("error validating extension service spec", zap.String("filename", serviceFile.Name()), zap.Error(err))

			continue
		}

		if _, exists := extServices[spec.Name]; exists {
			logger.Error("duplicate service spec", zap.String("filename", serviceFile.Name()), zap.String("name", spec.Name))

			continue
		}

		extServices[spec.Name] = struct{}{}

		svc := &services.Extension{
			Spec: spec,
		}

		ctrl.V1Alpha1Services.Load(svc)

		if err = ctrl.V1Alpha1Services.Start(svc.ID(nil)); err != nil {
			return fmt.Errorf("error starting %q service: %w", spec.Name, err)
		}
	}

	// watch for changes in the configStatuses
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		configStatuses, err := safe.ReaderListAll[*runtime.ExtensionServiceConfigStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing extension services config: %w", err)
		}

		configStatusesPresent := map[string]struct{}{}

		for res := range configStatuses.All() {
			configStatusesPresent[res.Metadata().ID()] = struct{}{}

			if ctrl.configStatusCache[res.Metadata().ID()] == res.TypedSpec().SpecVersion {
				continue
			}

			if err = ctrl.handleRestart(ctx, logger, "ext-"+res.Metadata().ID(), res.TypedSpec().SpecVersion); err != nil {
				return err
			}

			ctrl.configStatusCache[res.Metadata().ID()] = res.TypedSpec().SpecVersion
		}

		// cleanup configStatusesCache
		for id := range ctrl.configStatusCache {
			if _, ok := configStatusesPresent[id]; !ok {
				if err = ctrl.handleRestart(ctx, logger, "ext-"+id, "nan"); err != nil {
					return err
				}

				delete(ctrl.configStatusCache, id)
			}
		}
	}
}

func (ctrl *ExtensionServiceController) loadSpec(path string) (extservices.Spec, error) {
	var spec extservices.Spec

	f, err := os.Open(path)
	if err != nil {
		return spec, err
	}

	defer f.Close() //nolint:errcheck

	if err = yaml.NewDecoder(f).Decode(&spec); err != nil {
		return spec, fmt.Errorf("error unmarshalling extension service config: %w", err)
	}

	return spec, nil
}

func (ctrl *ExtensionServiceController) handleRestart(ctx context.Context, logger *zap.Logger, svcName, specVersion string) error {
	_, running, err := ctrl.V1Alpha1Services.IsRunning(svcName)
	if err != nil {
		return nil //nolint:nilerr // IsRunning returns an error only if the service is not found, so ignore it
	}

	// this means it's a new config and the service runner is already waiting for the config to start the service
	// we don't need restart it again since it will be started automatically
	if running && specVersion == "1" {
		return nil
	}

	logger.Warn("extension service config changed, restarting", zap.String("service", svcName))

	if running {
		if err = ctrl.V1Alpha1Services.Stop(ctx, svcName); err != nil {
			return fmt.Errorf("error stopping extension service %s: %w", svcName, err)
		}
	}

	if err = ctrl.V1Alpha1Services.Start(svcName); err != nil {
		return fmt.Errorf("error starting extension service %s: %w", svcName, err)
	}

	return nil
}
