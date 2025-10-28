// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	machinedruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

var noMatch = cel.MustExpression(cel.ParseBooleanExpression("false", celenv.Empty()))

// MetaProvider wraps acquiring meta.
type MetaProvider interface {
	Meta() machinedruntime.Meta
}

// VolumeConfigController provides volume configuration based on Talos defaults and machine configuration.
type VolumeConfigController struct {
	V1Alpha1Mode machinedruntime.Mode
	MetaProvider MetaProvider
}

// Name implements controller.Controller interface.
func (ctrl *VolumeConfigController) Name() string {
	return "block.VolumeConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VolumeConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaKeyType,
			ID:        optional.Some(runtime.MetaKeyTagToID(meta.StateEncryptionConfig)),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountRequestType,
			Kind:      controller.InputDestroyReady,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeConfigType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VolumeConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeConfigType,
			Kind: controller.OutputShared,
		},
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
	}
}

func labelVolumeMatch(label string) cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("volume.partition_label == '%s'", label), celenv.VolumeLocator()))
}

func labelVolumeMatchAndNonEmpty(label string) cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("volume.partition_label == '%s' && volume.name != ''", label), celenv.VolumeLocator()))
}

func metaMatch() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression(fmt.Sprintf("volume.partition_label == '%s' && volume.name in ['', 'talosmeta'] && volume.size == 1048576u", constants.MetaPartitionLabel), celenv.VolumeLocator())) //nolint:lll
}

func systemDiskMatch() cel.Expression {
	return cel.MustExpression(cel.ParseBooleanExpression("system_disk", celenv.DiskLocator()))
}

func convertEncryptionConfiguration(in configconfig.EncryptionConfig, out *block.VolumeConfigSpec) error {
	if in == nil {
		out.Encryption = block.EncryptionSpec{}

		return nil
	}

	out.Encryption.Provider = in.Provider()
	out.Encryption.Cipher = in.Cipher()
	out.Encryption.KeySize = in.KeySize()
	out.Encryption.BlockSize = in.BlockSize()
	out.Encryption.PerfOptions = in.Options()

	out.Encryption.Keys = make([]block.EncryptionKey, len(in.Keys()))

	for i, key := range in.Keys() {
		out.Encryption.Keys[i].Slot = key.Slot()
		out.Encryption.Keys[i].LockToSTATE = key.LockToSTATE()

		switch {
		case key.Static() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyStatic
			out.Encryption.Keys[i].StaticPassphrase = key.Static().Key()
		case key.NodeID() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyNodeID
		case key.KMS() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyKMS
			out.Encryption.Keys[i].KMSEndpoint = key.KMS().Endpoint()
		case key.TPM() != nil:
			out.Encryption.Keys[i].Type = block.EncryptionKeyTPM
			out.Encryption.Keys[i].TPMCheckSecurebootStatusOnEnroll = key.TPM().CheckSecurebootOnEnroll()
			out.Encryption.Keys[i].TPMPCRs = key.TPM().PCRs()
			out.Encryption.Keys[i].TPMPubKeyPCRs = key.TPM().PubKeyPCRs()
		default:
			return fmt.Errorf("unsupported encryption key type: slot %d", key.Slot())
		}
	}

	return nil
}

// Run implements controller.Controller interface.
func (ctrl *VolumeConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error { //nolint:gocyclo
	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		machineCfg, encryptionMeta, err := ctrl.loadConfiguration(ctx, r)
		if err != nil {
			return err
		}

		var cfg configconfig.Config
		if machineCfg != nil {
			cfg = machineCfg.Config()
		}

		transformers := append(ctrl.getSystemVolumeTransformers(ctx, encryptionMeta, logger), userVolumeTransformers...)

		var resources []volumeResource

		for _, transformer := range transformers {
			r, err := transformer(cfg)
			if err != nil {
				return err
			}

			resources = append(resources, r...)
		}

		volumeConfigsByID, volumeMountRequestsByID, err := ctrl.getExistingVolumes(ctx, r)
		if err != nil {
			return fmt.Errorf("error getting existing user volumes: %w", err)
		}

		for _, resource := range resources {
			if err := ctrl.createVolume(ctx, r, resource, volumeConfigsByID, volumeMountRequestsByID); err != nil {
				return fmt.Errorf("error creating volumes: %w", err)
			}
		}

		if err := ctrl.cleanupUnusedVolumes(ctx, r, volumeConfigsByID, volumeMountRequestsByID, logger); err != nil {
			return fmt.Errorf("error cleaning up unused volumes: %w", err)
		}
	}
}

func (ctrl *VolumeConfigController) loadConfiguration(ctx context.Context, r controller.Runtime) (*config.MachineConfig, *runtime.MetaKey, error) {
	// load config if present
	machineCfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return nil, nil, fmt.Errorf("error fetching machine configuration: %w", err)
	}

	// load STATE encryption meta key
	encryptionMeta, err := safe.ReaderGetByID[*runtime.MetaKey](ctx, r, runtime.MetaKeyTagToID(meta.StateEncryptionConfig))
	if err != nil && !state.IsNotFoundError(err) {
		return nil, nil, fmt.Errorf("error fetching state encryption meta key: %w", err)
	}

	return machineCfg, encryptionMeta, nil
}

type volumeConfigTransformer func(c configconfig.Config) ([]volumeResource, error)

type volumeResource struct {
	VolumeID           string
	Label              string
	TransformFunc      func(vc *block.VolumeConfig) error
	MountTransformFunc func(m *block.VolumeMountRequest) error
}

