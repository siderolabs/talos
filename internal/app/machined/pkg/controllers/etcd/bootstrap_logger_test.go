// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestBootstrapLoggerSuite(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.InfoLevel)

	s := &BootstrapLoggerSuite{logs: logs}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: time.Minute,
		Logger:  zap.New(core),
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&etcdctrl.BootstrapLoggerController{
				GracePeriod: 500 * time.Millisecond,
				Interval:    100 * time.Millisecond,
			}))
		},
	}

	suite.Run(t, s)
}

type BootstrapLoggerSuite struct {
	ctest.DefaultSuite

	logs *observer.ObservedLogs
}

func (suite *BootstrapLoggerSuite) reminders() []observer.LoggedEntry {
	return suite.logs.FilterMessageSnippet("please run `talosctl bootstrap`").All()
}

func (suite *BootstrapLoggerSuite) setPhase(phase etcd.MembershipPhase) {
	status := etcd.NewMembershipStatus()

	if _, err := suite.State().Get(suite.Ctx(), status.Metadata()); err != nil {
		status.TypedSpec().Phase = phase
		suite.Create(status)

		return
	}

	ctest.UpdateWithConflicts(suite, status, func(status *etcd.MembershipStatus) error {
		status.TypedSpec().Phase = phase

		return nil
	})
}

func (suite *BootstrapLoggerSuite) TestReminder() {
	addresses := network.NewNodeAddress(network.NamespaceName, network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s))
	addresses.TypedSpec().Addresses = []netip.Prefix{netip.MustParsePrefix("10.0.0.1/24")}
	suite.Create(addresses)

	// the machine is given some time to join the cluster
	suite.setPhase(etcd.MembershipPhaseJoining)

	time.Sleep(200 * time.Millisecond)

	suite.Empty(suite.reminders())

	// the reminders are repeated while the machine is waiting to join the cluster
	suite.EventuallyWithT(func(collect *assert.CollectT) {
		reminders := suite.reminders()

		if assert.GreaterOrEqual(collect, len(reminders), 2) {
			assert.Equal(collect, []any{"10.0.0.1"}, reminders[0].ContextMap()["ips"])
		}
	}, 10*time.Second, 50*time.Millisecond)

	// no reminders once the machine is not waiting anymore
	suite.setPhase(etcd.MembershipPhaseMember)

	time.Sleep(200 * time.Millisecond)

	count := len(suite.reminders())

	time.Sleep(300 * time.Millisecond)

	suite.Len(suite.reminders(), count)
}
