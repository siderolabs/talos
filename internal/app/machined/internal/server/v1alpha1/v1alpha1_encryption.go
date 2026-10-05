// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-blockdevice/v2/encryption"
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

// EncryptionServer implements machine.EncryptionServiceServer.
type EncryptionServer struct {
	machine.UnimplementedEncryptionServiceServer

	server *Server
}

// RecoveryKeySupply implements the machine.EncryptionServiceServer interface.
func (s *EncryptionServer) RecoveryKeySupply(ctx context.Context, req *machine.RecoveryKeySupplyRequest) (*machine.RecoveryKeySupplyResponse, error) {
	if len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "recovery key must not be empty")
	}

	st := s.server.Controller.Runtime().State().V1Alpha2().Resources()

	volumeIDs, err := lockedRecoveryKeyVolumes(ctx, st, req.Volumes)
	if err != nil {
		return nil, err
	}

	for _, volumeID := range volumeIDs {
		if err = supplyRecoveryKey(ctx, st, volumeID, req.Key); err != nil {
			return nil, fmt.Errorf("error supplying recovery key for volume %q: %w", volumeID, err)
		}
	}

	return &machine.RecoveryKeySupplyResponse{
		Messages: []*machine.RecoveryKeySupply{
			{
				Volumes: volumeIDs,
			},
		},
	}, nil
}

// RecoveryKeyFetch implements the machine.EncryptionServiceServer interface.
func (s *EncryptionServer) RecoveryKeyFetch(ctx context.Context, req *machine.RecoveryKeyFetchRequest) (*machine.RecoveryKeyFetchResponse, error) {
	st := s.server.Controller.Runtime().State().V1Alpha2().Resources()

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

	results := make([]*machine.RecoveryKeyFetchResult, 0, len(volumeIDs))

	for _, volumeID := range volumeIDs {
		key, err := fetchRecoveryKey(ctx, st, volumeID)
		if err != nil {
			return nil, err
		}

		results = append(results, &machine.RecoveryKeyFetchResult{
			Volume: volumeID,
			Key:    key,
		})
	}

	return &machine.RecoveryKeyFetchResponse{
		Messages: []*machine.RecoveryKeyFetch{
			{
				Results: results,
			},
		},
	}, nil
}

// RecoveryKeyVerify implements the machine.EncryptionServiceServer interface.
func (s *EncryptionServer) RecoveryKeyVerify(ctx context.Context, req *machine.RecoveryKeyVerifyRequest) (*machine.RecoveryKeyVerifyResponse, error) {
	if len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "recovery key must not be empty")
	}

	st := s.server.Controller.Runtime().State().V1Alpha2().Resources()

	volumeIDs, err := recoveryKeyVolumes(ctx, st, req.Volumes)
	if err != nil {
		return nil, err
	}

	results := make([]*machine.RecoveryKeyVerifyResult, 0, len(volumeIDs))

	for _, volumeID := range volumeIDs {
		valid, err := verifyRecoveryKey(ctx, st, volumeID, req.Key)
		if err != nil {
			return nil, err
		}

		results = append(results, &machine.RecoveryKeyVerifyResult{
			Volume: volumeID,
			Valid:  valid,
		})
	}

	return &machine.RecoveryKeyVerifyResponse{
		Messages: []*machine.RecoveryKeyVerify{
			{
				Results: results,
			},
		},
	}, nil
}

// supplyRecoveryKey stores the operator-supplied key for the volume manager to pick up.
//
// The resource is owned by the volume manager, which destroys it once the key was tried. A key which
// was supplied before and not tried yet is never replaced, so the volume manager can't destroy a key
// it has not seen.
func supplyRecoveryKey(ctx context.Context, st state.State, volumeID resource.ID, key []byte) error {
	res := secrets.NewEncryptionRecoveryKey(volumeID)
	res.TypedSpec().Key = slices.Clone(key)

	err := st.Create(ctx, res, state.WithCreateOwner(recoveryKeyOwner))
	if state.IsConflictError(err) {
		return status.Errorf(codes.AlreadyExists, "a recovery key for volume %q is still being tried, retry in a moment", volumeID)
	}

	return err
}

