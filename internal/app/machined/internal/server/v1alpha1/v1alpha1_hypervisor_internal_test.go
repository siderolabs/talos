// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"errors"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
)

func TestAssertConfigSettled(t *testing.T) {
	t.Parallel()

	live := container.NewV1Alpha1(&v1alpha1.Config{ConfigVersion: "v1alpha1"})
	staged := container.NewV1Alpha1(&v1alpha1.Config{ConfigVersion: "v1alpha1", ConfigDebug: new(true)})

	for name, test := range map[string]struct {
		persisted       *container.Container
		rollbackPending bool
		code            codes.Code
	}{
		// Loaded from the STATE partition, nothing applied since.
		"nothing persisted": {code: codes.OK},
		"persisted is live": {persisted: live, code: codes.OK},
		"staged":            {persisted: staged, code: codes.FailedPrecondition},
		// A try applied over a configuration loaded from the STATE partition persists nothing,
		// so only the rollback timer tells it apart.
		"try pending": {rollbackPending: true, code: codes.FailedPrecondition},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			st := state.WrapCore(namespaced.NewState(inmem.Build))

			require.NoError(t, st.Create(t.Context(), configres.NewMachineConfigWithID(live, configres.ActiveID)))

			if test.persisted != nil {
				require.NoError(t, st.Create(t.Context(), configres.NewMachineConfigWithID(test.persisted, configres.PersistentID)))
			}

			err := assertConfigSettled(t.Context(), st, test.rollbackPending)
			require.Equal(t, test.code, status.Code(err), "%v", err)
		})
	}
}

func TestCommitConfigRestoresPersistedWhenActivationFails(t *testing.T) {
	t.Parallel()

	previous := container.NewV1Alpha1(&v1alpha1.Config{ConfigVersion: "v1alpha1"})
	next := container.NewV1Alpha1(&v1alpha1.Config{ConfigVersion: "v1alpha1", ConfigDebug: new(true)})

	var persisted, active []config.Provider

	setPersisted := func(cfg config.Provider) error {
		persisted = append(persisted, cfg)

		return nil
	}

	activationErr := errors.New("activation failed")

	setActive := func(cfg config.Provider) error {
		active = append(active, cfg)

		return activationErr
	}

	require.ErrorIs(t, commitConfig(previous, next, setPersisted, setActive), activationErr)
	require.Equal(t, []config.Provider{next, previous}, persisted, "the failed request is not left for the next boot")
	require.Equal(t, []config.Provider{next}, active)

	persisted, active = nil, nil

	require.NoError(t, commitConfig(previous, next, setPersisted, func(cfg config.Provider) error {
		active = append(active, cfg)

		return nil
	}))
	require.Equal(t, []config.Provider{next}, persisted)
	require.Equal(t, []config.Provider{next}, active)
}
