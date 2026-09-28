// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-blockdevice/v2/encryption/luks"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	blockctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/pkg/encryption/keys"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// recoveryKeyOwner is the controller owning the recovery key resources: the API creates and destroys them on its behalf.
var recoveryKeyOwner = (&blockctrl.VolumeManagerController{}).Name()

// EncryptionRecoveryKeySupply implements the machine.MachineServer interface.
func (s *Server) EncryptionRecoveryKeySupply(ctx context.Context, req *machine.EncryptionRecoveryKeySupplyRequest) (*machine.EncryptionRecoveryKeySupplyResponse, error) {
	if len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "recovery key must not be empty")
	}

	st := s.Controller.Runtime().State().V1Alpha2().Resources()

	volumeIDs, err := recoveryKeyVolumes(ctx, st, req.Volumes)
	if err != nil {
		return nil, err
	}

	for _, volumeID := range volumeIDs {
		volumeStatus, err := safe.StateGetByID[*block.VolumeStatus](ctx, st, volumeID)
		if err != nil && !state.IsNotFoundError(err) {
			return nil, fmt.Errorf("error getting volume status %q: %w", volumeID, err)
		}

		if volumeStatus == nil || volumeStatus.TypedSpec().Phase != block.VolumePhaseLocked {
			return nil, status.Errorf(codes.FailedPrecondition, "volume %q is not locked: the recovery key can only be supplied to unlock a locked volume", volumeID)
		}
	}

	for _, volumeID := range volumeIDs {
		if err = supplyRecoveryKey(ctx, st, volumeID, req.Key); err != nil {
			return nil, fmt.Errorf("error supplying recovery key for volume %q: %w", volumeID, err)
		}
	}

	return &machine.EncryptionRecoveryKeySupplyResponse{
		Messages: []*machine.EncryptionRecoveryKeySupply{
			{
				Volumes: volumeIDs,
			},
		},
	}, nil
}

// EncryptionRecoveryKeyFetch implements the machine.MachineServer interface.
func (s *Server) EncryptionRecoveryKeyFetch(ctx context.Context, req *machine.EncryptionRecoveryKeyFetchRequest) (*machine.EncryptionRecoveryKeyFetchResponse, error) {
	st := s.Controller.Runtime().State().V1Alpha2().Resources()

	volumeIDs := req.Volumes

	if len(volumeIDs) == 0 {
		generated, err := safe.StateListAll[*secrets.GeneratedRecoveryKey](ctx, st)
		if err != nil {
			return nil, fmt.Errorf("error listing generated recovery keys: %w", err)
		}

		for key := range generated.All() {
			volumeIDs = append(volumeIDs, key.Metadata().ID())
		}

		slices.Sort(volumeIDs)

		if len(volumeIDs) == 0 {
			return nil, status.Error(codes.NotFound, "no recovery keys pending to be fetched")
		}
	}

	results := make([]*machine.EncryptionRecoveryKeyFetchResult, 0, len(volumeIDs))

	for _, volumeID := range volumeIDs {
		key, err := fetchRecoveryKey(ctx, st, volumeID)
		if err != nil {
			return nil, err
		}

		results = append(results, &machine.EncryptionRecoveryKeyFetchResult{
			Volume: volumeID,
			Key:    key,
		})
	}

	return &machine.EncryptionRecoveryKeyFetchResponse{
		Messages: []*machine.EncryptionRecoveryKeyFetch{
			{
				Results: results,
			},
		},
	}, nil
}

// EncryptionRecoveryKeyVerify implements the machine.MachineServer interface.
func (s *Server) EncryptionRecoveryKeyVerify(ctx context.Context, req *machine.EncryptionRecoveryKeyVerifyRequest) (*machine.EncryptionRecoveryKeyVerifyResponse, error) {
	if len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "recovery key must not be empty")
	}

	st := s.Controller.Runtime().State().V1Alpha2().Resources()

	volumeIDs, err := recoveryKeyVolumes(ctx, st, req.Volumes)
	if err != nil {
		return nil, err
	}

	results := make([]*machine.EncryptionRecoveryKeyVerifyResult, 0, len(volumeIDs))

	for _, volumeID := range volumeIDs {
		valid, err := verifyRecoveryKey(ctx, st, volumeID, req.Key)
		if err != nil {
			return nil, err
		}

		results = append(results, &machine.EncryptionRecoveryKeyVerifyResult{
			Volume: volumeID,
			Valid:  valid,
		})
	}

	return &machine.EncryptionRecoveryKeyVerifyResponse{
		Messages: []*machine.EncryptionRecoveryKeyVerify{
			{
				Results: results,
			},
		},
	}, nil
}

