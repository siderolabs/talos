// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	talosruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform"
	platformerrors "github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	configresource "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// PlatformConfigurator is a reduced interface of runtime.Platform.
type PlatformConfigurator interface {
	Name() string
	Configuration(context.Context) ([]byte, error)
}

// PlatformEventer sends events based on the config process via platform-specific interface.
type PlatformEventer interface {
	FireEvent(context.Context, platform.Event)
}

// Setter sets the current machine config.
type Setter interface {
	SetConfig(config.Provider) error
}

// AcquireController loads the machine configuration from multiple sources.
type AcquireController struct {
	PlatformConfiguration PlatformConfigurator
	PlatformEvent         PlatformEventer
	ConfigSetter          Setter
	EventPublisher        talosruntime.Publisher
	ValidationMode        validation.RuntimeMode
	ConfigPath            string

	configSourcesUsed []string
}

// Name implements controller.Controller interface.
func (ctrl *AcquireController) Name() string {
	return "config.AcquireController"
}

// Inputs implements controller.Controller interface.
func (ctrl *AcquireController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.AcquireConfigSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: configresource.NamespaceName,
			Type:      configresource.MachineConfigType,
			ID:        optional.Some(configresource.MaintenanceID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MaintenanceServiceRequestType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *AcquireController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: v1alpha1.AcquireConfigStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: runtime.MaintenanceServiceRequestType,
			Kind: controller.OutputExclusive,
		},
	}
}

// stateMachineFunc represents the state machine of config.AcquireController.
type stateMachineFunc func(context.Context, controller.Runtime, *zap.Logger) (stateMachineFunc, config.Provider, error)

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *AcquireController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.ConfigPath == "" {
		ctrl.ConfigPath = constants.ConfigPath
	}

	// start always with loading config from disk
	var currentState stateMachineFunc = ctrl.stateDisk

	// initialize with empty sources
	ctrl.configSourcesUsed = []string{}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// check the spec first
		_, err := safe.ReaderGet[*v1alpha1.AcquireConfigSpec](ctx, r, v1alpha1.NewAcquireConfigSpec().Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				// spec is not found, wait for it
				continue
			}

			return fmt.Errorf("failed to get spec: %w", err)
		}

		// run the state machine
		for {
			newState, cfg, err := currentState(ctx, r, logger)
			if err != nil {
				ctrl.EventPublisher.Publish(ctx, &machineapi.ConfigLoadErrorEvent{
					Error: err.Error(),
				})

				ctrl.PlatformEvent.FireEvent(
					ctx,
					platform.Event{
						Type:    platform.EventTypeFailure,
						Message: "Error loading and validating Talos machine config.",
						Error:   err,
					},
				)

				return err
			}

			if cfg != nil {
				// apply config
				if err = ctrl.ConfigSetter.SetConfig(cfg); err != nil {
					return fmt.Errorf("failed to set config: %w", err)
				}
			}

			if newState == nil {
				// wait for reconcile event, keep running in the same state
				break
			}

			currentState = newState
		}

		r.ResetRestartBackoff()
	}
}

// stateDisk acquires machine configuration from disk (STATE partition).
//
// Transitions:
//
//	--> platform: no config found on disk, proceed to platform
//	--> maintenanceEnter: config found on disk, but it's incomplete, proceed to maintenance
//	--> done: config found on disk, and it's complete
func (ctrl *AcquireController) stateDisk(ctx context.Context, r controller.Runtime, logger *zap.Logger) (stateMachineFunc, config.Provider, error) {
	cfg, err := ctrl.loadFromDisk(logger)
	if err != nil {
		return nil, nil, err
	}

	if cfg != nil {
		ctrl.configSourcesUsed = append(ctrl.configSourcesUsed, "state")
	}

	switch {
	case cfg == nil:
		// no config loaded, proceed to platform
		return ctrl.statePlatform, nil, nil
	case cfg.CompleteForBoot():
		// complete config, we are done
		return ctrl.stateDone, cfg, nil
	default:
		// incomplete config, proceed to maintenance
		return ctrl.stateMaintenanceEnter, cfg, nil
	}
}

// validationModeDiskConfig is a "fake" validation mode for config loaded from disk.
type validationModeDiskConfig struct{}

// RequiresInstall implements validation.RuntimeMode interface.
func (validationModeDiskConfig) RequiresInstall() bool {
	return false
}

