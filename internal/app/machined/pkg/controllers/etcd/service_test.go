// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/etcd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// fakeServiceManager records the operations on the services.
type fakeServiceManager struct {
	mu sync.Mutex

	loaded  bool
	running bool

	ops []string
}

func (m *fakeServiceManager) IsRunning(string) (system.Service, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.loaded {
		return nil, false, errors.New("service not found")
	}

	return nil, m.running, nil
}

func (m *fakeServiceManager) Load(...system.Service) []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.loaded = true
	m.ops = append(m.ops, "load")

	return []string{"etcd"}
}

func (m *fakeServiceManager) Stop(context.Context, ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.running = false
	m.ops = append(m.ops, "stop")

	return nil
}

func (m *fakeServiceManager) Start(...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.running = true
	m.ops = append(m.ops, "start")

	return nil
}

func (m *fakeServiceManager) operations() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return slices.Clone(m.ops)
}

func TestServiceSuite(t *testing.T) {
	t.Parallel()

	s := &ServiceSuite{}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: time.Minute,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			s.services = &fakeServiceManager{}

			suite.Require().NoError(suite.Runtime().RegisterController(&etcdctrl.ServiceController{
				V1Alpha1Services: s.services,
			}))
		},
	}

	suite.Run(t, s)
}

type ServiceSuite struct {
	ctest.DefaultSuite

	services *fakeServiceManager
}

func (suite *ServiceSuite) assertOperations(expected ...string) {
	suite.EventuallyWithT(func(collect *assert.CollectT) {
		assert.Equal(collect, expected, suite.services.operations())
	}, 10*time.Second, 50*time.Millisecond)
}

func (suite *ServiceSuite) createSpec() *etcd.Spec {
	spec := etcd.NewSpec(etcd.NamespaceName, etcd.SpecID)
	spec.TypedSpec().Name = "node1"
	spec.TypedSpec().AdvertisedAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.1")}
	spec.TypedSpec().ListenPeerAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.1")}
	spec.TypedSpec().ListenClientAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.1")}
	spec.TypedSpec().Image = "etcd:v1"
	suite.Create(spec)

	suite.Create(etcd.NewPKIStatus(etcd.NamespaceName, etcd.PKIID))

	return spec
}

func (suite *ServiceSuite) createLaunchSpec() *etcd.LaunchSpec {
	launch := etcd.NewLaunchSpec()
	launch.TypedSpec().InitialClusterState = "new"
	launch.TypedSpec().InitialCluster = "node1=https://10.0.0.1:2380"
	suite.Create(launch)

	return launch
}

