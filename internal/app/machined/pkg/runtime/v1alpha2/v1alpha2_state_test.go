// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha2_test

import (
	"strings"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha2"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestNewStateRegistersCPUPartitionResources(t *testing.T) {
	t.Parallel()

	s, err := v1alpha2.NewState()
	require.NoError(t, err)

	for _, test := range []struct {
		typ       resource.Type
		namespace resource.Namespace
	}{
		{runtime.CPUPartitionSpecType, runtime.NamespaceName},
		{runtime.CPUPartitionStatusType, runtime.NamespaceName},
		{k8s.KubeletCPUReservationType, k8s.NamespaceName},
		{hypervisor.VirtualMachineCPUPlacementType, hypervisor.NamespaceName},
	} {
		rd, err := safe.StateGetByID[*meta.ResourceDefinition](t.Context(), s.Resources(), strings.ToLower(test.typ))
		require.NoError(t, err, test.typ)

		assert.Equal(t, test.typ, rd.TypedSpec().Type)
		assert.Equal(t, test.namespace, rd.TypedSpec().DefaultNamespace)
	}
}