func (ctrl *VolumeConfigController) createVolume(
	ctx context.Context, r controller.ReaderWriter, rsrc volumeResource,
	volumeConfigsByID map[string]*block.VolumeConfig,
	volumeMountRequestsByID map[string]*block.VolumeMountRequest,
) error {
	volumeConfig := volumeConfigsByID[rsrc.VolumeID]
	volumeMountRequest := volumeMountRequestsByID[rsrc.VolumeID]

	tearingDown := (volumeConfig != nil && volumeConfig.Metadata().Phase() == resource.PhaseTearingDown) ||
		(volumeMountRequest != nil && volumeMountRequest.Metadata().Phase() == resource.PhaseTearingDown)

	// if the volume is being torn down, do the tear down (in the next loop)
	if tearingDown {
		return nil
	}

	delete(volumeConfigsByID, rsrc.VolumeID)
	delete(volumeMountRequestsByID, rsrc.VolumeID)

	if err := safe.WriterModify(ctx, r, block.NewVolumeConfig(block.NamespaceName, rsrc.VolumeID), func(vc *block.VolumeConfig) error {
		if rsrc.Label != "" {
			vc.Metadata().Labels().Set(rsrc.Label, "")
		}

		return rsrc.TransformFunc(vc)
	}); err != nil {
		return fmt.Errorf("error creating volume %s: %w", rsrc.VolumeID, err)
	}

	if rsrc.MountTransformFunc != nil {
		if err := safe.WriterModify(ctx, r, block.NewVolumeMountRequest(block.NamespaceName, rsrc.VolumeID), func(v *block.VolumeMountRequest) error {
			v.Metadata().Labels().Set(block.UserVolumeLabel, "")
			v.TypedSpec().Requester = ctrl.Name()
			v.TypedSpec().VolumeID = rsrc.VolumeID

			return rsrc.MountTransformFunc(v)
		}); err != nil && !xerrors.TagIs[skipUserVolumeMountRequest](err) {
			return fmt.Errorf("error creating volume mount request: %w", err)
		}
	}

	return nil
}

// getExistingVolumes retrieves existing volume configurations and mount requests.
func (ctrl *VolumeConfigController) getExistingVolumes(ctx context.Context, r controller.Runtime) (map[string]*block.VolumeConfig, map[string]*block.VolumeMountRequest, error) {
	labelQuery := []state.ListOption{
		state.WithLabelQuery(resource.LabelExists(block.SystemVolumeLabel)),
		state.WithLabelQuery(resource.LabelExists(block.UserVolumeLabel)),
		state.WithLabelQuery(resource.LabelExists(block.RawVolumeLabel)),
		state.WithLabelQuery(resource.LabelExists(block.ExistingVolumeLabel)),
		state.WithLabelQuery(resource.LabelExists(block.SwapVolumeLabel)),
	}

	volumeConfigs, err := safe.ReaderListAll[*block.VolumeConfig](ctx, r, labelQuery...)
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching volume configs: %w", err)
	}

	volumeMountRequests, err := safe.ReaderListAll[*block.VolumeMountRequest](ctx, r, labelQuery...)
	if err != nil {
		return nil, nil, fmt.Errorf("error fetching volume mount requests: %w", err)
	}

	volumeConfigsByID := xslices.ToMap(
		safe.ToSlice(volumeConfigs, identity),
		func(v *block.VolumeConfig) (resource.ID, *block.VolumeConfig) {
			return v.Metadata().ID(), v
		},
	)

	volumeMountRequestsByID := xslices.ToMap(
		safe.ToSlice(volumeMountRequests, identity),
		func(v *block.VolumeMountRequest) (resource.ID, *block.VolumeMountRequest) {
			return v.Metadata().ID(), v
		},
	)

	return volumeConfigsByID, volumeMountRequestsByID, nil
}

// cleanupUnusedVolumes removes volumes that are no longer needed.
func (ctrl *VolumeConfigController) cleanupUnusedVolumes(
	ctx context.Context,
	r controller.Runtime,
	volumeConfigsByID map[string]*block.VolumeConfig,
	volumeMountRequestsByID map[string]*block.VolumeMountRequest,
	l *zap.Logger,
) error {
	l.Info("cleaning up unused volumes")
	// Clean up unused volume configs
	for _, volumeConfig := range volumeConfigsByID {
		okToDestroy, err := r.Teardown(ctx, volumeConfig.Metadata())
		if err != nil {
			return fmt.Errorf("error tearing down volume config %q: %w", volumeConfig.Metadata().ID(), err)
		}

		if okToDestroy {
			if err = r.Destroy(ctx, volumeConfig.Metadata()); err != nil {
				return fmt.Errorf("error destroying volume config %q: %w", volumeConfig.Metadata().ID(), err)
			}
		}
	}

	// Clean up unused volume mount requests
	for _, volumeMountRequest := range volumeMountRequestsByID {
		okToDestroy, err := r.Teardown(ctx, volumeMountRequest.Metadata())
		if err != nil {
			return fmt.Errorf("error tearing down volume mount request %q: %w", volumeMountRequest.Metadata().ID(), err)
		}

		if okToDestroy {
			if err = r.Destroy(ctx, volumeMountRequest.Metadata()); err != nil {
				return fmt.Errorf("error destroying volume mount request %q: %w", volumeMountRequest.Metadata().ID(), err)
			}
		}
	}

	return nil
}