// supplyRecoveryKey stores the operator-supplied key for the volume manager to pick up.
//
// The resource is owned by the volume manager, so it can destroy it once the key was used.
func supplyRecoveryKey(ctx context.Context, st state.State, volumeID resource.ID, key []byte) error {
	res := secrets.NewEncryptionRecoveryKey(volumeID)
	res.TypedSpec().Key = slices.Clone(key)

	err := st.Create(ctx, res, state.WithCreateOwner(recoveryKeyOwner))
	if err == nil {
		return nil
	}

	if !state.IsConflictError(err) {
		return err
	}

	// a key was supplied before and not consumed yet, replace it
	existing, err := safe.StateGetByID[*secrets.EncryptionRecoveryKey](ctx, st, volumeID)
	if err != nil {
		return err
	}

	existing.TypedSpec().Key = slices.Clone(key)

	return st.Update(ctx, existing, state.WithUpdateOwner(recoveryKeyOwner))
}

// fetchRecoveryKey hands the generated recovery key of the volume over to the operator.
//
// The LUKS token of the recovery slot is marked as fetched (so the key is not regenerated on the next boot),
// and the generated key is dropped from the node.
func fetchRecoveryKey(ctx context.Context, st state.State, volumeID resource.ID) ([]byte, error) {
	generated, err := safe.StateGetByID[*secrets.GeneratedRecoveryKey](ctx, st, volumeID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Errorf(codes.FailedPrecondition, "no recovery key is pending for volume %q: it was already fetched, or the recovery slot is not enrolled yet", volumeID)
		}

		return nil, fmt.Errorf("error getting generated recovery key %q: %w", volumeID, err)
	}

	volume, err := recoveryVolume(ctx, st, volumeID)
	if err != nil {
		return nil, err
	}

	fetchedToken := &luks.Token[*keys.RecoveryToken]{
		Type: keys.TokenTypeRecovery,
		UserData: &keys.RecoveryToken{
			KeySlots: []int{volume.recoveryKey.Slot},
			Fetched:  true,
		},
	}

	if err = volume.provider.SetToken(ctx, volume.location, volume.recoveryKey.Slot, fetchedToken); err != nil {
		return nil, fmt.Errorf("error marking recovery key of volume %q as fetched: %w", volumeID, err)
	}

	if err = st.Destroy(ctx, generated.Metadata(), state.WithDestroyOwner(recoveryKeyOwner)); err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("error dropping generated recovery key %q: %w", volumeID, err)
	}

	return generated.TypedSpec().Key, nil
}

// verifyRecoveryKey checks the recovery key against the recovery key slot of the volume.
func verifyRecoveryKey(ctx context.Context, st state.State, volumeID resource.ID, key []byte) (bool, error) {
	volume, err := recoveryVolume(ctx, st, volumeID)
	if err != nil {
		return false, err
	}

	if !slices.Contains(volume.status.EnrolledEncryptionKeys, block.EncryptionKeyRecovery.String()) {
		return false, status.Errorf(codes.FailedPrecondition, "recovery key is not enrolled yet for volume %q", volumeID)
	}

	// build the same key handler the volume manager uses, so lockToState salting is applied the same way
	handler, err := keys.NewHandler(
		*volume.recoveryKey,
		keys.WithVolumeID(volumeID),
		keys.WithRecoveryKeyGetter(func(context.Context, string) ([]byte, bool, error) {
			return key, true, nil
		}),
		keys.WithSaltGetter(func(ctx context.Context) ([]byte, error) {
			salt, err := safe.StateGetByID[*secrets.EncryptionSalt](ctx, st, secrets.EncryptionSaltID)
			if err != nil {
				return nil, fmt.Errorf("error getting encryption salt: %w", err)
			}

			return salt.TypedSpec().DiskSalt, nil
		}),
	)
	if err != nil {
		return false, fmt.Errorf("error creating recovery key handler: %w", err)
	}

	slotKey, err := handler.GetKey(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("error deriving recovery key: %w", err)
	}

	valid, err := volume.provider.CheckKey(ctx, volume.location, slotKey)
	if err != nil {
		return false, fmt.Errorf("error checking recovery key for volume %q: %w", volumeID, err)
	}

	return valid, nil
}

