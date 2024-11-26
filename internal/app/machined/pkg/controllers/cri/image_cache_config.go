// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	mountv2 "github.com/siderolabs/talos/internal/pkg/mount/v2"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	cfg "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// ServiceManager is the interface to the v1alpha1 services subsystems.
type ServiceManager interface {
	IsRunning(id string) (system.Service, bool, error)
	Load(services ...system.Service) []string
	Start(serviceIDs ...string) error
}

// ImageCacheConfigController manages configures Image Cache.
type ImageCacheConfigController struct {
	V1Alpha1ServiceManager ServiceManager
	VolumeMounter          func(label string, opts ...mountv2.NewPointOption) error
}

// Name implements controller.StatsController interface.
func (ctrl *ImageCacheConfigController) Name() string {
	return "cri.ImageCacheConfigController"
}

// Inputs implements controller.StatsController interface.
func (ctrl *ImageCacheConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(RegistrydServiceID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.StatsController interface.
func (ctrl *ImageCacheConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cri.ImageCacheConfigType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: block.VolumeConfigType,
			Kind: controller.OutputShared,
		},
	}
}

// Volume configuration constants.
const (
	VolumeImageCacheISO  = "IMAGECACHE-ISO"
	VolumeImageCacheDISK = constants.ImageCachePartitionLabel

	MinImageCacheSize = 500 * 1024 * 1024      // 500MB
	MaxImageCacheSize = 1 * 1024 * 1024 * 1024 // 1GB

	RegistrydServiceID = services.RegistryID
)

// Run implements controller.StatsController interface.
//
//nolint:gocyclo,cyclop
func (ctrl *ImageCacheConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting config: %w", err)
		}

		registryDService, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, RegistrydServiceID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting service: %w", err)
		}

		// image cache is disabled
		imageCacheDisabled := cfg == nil || cfg.Config().Machine() == nil || !cfg.Config().Machine().Features().ImageCacheEnabled()

		var (
			status cri.ImageCacheStatus
			roots  []string
		)

		if imageCacheDisabled {
			status = cri.ImageCacheStatusDisabled
		} else {
			status = cri.ImageCacheStatusPreparing

			// image cache is enabled, so create the volume config resources to find the image cache roots
			if err = ctrl.createVolumeConfigISO(ctx, r); err != nil {
				return fmt.Errorf("error creating volume config: %w", err)
			}

			if err = ctrl.createVolumeConfigDisk(ctx, r, cfg.Config()); err != nil {
				return fmt.Errorf("error creating volume config: %w", err)
			}

			allReady := false

			// analyze volume statuses, and build the roots
			for _, volumeID := range []string{VolumeImageCacheISO, VolumeImageCacheDISK} {
				root, ready, err := ctrl.getImageCacheRoot(ctx, r, volumeID)
				if err != nil {
					return fmt.Errorf("error getting image cache root: %w", err)
				}

				allReady = allReady && ready

				if rootPath, ok := root.Get(); ok {
					roots = append(roots, rootPath)
				}
			}

			if allReady && len(roots) == 0 {
				// all volumes identified, but no roots found
				status = cri.ImageCacheStatusDisabled
			}
		}

		if status == cri.ImageCacheStatusPreparing && len(roots) > 0 {
			_, running, err := ctrl.V1Alpha1ServiceManager.IsRunning(RegistrydServiceID)
			if err != nil {
				ctrl.V1Alpha1ServiceManager.Load(services.NewRegistryD())
			}

			if !running {
				if err = ctrl.V1Alpha1ServiceManager.Start(RegistrydServiceID); err != nil {
					return fmt.Errorf("error starting service: %w", err)
				}
			}

			if registryDService != nil && registryDService.TypedSpec().Running && registryDService.TypedSpec().Healthy {
				status = cri.ImageCacheStatusReady
			}
		}

		if err = safe.WriterModify(ctx, r, cri.NewImageCacheConfig(), func(cfg *cri.ImageCacheConfig) error {
			cfg.TypedSpec().Status = status
			cfg.TypedSpec().Roots = roots

			return nil
		}); err != nil {
			return fmt.Errorf("error writing ImageCacheConfig: %w", err)
		}
	}
}

func (ctrl *ImageCacheConfigController) createVolumeConfigISO(ctx context.Context, r controller.ReaderWriter) error {
	builder := cel.NewBuilder(celenv.VolumeLocator())

	// volume.name == "iso9660" && volume.label.startsWith("TALOS_")
	expr := builder.NewCall(
		builder.NextID(),
		operators.LogicalAnd,
		builder.NewCall(
			builder.NextID(),
			operators.Equals,
			builder.NewSelect(
				builder.NextID(),
				builder.NewIdent(builder.NextID(), "volume"),
				"name",
			),
			builder.NewLiteral(builder.NextID(), types.String("iso9660")),
		),
		builder.NewMemberCall(
			builder.NextID(),
			"startsWith",
			builder.NewSelect(
				builder.NextID(),
				builder.NewIdent(builder.NextID(), "volume"),
				"label",
			),
			builder.NewLiteral(builder.NextID(), types.String("TALOS_")),
		),
	)

	boolExpr, err := builder.ToBooleanExpression(expr)
	if err != nil {
		return fmt.Errorf("error creating boolean expression: %w", err)
	}

	return safe.WriterModify(ctx, r, block.NewVolumeConfig(block.NamespaceName, VolumeImageCacheISO), func(volumeCfg *block.VolumeConfig) error {
		volumeCfg.TypedSpec().Type = block.VolumeTypeDisk
		volumeCfg.TypedSpec().Locator = block.LocatorSpec{
			Match: *boolExpr,
		}
		volumeCfg.TypedSpec().Mount.TargetPath = constants.ImageCacheISOMountPoint

		return nil
	})
}

