// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/config"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// PatchConfiguration applies a change to the machine configuration which is live, persists it, and
// activates it.
//
// It is the seam services other than MachineService write the machine configuration through, so
// that a targeted change and a full ApplyConfiguration are serialized against each other rather
// than racing as two read-modify-write cycles over the same document set.
//
// The change is persisted, never staged: a caller of this is asking for something to happen now.
// It is refused while a configuration applied in try mode waits to be rolled back, or while one
// staged for the next boot waits to be applied: either way the live configuration is not the
// persisted one, and patching one into the other would make a try permanent or drop a staged
// change.
//
// The patched configuration is encoded again from its documents, as with talosctl patch
// machineconfig, so comments and formatting of the configuration last applied are not kept.
//
// Unlike ApplyConfiguration this carries no role check of its own. Callers reach it only through an
// RPC whose own role set admits them, and the patch each of those builds is scoped to a part of one
// document, so the authority to write arbitrary machine configuration is never handed over here.
func (s *Server) PatchConfiguration(ctx context.Context, patch func(config.Container) (config.Provider, error)) error {
	if s.Controller.Runtime().State().Platform().Mode().IsAgent() {
		return status.Error(codes.Unimplemented, "API is not implemented in agent mode")
	}

	if !s.Controller.Runtime().ConfigCompleteForBoot() {
		return status.Error(codes.FailedPrecondition, "machine configuration has not been acquired yet")
	}

	// Fail fast rather than queue, which is the contract ApplyConfiguration already states.
	if !s.applyConfigMu.TryLock() {
		return status.Error(codes.FailedPrecondition, "another apply configuration is already in progress")
	}
	defer s.applyConfigMu.Unlock()

	if err := assertConfigSettled(ctx, s.Controller.Runtime().State().V1Alpha2().Resources(),
		s.Controller.Runtime().ConfigRollbackPending()); err != nil {
		return err
	}

	previous := s.Controller.Runtime().ConfigContainer()

	cfgProvider, err := patch(previous)
	if err != nil {
		return err
	}

	validationMode := modeWrapper{
		Mode:      s.Controller.Runtime().State().Platform().Mode(),
		installed: s.Controller.Runtime().State().Machine().Installed(),
	}

	if _, err = cfgProvider.ValidateAtRuntime(ctx, s.Controller.Runtime().State().V1Alpha2().Resources(), validationMode); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	previousProvider, ok := previous.(config.Provider)
	if !ok {
		return status.Error(codes.Internal, "live machine configuration cannot be restored")
	}

	return commitConfig(previousProvider, cfgProvider,
		s.Controller.Runtime().SetPersistedConfig, s.Controller.Runtime().SetConfig)
}

// commitConfig persists and then activates a patched machine configuration.
//
// When activating fails, the persisted configuration is put back to the one which was live, so a
// request reported as failed is not enacted by the next boot. That is also what was persisted
// before: assertConfigSettled refuses a patch while the two differ, and when nothing was persisted
// the live configuration is the one loaded from the STATE partition.
func commitConfig(previous, next config.Provider, setPersisted, setActive func(config.Provider) error) error {
	if err := setPersisted(next); err != nil {
		return err
	}

	if err := setActive(next); err != nil {
		if restoreErr := setPersisted(previous); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore persisted machine configuration: %w", restoreErr))
		}

		return err
	}

	return nil
}

// assertConfigSettled refuses a patch while the live machine configuration is not the persisted one.
func assertConfigSettled(ctx context.Context, resources state.State, rollbackPending bool) error {
	if rollbackPending {
		return status.Error(codes.FailedPrecondition,
			"a configuration applied in try mode is pending rollback; apply it or let it roll back first")
	}

	// Absent when the configuration was loaded from the STATE partition and nothing was applied
	// since, in which case what is persisted is what is live.
	persisted, err := safe.StateGetByID[*configres.MachineConfig](ctx, resources, configres.PersistentID)
	if state.IsNotFoundError(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("read persisted machine configuration: %w", err)
	}

	active, err := safe.StateGetByID[*configres.MachineConfig](ctx, resources, configres.ActiveID)
	if err != nil {
		return fmt.Errorf("read active machine configuration: %w", err)
	}

	persistedBytes, err := persisted.Provider().Bytes()
	if err != nil {
		return fmt.Errorf("encode persisted machine configuration: %w", err)
	}

	activeBytes, err := active.Provider().Bytes()
	if err != nil {
		return fmt.Errorf("encode active machine configuration: %w", err)
	}

	if !bytes.Equal(persistedBytes, activeBytes) {
		return status.Error(codes.FailedPrecondition,
			"a staged configuration is pending the next reboot; reboot or apply the configuration first")
	}

	return nil
}