func (suite *ServiceSuite) setPhase(phase etcd.MembershipPhase) {
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

func (suite *ServiceSuite) TestStartStop() {
	suite.createSpec()

	// etcd is not started without the launch spec
	ctest.AssertNoResource[*etcd.ServiceSpec](suite, etcd.ServiceSpecID)
	suite.Empty(suite.services.operations())

	launch := suite.createLaunchSpec()

	ctest.AssertResource(suite, etcd.ServiceSpecID, func(spec *etcd.ServiceSpec, asrt *assert.Assertions) {
		asrt.Equal("etcd:v1", spec.TypedSpec().Image)
		asrt.Contains(spec.TypedSpec().Args, "--name=node1")
		asrt.Contains(spec.TypedSpec().Args, "--initial-cluster-state=new")
		asrt.Contains(spec.TypedSpec().Args, "--initial-cluster=node1=https://10.0.0.1:2380")
	})
	suite.assertOperations("load", "start")

	// etcd is stopped once the launch spec is gone
	suite.Destroy(launch)

	ctest.AssertNoResource[*etcd.ServiceSpec](suite, etcd.ServiceSpecID)
	suite.assertOperations("load", "start", "stop")

	// and it is started back
	suite.createLaunchSpec()

	ctest.AssertResource(suite, etcd.ServiceSpecID, func(*etcd.ServiceSpec, *assert.Assertions) {})
	suite.assertOperations("load", "start", "stop", "start")
}

func (suite *ServiceSuite) TestRestartOnChange() {
	spec := suite.createSpec()
	suite.createLaunchSpec()
	suite.setPhase(etcd.MembershipPhaseMember)

	suite.assertOperations("load", "start")

	// etcd reports healthy after the restart
	service := v1alpha1.NewService("etcd")
	service.TypedSpec().Running = true
	service.TypedSpec().Healthy = true
	suite.Create(service)

	ctest.UpdateWithConflicts(suite, spec, func(spec *etcd.Spec) error {
		spec.TypedSpec().Image = "etcd:v2"
		spec.TypedSpec().ExtraArgs = map[string]etcd.ArgValues{
			"election-timeout": {Values: []string{"5000"}},
		}

		return nil
	})

	ctest.AssertResource(suite, etcd.ServiceSpecID, func(spec *etcd.ServiceSpec, asrt *assert.Assertions) {
		asrt.Equal("etcd:v2", spec.TypedSpec().Image)
		asrt.Contains(spec.TypedSpec().Args, "--election-timeout=5000")
	})

	// etcd is restarted right away
	suite.assertOperations("load", "start", "stop", "start")
}

func (suite *ServiceSuite) TestNoRestartOnAdvertisedAddressChange() {
	spec := suite.createSpec()
	suite.createLaunchSpec()
	suite.setPhase(etcd.MembershipPhaseMember)

	suite.assertOperations("load", "start")

	var initialArgs []string

	ctest.AssertResource(suite, etcd.ServiceSpecID, func(spec *etcd.ServiceSpec, asrt *assert.Assertions) {
		initialArgs = spec.TypedSpec().Args
	})

	// the machine gets a new address
	ctest.UpdateWithConflicts(suite, spec, func(spec *etcd.Spec) error {
		spec.TypedSpec().AdvertisedAddresses = append(spec.TypedSpec().AdvertisedAddresses, netip.MustParseAddr("fd00::1"))

		return nil
	})

	// etcd is not restarted, and the service spec still describes what etcd is running with
	time.Sleep(500 * time.Millisecond)

	ctest.AssertResource(suite, etcd.ServiceSpecID, func(spec *etcd.ServiceSpec, asrt *assert.Assertions) {
		asrt.Equal(initialArgs, spec.TypedSpec().Args)
	})
	suite.assertOperations("load", "start")

	// the new address is picked up on the restart for another reason
	ctest.UpdateWithConflicts(suite, spec, func(spec *etcd.Spec) error {
		spec.TypedSpec().Image = "etcd:v2"

		return nil
	})

	ctest.AssertResource(suite, etcd.ServiceSpecID, func(spec *etcd.ServiceSpec, asrt *assert.Assertions) {
		asrt.Equal("etcd:v2", spec.TypedSpec().Image)
		asrt.True(slices.ContainsFunc(spec.TypedSpec().Args, func(arg string) bool {
			return strings.HasPrefix(arg, "--advertise-client-urls=") && strings.Contains(arg, "[fd00::1]")
		}))
	})
}

func (suite *ServiceSuite) TestRestartLearnerNotRunning() {
	spec := suite.createSpec()
	suite.createLaunchSpec()
	suite.setPhase(etcd.MembershipPhaseLearner)

	suite.assertOperations("load", "start")

	// etcd failed to start (e.g. the image can't be pulled), so the fixed spec is applied right away
	ctest.UpdateWithConflicts(suite, spec, func(spec *etcd.Spec) error {
		spec.TypedSpec().Image = "etcd:v2"

		return nil
	})

	ctest.AssertResource(suite, etcd.ServiceSpecID, func(spec *etcd.ServiceSpec, asrt *assert.Assertions) {
		asrt.Equal("etcd:v2", spec.TypedSpec().Image)
	})

	suite.assertOperations("load", "start", "stop", "start")
}

func (suite *ServiceSuite) TestRestartDeferredForLearner() {
	spec := suite.createSpec()
	suite.createLaunchSpec()
	suite.setPhase(etcd.MembershipPhaseLearner)

	suite.assertOperations("load", "start")

	service := v1alpha1.NewService("etcd")
	service.TypedSpec().Running = true
	service.TypedSpec().Healthy = true
	suite.Create(service)

	// make sure the controller has seen etcd running
	time.Sleep(200 * time.Millisecond)

	ctest.UpdateWithConflicts(suite, spec, func(spec *etcd.Spec) error {
		spec.TypedSpec().Image = "etcd:v2"

		return nil
	})

	// a running learner is never restarted
	time.Sleep(500 * time.Millisecond)

	ctest.AssertResource(suite, etcd.ServiceSpecID, func(spec *etcd.ServiceSpec, asrt *assert.Assertions) {
		asrt.Equal("etcd:v1", spec.TypedSpec().Image)
	})
	suite.assertOperations("load", "start")

	// the restart goes through once the learner is promoted
	suite.setPhase(etcd.MembershipPhaseMember)

	ctest.AssertResource(suite, etcd.ServiceSpecID, func(spec *etcd.ServiceSpec, asrt *assert.Assertions) {
		asrt.Equal("etcd:v2", spec.TypedSpec().Image)
	})
	suite.assertOperations("load", "start", "stop", "start")
}
