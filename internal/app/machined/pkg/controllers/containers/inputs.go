// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	timeres "github.com/siderolabs/talos/pkg/machinery/resources/time"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// containerCreationGateInputs returns the inputs needed to evaluate a container's gates, everything ContainerSpecSpec.Ready reads.
func containerCreationGateInputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerSpecType,
			Kind:      controller.InputWeak,
		},
		// Gates on the image having a usable digest.
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerImageStatusType,
			Kind:      controller.InputWeak,
		},
		// Gates on the declared mounts having been resolved.
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerMountStatusType,
			Kind:      controller.InputWeak,
		},
		// Gates on dependsOn.containers: another container's aggregated Health.
		{
			Namespace: containers.NamespaceName,
			Type:      containers.ContainerStatusType,
			Kind:      controller.InputWeak,
		},
		// Gate on dependsOn.networks.
		{
			Namespace: network.NamespaceName,
			Type:      network.StatusType,
			ID:        optional.Some(network.StatusID),
			Kind:      controller.InputWeak,
		},
		// Gate on dependsOn.time.
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      timeres.StatusType,
			ID:        optional.Some(timeres.StatusID),
			Kind:      controller.InputWeak,
		},
	}
}