// InContainer implements validation.RuntimeMode interface.
func (validationModeDiskConfig) InContainer() bool {
	// containers don't persist config to disk
	return false
}

// String implements validation.RuntimeMode interface.
func (validationModeDiskConfig) String() string {
	return "diskConfig"
}

// loadFromDisk is a helper function for stateDisk.
func (ctrl *AcquireController) loadFromDisk(logger *zap.Logger) (config.Provider, error) {
	logger.Debug("loading config from STATE", zap.String("path", ctrl.ConfigPath))

	_, err := os.Stat(ctrl.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// no saved machine config
			return nil, nil
		}

		return nil, fmt.Errorf("failed to stat %s: %w", ctrl.ConfigPath, err)
	}

	cfg, err := configloader.NewFromFile(ctrl.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from STATE: %w", err)
	}

	// if the STATE partition is present & contains machine config, Talos is already installed
	warnings, err := cfg.Validate(validationModeDiskConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to validate on-disk config: %w", err)
	}

	for _, warning := range warnings {
		logger.Warn("config validation warning", zap.String("warning", warning))
	}

	return cfg, nil
}

// statePlatform acquires machine configuration from the platform source.
//
// Transitions:
//
//	--> maintenanceEnter: config loaded from platform, but it's incomplete, or no config from platform: proceed to maintenance
//	--> done: config loaded from platform, and it's complete
func (ctrl *AcquireController) statePlatform(ctx context.Context, r controller.Runtime, logger *zap.Logger) (stateMachineFunc, config.Provider, error) {
	cfg, err := ctrl.loadFromPlatform(ctx, logger)
	if err != nil {
		return nil, nil, err
	}

	if cfg != nil {
		ctrl.configSourcesUsed = append(ctrl.configSourcesUsed, ctrl.PlatformConfiguration.Name())
	}

	switch {
	case cfg == nil:
		fallthrough
	case !cfg.CompleteForBoot():
		// incomplete or missing config, proceed to maintenance
		return ctrl.stateMaintenanceEnter, cfg, nil
	default:
		// complete config, we are done
		return ctrl.stateDone, cfg, nil
	}
}

// loadFromPlatform is a helper function for statePlatform.
func (ctrl *AcquireController) loadFromPlatform(ctx context.Context, logger *zap.Logger) (config.Provider, error) {
	platformName := ctrl.PlatformConfiguration.Name()

	logger.Info("downloading config", zap.String("platform", platformName))

	cfgBytes, err := ctrl.PlatformConfiguration.Configuration(ctx)
	if err != nil {
		if errors.Is(err, platformerrors.ErrNoConfigSource) {
			// no config in the platform
			return nil, nil
		}

		return nil, fmt.Errorf("error acquiring via platform %s: %w", platformName, err)
	}

	// Detect if config is a gzip archive and unzip it if so
	contentType := http.DetectContentType(cfgBytes)
	if contentType == "application/x-gzip" {
		var gzipReader *gzip.Reader

		gzipReader, err = gzip.NewReader(bytes.NewReader(cfgBytes))
		if err != nil {
			return nil, fmt.Errorf("error creating gzip reader: %w", err)
		}

		//nolint:errcheck
		defer gzipReader.Close()

		var unzippedData []byte

		unzippedData, err = io.ReadAll(gzipReader)
		if err != nil {
			return nil, fmt.Errorf("error unzipping machine config: %w", err)
		}

		cfgBytes = unzippedData
	}

	cfg, err := configloader.NewFromBytes(cfgBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to load config via platform %s: %w", platformName, err)
	}

	warnings, err := cfg.Validate(ctrl.ValidationMode)
	if err != nil {
		return nil, fmt.Errorf("failed to validate config acquired via platform %s: %w", platformName, err)
	}

	for _, warning := range warnings {
		logger.Warn("config validation warning", zap.String("platform", platformName), zap.String("warning", warning))
	}

	return cfg, nil
}

