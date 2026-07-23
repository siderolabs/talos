// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/install"
	"github.com/siderolabs/talos/pkg/images"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block/blockhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	crires "github.com/siderolabs/talos/pkg/machinery/resources/cri"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// UnattendedInstallController performs an unattended install driven by the UnattendedInstallConfig config document.
//
// It mirrors the legacy `.machine.install` install behavior, but is driven entirely by the multi-document
// config. It does NOT reboot the node after a successful install; reboot is handled separately.
type UnattendedInstallController struct {
	V1Alpha1Mode v1alpha1runtime.Mode

	// State is the resource state used to match the install disk.
	State state.State

	// InstalledFunc reports whether the node is already installed to disk.
	InstalledFunc func() bool

	// PlatformFunc returns the platform name (e.g. "metal", "aws", etc.).
	PlatformFunc func() string

	// InstallFunc performs the actual install of the given image to the given disk.
	//
	// It is a field to allow the install side-effect to be stubbed in tests.
	InstallFunc func(ctx context.Context, disk, image string, wipe bool) error

	// installMu provides single-flight semantics for the install: only one install may run at a time.
	installMu sync.Mutex
	// installDone records, in-memory, that the installer has already run this boot, so it is never run
	// twice even if the status resource read lags behind a just-written value.
	installDone bool
}

// NewUnattendedInstallController creates an UnattendedInstallController wired to the runtime, with the
// default install behavior (run the installer container, waiting for the image cache around it).
func NewUnattendedInstallController(rt v1alpha1runtime.Runtime) *UnattendedInstallController {
	resources := rt.State().V1Alpha2().Resources()

	return &UnattendedInstallController{
		V1Alpha1Mode:  rt.State().Platform().Mode(),
		State:         resources,
		InstalledFunc: rt.State().Machine().Installed,
		PlatformFunc:  rt.State().Platform().Name,
		InstallFunc: func(ctx context.Context, disk, image string, wipe bool) error {
			if err := crires.WaitForImageCache(ctx, resources); err != nil {
				return fmt.Errorf("failed to wait for the image cache: %w", err)
			}

			if err := install.RunInstallerContainer(
				disk,
				rt.State().Platform().Name(),
				image,
				rt.Config(),
				rt.ConfigContainer(),
				resources,
				crires.RegistryBuilder(resources),
				install.WithForce(true),
				install.WithZero(wipe),
			); err != nil {
				return err
			}

			return crires.WaitForImageCacheCopy(ctx, resources)
		},
	}
}

// Name implements controller.Controller interface.
func (ctrl *UnattendedInstallController) Name() string {
	return "runtime.UnattendedInstallController"
}

// Inputs implements controller.Controller interface.
func (ctrl *UnattendedInstallController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.ImageFactorySchematicType,
			ID:        optional.Some(runtime.ImageFactorySchematicID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiskType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *UnattendedInstallController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.UnattendedInstallStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: runtime.RebootRequestType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *UnattendedInstallController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode == v1alpha1runtime.ModeContainer {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.run(ctx, r, logger); err != nil {
			return err
		}
	}
}

func (ctrl *UnattendedInstallController) run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting machine config: %w", err)
	}

	var doc talosconfig.UnattendedInstallConfig

	if cfg != nil {
		doc = cfg.Config().UnattendedInstallConfig()
	}

	r.StartTrackingOutputs()

	if doc != nil {
		if err = ctrl.reconcile(ctx, r, logger, doc); err != nil {
			return err
		}
	}

	return safe.CleanupOutputs[*runtime.UnattendedInstallStatus](ctx, r)
}

