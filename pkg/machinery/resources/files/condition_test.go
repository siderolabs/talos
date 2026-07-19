// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

func TestEtcFileConditionWaitsForEveryFile(t *testing.T) {
	t.Parallel()

	st := state.WrapCore(namespaced.NewState(inmem.Build))

	require.NoError(t, st.Create(t.Context(), files.NewEtcFileStatus(files.NamespaceName, constants.CRIConfig)))

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()

	err := files.NewEtcFileCondition(st, constants.CRIConfig, constants.CRIBaseRuntimeSpec).Wait(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	require.NoError(t, st.Create(t.Context(), files.NewEtcFileStatus(files.NamespaceName, constants.CRIBaseRuntimeSpec)))
	require.NoError(t, files.NewEtcFileCondition(st, constants.CRIConfig, constants.CRIBaseRuntimeSpec).Wait(t.Context()))
}
