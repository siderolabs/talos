// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2_test

import (
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha2"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestVirtualMachineSpecRegistered(t *testing.T) {
	t.Parallel()

	state, err := v1alpha2.NewState()
	require.NoError(t, err)

	for _, resourceType := range []string{hypervisor.VirtualMachineSpecType, hypervisor.VirtualMachineDomainSpecType} {
		definition, getErr := safe.StateGetByID[*meta.ResourceDefinition](t.Context(), state.Resources(), strings.ToLower(resourceType))
		require.NoError(t, getErr)
		assert.Equal(t, hypervisor.NamespaceName, definition.TypedSpec().DefaultNamespace)
	}
}
