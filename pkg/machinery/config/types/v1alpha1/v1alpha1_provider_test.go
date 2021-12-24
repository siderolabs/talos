// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

func TestInterfaces(t *testing.T) {
	t.Parallel()

	assert.Implements(t, (*config.APIServer)(nil), (*v1alpha1.APIServerConfig)(nil))
	assert.Implements(t, (*config.ClusterConfig)(nil), (*v1alpha1.ClusterConfig)(nil))
	assert.Implements(t, (*config.ClusterNetwork)(nil), (*v1alpha1.ClusterConfig)(nil))
	assert.Implements(t, (*config.ControllerManager)(nil), (*v1alpha1.ControllerManagerConfig)(nil))
	assert.Implements(t, (*config.Etcd)(nil), (*v1alpha1.EtcdConfig)(nil))
	assert.Implements(t, (*config.ExternalCloudProvider)(nil), (*v1alpha1.ExternalCloudProviderConfig)(nil))
	assert.Implements(t, (*config.Features)(nil), (*v1alpha1.FeaturesConfig)(nil))
	assert.Implements(t, (*config.MachineConfig)(nil), (*v1alpha1.MachineConfig)(nil))
	assert.Implements(t, (*config.Scheduler)(nil), (*v1alpha1.SchedulerConfig)(nil))
	assert.Implements(t, (*config.Token)(nil), (*v1alpha1.ClusterConfig)(nil))

	tok := new(v1alpha1.ClusterConfig).Token()
	assert.Implements(t, (*config.Token)(nil), (tok))
}