// recoveryVolumeInfo is what the recovery key operations need to know about a volume.
type recoveryVolumeInfo struct {
	status      *block.VolumeStatusSpec
	recoveryKey *block.EncryptionKey
	location    string
	provider    *luks.LUKS
}

// recoveryVolume resolves the volume configuration and status needed to operate on the recovery key slot.
func recoveryVolume(ctx context.Context, st state.State, volumeID resource.ID) (*recoveryVolumeInfo, error) {
	volumeConfig, err := safe.StateGetByID[*block.VolumeConfig](ctx, st, volumeID)
	if err != nil {
		return nil, fmt.Errorf("error getting volume configuration %q: %w", volumeID, err)
	}

	volumeStatus, err := safe.StateGetByID[*block.VolumeStatus](ctx, st, volumeID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Errorf(codes.FailedPrecondition, "volume %q is not discovered yet", volumeID)
		}

		return nil, fmt.Errorf("error getting volume status %q: %w", volumeID, err)
	}

	if volumeStatus.TypedSpec().Location == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "volume %q is not located yet", volumeID)
	}

	encryptionSpec := volumeConfig.TypedSpec().Encryption

	if encryptionSpec.Provider != block.EncryptionProviderLUKS2 {
		return nil, status.Errorf(codes.FailedPrecondition, "volume %q is not encrypted with LUKS2", volumeID)
	}

	recoveryKeyConfig := findRecoveryKey(encryptionSpec)
	if recoveryKeyConfig == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "volume %q has no recovery key configured", volumeID)
	}

	cipher, err := luks.ParseCipherKind(encryptionSpec.Cipher)
	if err != nil {
		return nil, fmt.Errorf("error parsing cipher: %w", err)
	}

	return &recoveryVolumeInfo{
		status:      volumeStatus.TypedSpec(),
		recoveryKey: recoveryKeyConfig,
		location:    volumeStatus.TypedSpec().Location,
		provider:    luks.New(cipher),
	}, nil
}

// recoveryKeyVolumes resolves the requested volume IDs to the volumes which have a recovery key configured.
//
// An empty request means all volumes with a recovery key configured.
func recoveryKeyVolumes(ctx context.Context, st state.State, requested []string) ([]resource.ID, error) {
	withRecoveryKey, err := volumesWithRecoveryKey(ctx, st)
	if err != nil {
		return nil, err
	}

	if len(requested) == 0 {
		volumeIDs := slices.Sorted(maps.Keys(withRecoveryKey))

		if len(volumeIDs) == 0 {
			return nil, status.Error(codes.NotFound, "no volumes with a recovery key configured")
		}

		return volumeIDs, nil
	}

	volumeIDs := make([]resource.ID, 0, len(requested))

	for _, volumeID := range requested {
		if _, ok := withRecoveryKey[volumeID]; !ok {
			return nil, status.Errorf(codes.InvalidArgument, "volume %q does not exist or has no recovery key configured", volumeID)
		}

		if !slices.Contains(volumeIDs, volumeID) {
			volumeIDs = append(volumeIDs, volumeID)
		}
	}

	return volumeIDs, nil
}

// volumesWithRecoveryKey returns the IDs of the volumes which have a recovery key configured.
func volumesWithRecoveryKey(ctx context.Context, st state.State) (map[resource.ID]struct{}, error) {
	volumeConfigs, err := safe.StateListAll[*block.VolumeConfig](ctx, st)
	if err != nil {
		return nil, fmt.Errorf("error listing volume configurations: %w", err)
	}

	withRecoveryKey := map[resource.ID]struct{}{}

	for vc := range volumeConfigs.All() {
		if findRecoveryKey(vc.TypedSpec().Encryption) != nil {
			withRecoveryKey[vc.Metadata().ID()] = struct{}{}
		}
	}

	return withRecoveryKey, nil
}

// findRecoveryKey returns the recovery key configuration of the encryption spec, if any.
func findRecoveryKey(spec block.EncryptionSpec) *block.EncryptionKey {
	for i := range spec.Keys {
		if spec.Keys[i].Type == block.EncryptionKeyRecovery {
			return &spec.Keys[i]
		}
	}

	return nil
}
