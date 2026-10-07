// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestCloudInitSecretRedactionAndIdentity(t *testing.T) {
	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest")
	vm.TypedSpec().CloudInit = &hypervisor.VirtualMachineCloudInitSpec{
		Library:       "images",
		MetaData:      "instance-id: 1\n",
		UserData:      "secret",
		NetworkConfig: "version: 2\n",
	}

	redacted := vm.TypedSpec().RedactSecrets(*vm.Metadata())
	require.Equal(t, constants.Redacted, redacted.CloudInit.UserData)
	require.Equal(t, "secret", vm.TypedSpec().CloudInit.UserData)

	seed := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "guest")
	seed.TypedSpec().Library = "images"
	seed.TypedSpec().MetaData = "metadata-secret"
	seed.TypedSpec().UserData = "secret"
	seed.TypedSpec().NetworkConfig = "network-secret"

	redactedSeed := seed.TypedSpec().RedactSecrets(*seed.Metadata())
	require.Equal(t, constants.Redacted, redactedSeed.MetaData)
	require.Equal(t, constants.Redacted, redactedSeed.UserData)
	require.Equal(t, constants.Redacted, redactedSeed.NetworkConfig)
	require.Equal(t, "images", redactedSeed.Library)
	require.Equal(t, "metadata-secret", seed.TypedSpec().MetaData)
	require.Equal(t, "network-secret", seed.TypedSpec().NetworkConfig)
	require.Equal(t, constants.Redacted, redacted.CloudInit.MetaData)
	require.Equal(t, constants.Redacted, redacted.CloudInit.NetworkConfig)
	require.Equal(t, "instance-id: 1\n", vm.TypedSpec().CloudInit.MetaData)
	require.Equal(t, "version: 2\n", vm.TypedSpec().CloudInit.NetworkConfig)
	require.Equal(t, "secret", seed.TypedSpec().UserData)
	require.NotEmpty(t, hypervisor.CloudInitStatusID("guest", *seed.TypedSpec()))
}
