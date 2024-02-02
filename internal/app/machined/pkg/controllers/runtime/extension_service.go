// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	extservices "github.com/siderolabs/talos/pkg/machinery/extensions/services"
)

// ServiceManager is the interface to the v1alpha1 services subsystems.
type ServiceManager interface {
	Load(services ...system.Service) []string
	Start(serviceIDs ...string) error
}

// ExtensionServiceController creates extension services based on the extension service configuration found in the rootfs.
type ExtensionServiceController struct {
	V1Alpha1Services ServiceManager
	ConfigPath       string
}

// Name implements controller.Controller interface.
func (ctrl *ExtensionServiceController) Name() string {
	return "runtime.ExtensionServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ExtensionServiceController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *ExtensionServiceController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ExtensionServiceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	// controller runs only once, as services are static
	serviceFiles, err := os.ReadDir(ctrl.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// directory not present, skip completely
			logger.Debug("extension service directory is not found")

			return nil
		}

		return err
	}

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

	return nil
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