func (ctrl *ImageCacheConfigController) createVolumeConfigDisk(ctx context.Context, r controller.ReaderWriter, cfg cfg.Config) error {
	builder := cel.NewBuilder(celenv.VolumeLocator())

	// volume.partition_label == "IMAGECACHE"
	expr := builder.NewCall(
		builder.NextID(),
		operators.Equals,
		builder.NewSelect(
			builder.NextID(),
			builder.NewIdent(builder.NextID(), "volume"),
			"partition_label",
		),
		builder.NewLiteral(builder.NextID(), types.String(constants.ImageCachePartitionLabel)),
	)

	locatorExpr, err := builder.ToBooleanExpression(expr)
	if err != nil {
		return fmt.Errorf("error creating boolean expression: %w", err)
	}

	// system_disk
	builder = cel.NewBuilder(celenv.DiskLocator())

	expr = builder.NewIdent(builder.NextID(), "system_disk")

	diskExpr, err := builder.ToBooleanExpression(expr)
	if err != nil {
		return fmt.Errorf("error creating boolean expression: %w", err)
	}

	return safe.WriterModify(ctx, r, block.NewVolumeConfig(block.NamespaceName, VolumeImageCacheDISK), func(volumeCfg *block.VolumeConfig) error {
		volumeCfg.TypedSpec().Type = block.VolumeTypePartition
		volumeCfg.TypedSpec().Locator = block.LocatorSpec{
			Match: *locatorExpr,
		}

		if extraCfg, ok := cfg.Volumes().ByName(constants.ImageCachePartitionLabel); ok {
			volumeCfg.TypedSpec().Provisioning.DiskSelector.Match = extraCfg.Provisioning().DiskSelector().ValueOr(*diskExpr)
			volumeCfg.TypedSpec().Provisioning.PartitionSpec.Grow = extraCfg.Provisioning().Grow().ValueOr(false)
			volumeCfg.TypedSpec().Provisioning.PartitionSpec.MinSize = extraCfg.Provisioning().MinSize().ValueOr(MinImageCacheSize)
			volumeCfg.TypedSpec().Provisioning.PartitionSpec.MaxSize = extraCfg.Provisioning().MaxSize().ValueOr(MaxImageCacheSize)
			volumeCfg.TypedSpec().Provisioning.PartitionSpec.Label = constants.ImageCachePartitionLabel
			volumeCfg.TypedSpec().Provisioning.PartitionSpec.TypeUUID = partition.LinuxFilesystemData
			volumeCfg.TypedSpec().Provisioning.FilesystemSpec.Type = block.FilesystemTypeEXT4
		}

		volumeCfg.TypedSpec().Mount.TargetPath = constants.ImageCacheDiskMountPoint

		return nil
	})
}

func (ctrl *ImageCacheConfigController) getImageCacheRoot(ctx context.Context, r controller.Reader, volumeID string) (optional.Optional[string], bool, error) {
	volumeStatus, err := safe.ReaderGetByID[*block.VolumeStatus](ctx, r, volumeID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return optional.None[string](), false, nil
		}

		return optional.None[string](), false, fmt.Errorf("error getting volume status: %w", err)
	}

	switch volumeStatus.TypedSpec().Phase { //nolint:exhaustive
	case block.VolumePhaseMissing:
		// image cache is missing
		return optional.None[string](), true, nil
	case block.VolumePhaseReady:
		// fall through to below
	default:
		// undetermined status
		return optional.None[string](), false, nil
	}

	volumeConfig, err := safe.ReaderGetByID[*block.VolumeConfig](ctx, r, volumeID)
	if err != nil {
		return optional.None[string](), false, fmt.Errorf("error getting volume config: %w", err)
	}

	if err = ctrl.VolumeMounter(volumeID, mountv2.WithReadonly()); err != nil {
		return optional.None[string](), false, fmt.Errorf("error mounting volume: %w", err)
	}

	targetPath := volumeConfig.TypedSpec().Mount.TargetPath

	if volumeID == VolumeImageCacheISO {
		// the ISO volume has a subdirectory with the actual image cache
		targetPath = filepath.Join(targetPath, "imagecache")
	}

	return optional.Some(targetPath), true, nil
}