// stateMaintenanceEnter initializes maintenance service.
//
// Transitions:
//
//	--> maintenance: run the maintenance service
func (ctrl *AcquireController) stateMaintenanceEnter(ctx context.Context, r controller.Runtime, logger *zap.Logger) (stateMachineFunc, config.Provider, error) {
	logger.Info("entering maintenance service")

	// nb: we treat maintenance mode as an "activate"
	// event b/c the user is expected to be able to
	// interact with the system at this point.
	ctrl.PlatformEvent.FireEvent(
		ctx,
		platform.Event{
			Type:    platform.EventTypeActivate,
			Message: "Talos booted into maintenance mode. Ready for user interaction.",
		},
	)

	// add "fake" events to signal when Talos enters and leaves maintenance mode
	ctrl.EventPublisher.Publish(ctx, &machineapi.TaskEvent{
		Action: machineapi.TaskEvent_START,
		Task:   "runningMaintenance",
	})

	return ctrl.stateMaintenance, nil, nil
}

// stateMaintenance acquires machine configuration from the maintenance service.
//
// Transitions:
//
//	--> maintenanceLeave: config loaded from maintenance service, and it's complete
func (ctrl *AcquireController) stateMaintenance(ctx context.Context, r controller.Runtime, logger *zap.Logger) (stateMachineFunc, config.Provider, error) {
	// init maintenance
	if err := safe.WriterModify(ctx, r, runtime.NewMaintenanceServiceRequest(), func(*runtime.MaintenanceServiceRequest) error {
		return nil
	}); err != nil {
		return nil, nil, fmt.Errorf("failed creating maintenance service request: %w", err)
	}

	// check current maintenance config
	cfgResource, err := safe.ReaderGetByID[*configresource.MachineConfig](ctx, r, configresource.MaintenanceID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// no config loaded, wait for it
			return nil, nil, nil
		}

		return nil, nil, fmt.Errorf("failed to get maintenance config: %w", err)
	}

	cfg := cfgResource.Provider()

	if cfg.CompleteForBoot() {
		// complete config, we are done
		ctrl.configSourcesUsed = append(ctrl.configSourcesUsed, "maintenance")

		return ctrl.stateMaintenanceLeave, cfg, nil
	}

	// incomplete config, keep waiting, but apply new config
	return nil, cfg, nil
}

// stateMaintenanceLeave leaves the maintenance service.
//
// Transitions:
//
//	--> done: proceed to done state
func (ctrl *AcquireController) stateMaintenanceLeave(ctx context.Context, r controller.Runtime, logger *zap.Logger) (stateMachineFunc, config.Provider, error) {
	// stop the maintenance service
	ready, err := r.Teardown(ctx, runtime.NewMaintenanceServiceRequest().Metadata())

	switch {
	case err != nil && !state.IsNotFoundError(err):
		return nil, nil, fmt.Errorf("failed to tear down maintenance service: %w", err)
	case err == nil && !ready:
		// wait for the maintenance service to be torn down
		return nil, nil, nil
	case err == nil && ready:
		if err = r.Destroy(ctx, runtime.NewMaintenanceServiceRequest().Metadata()); err != nil {
			return nil, nil, fmt.Errorf("failed cleaning up maintenance service request: %w", err)
		}
	}

	ctrl.EventPublisher.Publish(ctx, &machineapi.TaskEvent{
		Action: machineapi.TaskEvent_STOP,
		Task:   "runningMaintenance",
	})

	logger.Info("leaving maintenance service")

	return ctrl.stateDone, nil, nil
}

// stateDone is the final state of the controller.
func (ctrl *AcquireController) stateDone(ctx context.Context, r controller.Runtime, logger *zap.Logger) (stateMachineFunc, config.Provider, error) {
	if err := safe.WriterModify(ctx, r, v1alpha1.NewAcquireConfigStatus(), func(_ *v1alpha1.AcquireConfigStatus) error {
		return nil
	}); err != nil {
		return nil, nil, fmt.Errorf("failed to write status: %w", err)
	}

	ctrl.PlatformEvent.FireEvent(
		ctx,
		platform.Event{
			Type:    platform.EventTypeConfigLoaded,
			Message: "Talos machine config loaded successfully.",
		},
	)

	logger.Info("machine config loaded successfully", zap.Strings("sources", ctrl.configSourcesUsed))

	// fall through to the controller loop
	return ctrl.stateFinal, nil, nil
}

// stateFinal just makes the controller do nothing.
func (ctrl *AcquireController) stateFinal(ctx context.Context, r controller.Runtime, logger *zap.Logger) (stateMachineFunc, config.Provider, error) {
	return nil, nil, nil
}