//nolint:gocyclo
func (ctrl *UnattendedInstallController) reconcile(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	doc talosconfig.UnattendedInstallConfig,
) error {
	// Once we have recorded a completed install for this boot, the install target is fixed.
	// A new disk later matching the selector must not flip the reported disk or trigger a reinstall.
	if existing, err := safe.ReaderGetByID[*runtime.UnattendedInstallStatus](ctx, r, runtime.UnattendedInstallStatusID); err != nil {
		if !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting unattended install status: %w", err)
		}
	} else if existing.TypedSpec().Phase == runtime.UnattendedInstallPhaseInstalled {
		// re-affirm the status (so CleanupOutputs retains it): the install target is fixed and a new
		// disk later matching the selector must not trigger a reinstall.
		return ctrl.setStatus(ctx, r, doc, runtime.UnattendedInstallPhaseInstalled, nil)
	}

	if ctrl.InstalledFunc() {
		return ctrl.setStatus(ctx, r, doc, runtime.UnattendedInstallPhaseInstalled, nil)
	}

	// resolve the target disk from the CEL selector against the discovered disks.
	// the selector still matches after the install, so the disk is re-resolved and reported in the
	// status after a reboot (the status resource is in-memory and gone after reboot).
	matchExpr := doc.VolumeSelector()

	matchedDisks, err := blockhelpers.MatchDisks(ctx, ctrl.State, &matchExpr)
	if err != nil {
		return fmt.Errorf("failed to match install disk: %w", err)
	}

	var disk string

	if len(matchedDisks) > 0 {
		if len(matchedDisks) > 1 {
			logger.Warn("multiple disks matched the install selector, using the first one",
				zap.Int("matched", len(matchedDisks)),
				zap.String("disk", matchedDisks[0].TypedSpec().DevPath),
			)
		}

		if disk, err = filepath.EvalSymlinks(matchedDisks[0].TypedSpec().DevPath); err != nil {
			return fmt.Errorf("failed to resolve disk symlink: %w", err)
		}
	}

	if len(matchedDisks) == 0 {
		// disks may not have been discovered yet; record and wait for the next event.
		return ctrl.setStatus(ctx, r, doc, runtime.UnattendedInstallPhasePending, fmt.Errorf("no disk matched the selector"))
	}

	// single-flight: only one install may run at a time.
	if !ctrl.installMu.TryLock() {
		// an install is already in progress; keep the status and wait for it to complete.
		return ctrl.setStatus(ctx, r, doc, runtime.UnattendedInstallPhaseInstalling, nil)
	}
	defer ctrl.installMu.Unlock()

	// the installer already ran this boot: don't run it again, just keep the status as installed.
	if ctrl.installDone {
		installPhase := runtime.UnattendedInstallPhaseInstalled

		if ctrl.shouldReboot(doc) {
			installPhase = runtime.UnattendedInstallPhaseWaitingForReboot
		}

		return ctrl.setStatus(ctx, r, doc, installPhase, nil)
	}

	installerImage := doc.InstallerImage()
	if installerImage == "" {
		installerImage, err = ctrl.getInstallerFromBootEntry(ctx, r)
		if err != nil {
			return ctrl.setStatus(ctx, r, doc, runtime.UnattendedInstallPhaseFailed, fmt.Errorf("failed to determine installer image: %w", err))
		}

		logger.Warn("installer image not specified in config, using image from boot entry", zap.String("image", installerImage))
	}

	if err = ctrl.setStatus(ctx, r, doc, runtime.UnattendedInstallPhaseInstalling, nil); err != nil {
		return err
	}

	logger.Info("installing Talos", zap.String("disk", disk), zap.String("image", installerImage))

	if err = ctrl.InstallFunc(ctx, disk, installerImage, doc.VolumeWipe()); err != nil {
		if statErr := ctrl.setStatus(ctx, r, doc, runtime.UnattendedInstallPhaseFailed, err); statErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to flush install failure status: %w", statErr))
		}

		return fmt.Errorf("failed to run installer: %w", err)
	}

	ctrl.installDone = true

	logger.Info("install successful")

	installPhase := runtime.UnattendedInstallPhaseInstalled

	if ctrl.shouldReboot(doc) {
		logger.Info("requesting reboot after successful install")

		if err = safe.WriterModify(ctx, r, runtime.NewRebootRequest(), func(_ *runtime.RebootRequest) error {
			return nil
		}); err != nil {
			return fmt.Errorf("failed to create reboot request: %w", err)
		}

		installPhase = runtime.UnattendedInstallPhaseWaitingForReboot
	} else {
		logger.Info("not rebooting after successful install (reboot disabled)")
	}

	return ctrl.setStatus(ctx, r, doc, installPhase, nil)
}

func (ctrl *UnattendedInstallController) getInstallerFromBootEntry(ctx context.Context, r controller.Runtime) (string, error) {
	schematic, err := safe.ReaderGetByID[*runtime.ImageFactorySchematic](ctx, r, runtime.ImageFactorySchematicID)
	if err != nil {
		return "", fmt.Errorf("failed to get image factory schematic: %w", err)
	}

	apiURL, err := url.Parse(schematic.TypedSpec().APIURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse image factory API URL: %w", err)
	}

	return images.NewInstallerImage(
		apiURL.Host,
		strings.ToLower(ctrl.PlatformFunc()),
		schematic.TypedSpec().SchematicID,
		"", // automatic fallback to current version if not specified
	), nil
}

func (ctrl *UnattendedInstallController) setStatus(
	ctx context.Context,
	r controller.Runtime,
	doc talosconfig.UnattendedInstallConfig,
	phase runtime.UnattendedInstallPhase,
	statusErr error,
) error {
	return safe.WriterModify(ctx, r, runtime.NewUnattendedInstallStatus(), func(status *runtime.UnattendedInstallStatus) error {
		status.TypedSpec().Image = doc.InstallerImage()
		status.TypedSpec().Phase = phase

		if statusErr != nil {
			status.TypedSpec().Error = statusErr.Error()
		} else {
			status.TypedSpec().Error = ""
		}

		return nil
	})
}

// shouldReboot determines whether the node should reboot after a successful install.
//
// The reboot behavior is controlled by the UnattendedInstallConfig.RebootAfterInstall():
//   - nil (not set): reboot only if an explicit installer image was provided
//   - true: always reboot
//   - false: never reboot
func (ctrl *UnattendedInstallController) shouldReboot(doc talosconfig.UnattendedInstallConfig) bool {
	switch reboot := doc.RebootAfterInstall(); reboot {
	case nil:
		return doc.InstallerImage() != ""
	default:
		return *reboot
	}
}
