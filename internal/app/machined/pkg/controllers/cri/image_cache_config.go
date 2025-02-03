// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/dustin/go-humanize"
	"github.com/google/cel-go/common/ast"
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

	DisableCacheCopy bool // used for testing

	cacheCopyDone bool
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
func (ctrl *ImageCacheConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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
		imageCacheDisabled := cfg == nil || cfg.Config().Machine() == nil || !cfg.Config().Machine().Features().ImageCache().LocalEnabled()

		var (
			status     cri.ImageCacheStatus
			copyStatus cri.ImageCacheCopyStatus
			roots      []string
			allReady   bool
		)

		if imageCacheDisabled {
			status = cri.ImageCacheStatusDisabled
			copyStatus = cri.ImageCacheCopyStatusSkipped
		} else {
			status = cri.ImageCacheStatusPreparing

			// image cache is enabled, so create the volume config resources to find the image cache roots
			if err = ctrl.createVolumeConfigISO(ctx, r); err != nil {
				return fmt.Errorf("error creating volume config: %w", err)
			}

			if err = ctrl.createVolumeConfigDisk(ctx, r, cfg.Config()); err != nil {
				return fmt.Errorf("error creating volume config: %w", err)
			}

			cacheVolumeStatus, err := ctrl.analyzeImageCacheVolumes(ctx, logger, r)
			if err != nil {
				return fmt.Errorf("error analyzing image cache volumes: %w", err)
			}

			allReady = cacheVolumeStatus.allReady
			roots = cacheVolumeStatus.roots
			copyStatus = cacheVolumeStatus.copyStatus

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

		logger.Debug("image cache status", zap.String("status", status.String()), zap.String("copy_status", copyStatus.String()))

		if err = safe.WriterModify(ctx, r, cri.NewImageCacheConfig(), func(cfg *cri.ImageCacheConfig) error {
			cfg.TypedSpec().Status = status
			cfg.TypedSpec().CopyStatus = copyStatus
			cfg.TypedSpec().Roots = roots

			return nil
		}); err != nil {
			return fmt.Errorf("error writing ImageCacheConfig: %w", err)
		}
	}
}