// fetchRecoveryKey returns the generated recovery key of the volume.
//
// Nothing changes on the node: the key stays pending until the operator acknowledges it by verifying it.
func fetchRecoveryKey(ctx context.Context, st state.State, volumeID resource.ID) ([]byte, error) {
	generated, err := safe.StateGetByID[*secrets.GeneratedRecoveryKey](ctx, st, volumeID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, status.Errorf(codes.FailedPrecondition, "no recovery key is pending for volume %q: it was already acknowledged, or the recovery slot is not enrolled yet", volumeID)
		}

		return nil, fmt.Errorf("error getting generated recovery key %q: %w", volumeID, err)
	}

	return generated.TypedSpec().Key, nil
}

// verifyRecoveryKey checks the recovery key against the recovery key slot of the volume.
//
// A valid key is acknowledged: the operator proved to hold the key which is in the slot, so the slot is marked
// as fetched and the generated key is dropped from the node.
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

	if valid {
		if err = acknowledgeRecoveryKey(ctx, st, volumeID, volume); err != nil {
			return false, err
		}
	}

	return valid, nil
}

// acknowledgeRecoveryKey marks the recovery key slot as fetched, and drops the generated key from the node.
func acknowledgeRecoveryKey(ctx context.Context, st state.State, volumeID resource.ID, volume *recoveryVolumeInfo) error {
	token := &luks.Token[*keys.RecoveryToken]{}

	err := volume.provider.ReadToken(ctx, volume.location, volume.recoveryKey.Slot, token)
	if err != nil && !errors.Is(err, encryption.ErrTokenNotFound) {
		return fmt.Errorf("error reading recovery key token of volume %q: %w", volumeID, err)
	}

	if err == nil && token.UserData.Fetched {
		return nil
	}

	fetchedToken := &luks.Token[*keys.RecoveryToken]{
		Type: keys.TokenTypeRecovery,
		UserData: &keys.RecoveryToken{
			KeySlots: []int{volume.recoveryKey.Slot},
			Fetched:  true,
		},
	}

	if err = volume.provider.SetToken(ctx, volume.location, volume.recoveryKey.Slot, fetchedToken); err != nil {
		return fmt.Errorf("error marking recovery key of volume %q as fetched: %w", volumeID, err)
	}

	err = st.Destroy(ctx, secrets.NewGeneratedRecoveryKey(volumeID).Metadata(), state.WithDestroyOwner(recoveryKeyOwner))
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error dropping generated recovery key %q: %w", volumeID, err)
	}

	return nil
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

// lockedRecoveryKeyVolumes resolves the requested volume IDs to the locked volumes which have a recovery key configured.
//
// An empty request means all locked volumes with a recovery key configured, an explicitly requested volume must be locked.
func lockedRecoveryKeyVolumes(ctx context.Context, st state.State, requested []string) ([]resource.ID, error) {
	candidates, err := recoveryKeyVolumes(ctx, st, requested)
	if err != nil {
		return nil, err
	}

	volumeIDs := make([]resource.ID, 0, len(candidates))

	for _, volumeID := range candidates {
		volumeStatus, err := safe.StateGetByID[*block.VolumeStatus](ctx, st, volumeID)
		if err != nil && !state.IsNotFoundError(err) {
			return nil, fmt.Errorf("error getting volume status %q: %w", volumeID, err)
		}

		locked := volumeStatus != nil && volumeStatus.TypedSpec().Phase == block.VolumePhaseLocked

		switch {
		case locked:
			volumeIDs = append(volumeIDs, volumeID)
		case len(requested) > 0:
			return nil, status.Errorf(codes.FailedPrecondition, "volume %q is not locked: the recovery key can only be supplied to unlock a locked volume", volumeID)
		}
	}

	if len(volumeIDs) == 0 {
		return nil, status.Error(codes.NotFound, "no locked volumes with a recovery key configured")
	}

	return volumeIDs, nil
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
