// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// SeccompProfileFileController manages the Seccomp Profiles on the host.
type SeccompProfileFileController struct {
	V1Alpha1Mode             runtimetalos.Mode
	SeccompProfilesDirectory string
}

// Name implements controller.StatsController interface.
func (ctrl *SeccompProfileFileController) Name() string {
	return "cri.SeccompProfileFileController"
}

// Inputs implements controller.StatsController interface.
func (ctrl *SeccompProfileFileController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.StatsController interface.
func (ctrl *SeccompProfileFileController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.StatsController interface.
//
//nolint:gocyclo,cyclop
func (ctrl *SeccompProfileFileController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	// initially, wait for /var to be mounted
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: runtimeres.NamespaceName,
			Type:      runtimeres.MountStatusType,
			ID:        optional.Some(constants.EphemeralPartitionLabel),
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		_, err := safe.ReaderGet[*runtimeres.MountStatus](ctx, r, resource.NewMetadata(runtimeres.NamespaceName, runtimeres.MountStatusType, constants.EphemeralPartitionLabel, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				// in container mode EPHEMERAL is always mounted
				if ctrl.V1Alpha1Mode != runtimetalos.ModeContainer {
					// wait for the EPHEMERAL to be mounted
					continue
				}
			} else {
				return fmt.Errorf("error getting ephemeral mount status: %w", err)
			}
		}

		break
	}

	// normal reconcile loop
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: cri.NamespaceName,
			Type:      cri.SeccompProfileType,
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return err
	}

	r.QueueReconcile()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		list, err := safe.ReaderListAll[*cri.SeccompProfile](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing seccomp profiles: %w", err)
		}

		touchedIDs := make(map[string]struct{}, list.Len())

		for profile := range list.All() {
			profileName := profile.TypedSpec().Name
			profilePath := filepath.Join(ctrl.SeccompProfilesDirectory, profileName)

			profileContent, err := json.Marshal(profile.TypedSpec().Value)
			if err != nil {
				return fmt.Errorf("error marshaling seccomp profile: %w", err)
			}

			existingProfileContent, err := os.ReadFile(profilePath)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("error reading existing seccomp profile at %s: %w", profilePath, err)
				}

				if err := writeSeccompFile(profilePath, profileContent); err != nil {
					return err
				}
			} else {
				if val := bytes.Compare(existingProfileContent, profileContent); val != 0 {
					if err := writeSeccompFile(profilePath, profileContent); err != nil {
						return err
					}
				}
			}

			touchedIDs[profileName] = struct{}{}
		}

		// cleanup
		if err := filepath.WalkDir(ctrl.SeccompProfilesDirectory, func(path string, d fs.DirEntry, err error) error {
			fileName, errRel := filepath.Rel(ctrl.SeccompProfilesDirectory, path)
			if errRel != nil {
				return errRel
			}

			// ignore current folder
			if fileName != "." {
				if _, ok := touchedIDs[fileName]; !ok {
					if err := os.RemoveAll(path); err != nil {
						return err
					}
				}
			}

			return nil
		}); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func writeSeccompFile(path string, content []byte) error {
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("error writing seccomp profile at %s: %w", path, err)
	}

	return nil
}