func (ctrl *ImageCacheConfigController) createVolumeConfigISO(ctx context.Context, r controller.ReaderWriter) error {
	builder := cel.NewBuilder(celenv.VolumeLocator())

	// volume.name in ["iso9660", "vfat"] && volume.label.startsWith("TALOS_")
	expr := builder.NewCall(
		builder.NextID(),
		operators.LogicalAnd,
		builder.NewCall(
			builder.NextID(),
			operators.In,
			builder.NewSelect(
				builder.NextID(),
				builder.NewIdent(builder.NextID(), "volume"),
				"name",
			),
			builder.NewList(
				builder.NextID(),
				[]ast.Expr{
					builder.NewLiteral(builder.NextID(), types.String("iso9660")),
					builder.NewLiteral(builder.NextID(), types.String("vfat")),
				},
				nil,
			),
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
			volumeCfg.TypedSpec().Provisioning.Wave = block.WaveSystemDisk
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

type imageCacheVolumeStatus struct {
	roots      []string
	allReady   bool
	copyStatus cri.ImageCacheCopyStatus
}

//nolint:gocyclo,cyclop
func (ctrl *ImageCacheConfigController) analyzeImageCacheVolumes(ctx context.Context, logger *zap.Logger, r controller.Reader) (*imageCacheVolumeStatus, error) {
	volumeIDs := []string{VolumeImageCacheDISK, VolumeImageCacheISO} // prefer disk cache over ISO cache
	volumeStatuses := make([]*block.VolumeStatus, 0, len(volumeIDs))

	for _, volumeID := range volumeIDs {
		volumeStatus, err := safe.ReaderGetByID[*block.VolumeStatus](ctx, r, volumeID)
		if err != nil {
			if state.IsNotFoundError(err) {
				// wait for volume statuses to be present
				return &imageCacheVolumeStatus{}, nil
			}

			return nil, fmt.Errorf("error getting volume status: %w", err)
		}

		volumeStatuses = append(volumeStatuses, volumeStatus)
	}

	// we need to ensure that we first wait for the ISO to be either missing or ready,
	// so that we can make a decision on copying the image cache from an ISO to the disk volume
	var isoStatus, diskStatus block.VolumePhase

	for _, volumeStatus := range volumeStatuses {
		switch volumeStatus.Metadata().ID() {
		case VolumeImageCacheISO:
			isoStatus = volumeStatus.TypedSpec().Phase
		case VolumeImageCacheDISK:
			diskStatus = volumeStatus.TypedSpec().Phase
		}
	}

	if isoStatus != block.VolumePhaseMissing && isoStatus != block.VolumePhaseReady {
		return &imageCacheVolumeStatus{}, nil
	}

	isoPresent := isoStatus == block.VolumePhaseReady
	diskMissing := diskStatus == block.VolumePhaseMissing

	roots := make([]string, 0, len(volumeIDs))

	var (
		isoReady, diskReady    bool
		copySource, copyTarget string
	)

	allReady := true

	// analyze volume statuses, and build the roots
	for _, volumeStatus := range volumeStatuses {
		// mount as rw only disk cache if the ISO cache is present
		root, ready, err := ctrl.getImageCacheRoot(ctx, r, volumeStatus, !(volumeStatus.Metadata().ID() == VolumeImageCacheDISK && isoPresent))
		if err != nil {
			return nil, fmt.Errorf("error getting image cache root: %w", err)
		}

		if ready {
			switch volumeStatus.Metadata().ID() {
			case VolumeImageCacheISO:
				isoReady = true
				copySource = root.ValueOr("")
				logger = logger.With(zap.String("iso_size", volumeStatus.TypedSpec().PrettySize))
			case VolumeImageCacheDISK:
				diskReady = true
				copyTarget = root.ValueOr("")
				logger = logger.With(zap.String("disk_size", volumeStatus.TypedSpec().PrettySize))
			}
		}

		allReady = allReady && ready

		if rootPath, ok := root.Get(); ok {
			roots = append(roots, rootPath)
		}
	}

	logger = logger.With(zap.Bool("all_ready", allReady))

	var copyStatus cri.ImageCacheCopyStatus

	switch {
	case !isoPresent:
		// if there's no ISO, we don't need to copy anything
		copyStatus = cri.ImageCacheCopyStatusSkipped
	case diskMissing:
		// if the disk volume is not configured, we can't copy the image cache
		copyStatus = cri.ImageCacheCopyStatusSkipped
	case ctrl.cacheCopyDone:
		// if the copy has already been done, we don't need to do it again
		copyStatus = cri.ImageCacheCopyStatusReady
	case isoReady && diskReady:
		// ready to copy
		if err := ctrl.copyImageCache(ctx, logger, copySource, copyTarget); err != nil {
			return nil, fmt.Errorf("error copying image cache: %w", err)
		}

		copyStatus = cri.ImageCacheCopyStatusReady
	default:
		// waiting for copy preconditions
		copyStatus = cri.ImageCacheCopyStatusPending
	}

	return &imageCacheVolumeStatus{
		roots:      roots,
		allReady:   allReady,
		copyStatus: copyStatus,
	}, nil
}

func (ctrl *ImageCacheConfigController) getImageCacheRoot(ctx context.Context, r controller.Reader, volumeStatus *block.VolumeStatus, mountReadyOnly bool) (optional.Optional[string], bool, error) {
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

	volumeID := volumeStatus.Metadata().ID()

	volumeConfig, err := safe.ReaderGetByID[*block.VolumeConfig](ctx, r, volumeID)
	if err != nil {
		return optional.None[string](), false, fmt.Errorf("error getting volume config: %w", err)
	}

	var mountOpts []mountv2.NewPointOption

	if mountReadyOnly {
		mountOpts = append(mountOpts, mountv2.WithReadonly())
	}

	if err = ctrl.VolumeMounter(volumeID, mountOpts...); err != nil {
		return optional.None[string](), false, fmt.Errorf("error mounting volume: %w", err)
	}

	targetPath := volumeConfig.TypedSpec().Mount.TargetPath

	if volumeID == VolumeImageCacheISO {
		// the ISO volume has a subdirectory with the actual image cache
		targetPath = filepath.Join(targetPath, "imagecache")
	}

	return optional.Some(targetPath), true, nil
}

func (ctrl *ImageCacheConfigController) copyImageCache(ctx context.Context, logger *zap.Logger, source, target string) error {
	logger.Info("copying image cache", zap.String("source", source), zap.String("target", target))

	if ctrl.DisableCacheCopy {
		// used for testing
		return nil
	}

	var bytesCopied int64

	if err := filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking source directory: %w", err)
		} else if err = ctx.Err(); err != nil {
			return err
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %w", err)
		}

		targetPath := filepath.Join(target, relPath)

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("error getting file info: %w", err)
		}

		// we only support directories and files
		switch {
		case info.Mode().IsDir():
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("error creating directory: %w", err)
			}

			return nil
		case info.Mode().IsRegular():
			bytesCopied += info.Size()

			return copyFileSafe(path, targetPath)
		default:
			return fmt.Errorf("unsupported file type %s: %s", info.Mode(), path)
		}
	}); err != nil {
		return fmt.Errorf("error copying image cache: %w", err)
	}

	logger.Info("image cache copied", zap.String("size", humanize.IBytes(uint64(bytesCopied))))

	ctrl.cacheCopyDone = true

	return nil
}

func copyFileSafe(src, dst string) error {
	srcStat, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("error getting source file info: %w", err)
	}

	dstStat, err := os.Stat(dst)
	if err == nil && srcStat.Size() == dstStat.Size() {
		// skipping copy
		return nil
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}

	defer srcFile.Close() //nolint:errcheck

	tempPath := dst + ".tmp"

	dstFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}

	defer dstFile.Close() //nolint:errcheck

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("error copying file: %w, source size is %d", err, srcStat.Size())
	}

	if err = dstFile.Close(); err != nil {
		return fmt.Errorf("error closing destination file: %w", err)
	}

	if err = os.Rename(tempPath, dst); err != nil {
		return fmt.Errorf("error renaming file: %w", err)
	}

	return nil
}
