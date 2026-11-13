// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/etcd"
	etcdcli "github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

const (
	testLearnerID       = uint64(0x0102030405060708)
	testLearnerIDString = "0102030405060708"
	testMemberIDString  = "a1a2a3a4a5a6a7a8"
)

// fakeCluster is a fake etcd cluster for the MembershipController.
type fakeCluster struct {
	mu sync.Mutex

	joinErr    error
	promoteErr error
	removeErr  error

	memberStates map[uint64]etcdcli.MemberState

	joins    int
	removed  []uint64
	promoted []uint64
}

func (c *fakeCluster) set(f func(c *fakeCluster)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	f(c)
}

func (c *fakeCluster) JoinAsLearner(context.Context, *etcd.ClientSpecSpec, string, []string) (etcdcli.JoinResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.joinErr != nil {
		return etcdcli.JoinResult{}, c.joinErr
	}

	c.joins++

	return etcdcli.JoinResult{
		InitialCluster:        "node0=https://10.0.0.0:2380,node1=https://10.0.0.1:2380",
		LearnerMemberID:       testLearnerID,
		VotingMemberEndpoints: []string{"https://10.0.0.0:2379"},
	}, nil
}

func (c *fakeCluster) GetMemberState(_ context.Context, _ *etcd.ClientSpecSpec, memberID uint64) (etcdcli.MemberState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.memberStates[memberID], nil
}

func (c *fakeCluster) RemoveMember(_ context.Context, _ *etcd.ClientSpecSpec, memberID uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.removeErr != nil {
		return c.removeErr
	}

	c.removed = append(c.removed, memberID)

	return nil
}

func (c *fakeCluster) PromoteLearner(_ context.Context, _ *etcd.ClientSpecSpec, endpoints []string, memberID uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.promoteErr != nil {
		return c.promoteErr
	}

	if !slices.Equal(endpoints, []string{"10.0.0.0:2379"}) {
		return errors.New("unexpected promotion endpoints")
	}

	c.promoted = append(c.promoted, memberID)

	return nil
}

func TestMembershipSuite(t *testing.T) {
	t.Parallel()

	s := &MembershipSuite{}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: time.Minute,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			s.cluster = &fakeCluster{}
			s.dataDir = s.T().TempDir()

			// the timeout is short only for the test which expects it to expire, so that the other tests don't race with it
			learnerStartTimeout := time.Hour

			if strings.HasSuffix(s.T().Name(), "/TestLearnerNotStarted") {
				learnerStartTimeout = time.Second
			}

			suite.Require().NoError(suite.Runtime().RegisterController(&etcdctrl.MembershipController{
				Cluster:             s.cluster,
				SnapshotPath:        filepath.Join(s.T().TempDir(), "missing.snapshot"),
				LearnerStartTimeout: learnerStartTimeout,
			}))
		},
	}

	suite.Run(t, s)
}

type MembershipSuite struct {
	ctest.DefaultSuite

	cluster *fakeCluster
	dataDir string
}

// createInputs creates the resources required for the controller to start operating.
func (suite *MembershipSuite) createInputs(modify ...func(*etcd.MembershipSpecSpec)) {
	spec := etcd.NewMembershipSpec()
	spec.TypedSpec().Name = "node1"
	spec.TypedSpec().AdvertisedAddresses = []netip.Addr{netip.MustParseAddr("10.0.0.1")}

	for _, f := range modify {
		f(spec.TypedSpec())
	}

	suite.Create(spec)

	suite.Create(etcd.NewClientSpec())

	// the controller requests the volume to be mounted
	ctest.AssertResources(suite,
		[]string{"etcd.MembershipController-/var/lib", "etcd.MembershipController-" + constants.EtcdDataVolumeID},
		func(*block.VolumeMountRequest, *assert.Assertions) {},
	)

	mountStatus := block.NewVolumeMountStatus(block.NamespaceName, "etcd.MembershipController-"+constants.EtcdDataVolumeID)
	mountStatus.TypedSpec().Target = suite.dataDir
	suite.Create(mountStatus)
}

func (suite *MembershipSuite) createMemberData() {
	suite.Require().NoError(os.MkdirAll(filepath.Join(suite.dataDir, "member", "wal"), 0o700))
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.dataDir, "member", "wal", "0.wal"), []byte("wal"), 0o600))
}

func (suite *MembershipSuite) assertStatus(phase etcd.MembershipPhase, memberID string) {
	ctest.AssertResource(suite, etcd.MembershipStatusID, func(status *etcd.MembershipStatus, asrt *assert.Assertions) {
		asrt.Equal(phase, status.TypedSpec().Phase)
		asrt.Equal(memberID, status.TypedSpec().MemberID)
	})
}

func (suite *MembershipSuite) assertLaunch(initialClusterState, initialCluster string) {
	ctest.AssertResource(suite, etcd.LaunchSpecID, func(launch *etcd.LaunchSpec, asrt *assert.Assertions) {
		asrt.Equal(initialClusterState, launch.TypedSpec().InitialClusterState)
		asrt.Equal(initialCluster, launch.TypedSpec().InitialCluster)
	})
}

func (suite *MembershipSuite) assertDataDir(expected ...string) {
	suite.EventuallyWithT(func(collect *assert.CollectT) {
		entries, err := os.ReadDir(suite.dataDir)
		if !assert.NoError(collect, err) {
			return
		}

		names := make([]string, 0, len(entries))

		for _, entry := range entries {
			names = append(names, entry.Name())
		}

		assert.ElementsMatch(collect, expected, names)
	}, 10*time.Second, 50*time.Millisecond)
}

func (suite *MembershipSuite) TestNotReady() {
	// no etcd membership spec: etcd is not configured
	ctest.AssertNoResource[*etcd.MembershipStatus](suite, etcd.MembershipStatusID)

	spec := etcd.NewMembershipSpec()
	suite.Create(spec)

	// the volume is not mounted yet
	suite.assertStatus(etcd.MembershipPhasePreparing, "")
	ctest.AssertNoResource[*etcd.LaunchSpec](suite, etcd.LaunchSpecID)

	// the volume is mounted, but there is no way to talk to the etcd cluster yet
	mountStatus := block.NewVolumeMountStatus(block.NamespaceName, "etcd.MembershipController-"+constants.EtcdDataVolumeID)
	mountStatus.TypedSpec().Target = suite.dataDir
	suite.Create(mountStatus)

	ctest.AssertResource(suite, mountStatus.Metadata().ID(), func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has("etcd.MembershipController"))
	})

	suite.assertStatus(etcd.MembershipPhasePreparing, "")
	ctest.AssertNoResource[*etcd.LaunchSpec](suite, etcd.LaunchSpecID)

	suite.Zero(suite.cluster.joins)

	// the volume is still being used
	const holder = "holder"

	varLibRequest := block.NewVolumeMountRequest(block.NamespaceName, "etcd.MembershipController-/var/lib").Metadata()
	suite.Require().NoError(suite.State().AddFinalizer(suite.Ctx(), varLibRequest, holder))

	// etcd is not configured anymore
	suite.Destroy(spec)

	ctest.AssertNoResource[*etcd.MembershipStatus](suite, etcd.MembershipStatusID)

	// the volume is released
	ctest.AssertResource(suite, mountStatus.Metadata().ID(), func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Empty())
	})

	// the request is not destroyed while it is held
	ctest.AssertNoResource[*block.VolumeMountRequest](suite, "etcd.MembershipController-"+constants.EtcdDataVolumeID)
	ctest.AssertResource(suite, varLibRequest.ID(), func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, request.Metadata().Phase())
	})

	// the request is destroyed as soon as it is released, without any other change to the inputs of the controller
	suite.Require().NoError(suite.State().RemoveFinalizer(suite.Ctx(), varLibRequest, holder))

	ctest.AssertNoResources[*block.VolumeMountRequest](suite,
		[]string{"etcd.MembershipController-/var/lib", "etcd.MembershipController-" + constants.EtcdDataVolumeID},
	)
}

func (suite *MembershipSuite) TestExistingData() {
	suite.createMemberData()
	suite.createInputs()

	suite.assertLaunch("existing", "")
	suite.assertStatus(etcd.MembershipPhaseMember, "")

	// the member ID is picked up once etcd is healthy
	member := etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID)
	member.TypedSpec().MemberID = testMemberIDString
	suite.Create(member)

	suite.assertStatus(etcd.MembershipPhaseMember, testMemberIDString)

	suite.Zero(suite.cluster.joins)
}

func (suite *MembershipSuite) TestBootstrap() {
	suite.cluster.joinErr = errors.New("no cluster")

	suite.createInputs()

	// the machine keeps trying to join the cluster
	ctest.AssertResource(suite, etcd.MembershipStatusID, func(status *etcd.MembershipStatus, asrt *assert.Assertions) {
		asrt.Equal(etcd.MembershipPhaseJoining, status.TypedSpec().Phase)
		asrt.Equal("no cluster", status.TypedSpec().Error)
	})
	ctest.AssertNoResource[*etcd.LaunchSpec](suite, etcd.LaunchSpecID)

	suite.Create(etcd.NewBootstrapRequest())

	suite.assertLaunch("new", "node1=https://10.0.0.1:2380")
	suite.assertStatus(etcd.MembershipPhaseMember, "")
	suite.assertDataDir()

	// the peer URLs are fixed along with the initial cluster
	ctest.AssertResource(suite, etcd.LaunchSpecID, func(launch *etcd.LaunchSpec, asrt *assert.Assertions) {
		asrt.Equal([]string{"https://10.0.0.1:2380"}, launch.TypedSpec().InitialAdvertisePeerURLs)
	})
}

func (suite *MembershipSuite) TestBootstrapRecoverFailed() {
	suite.cluster.joinErr = errors.New("no cluster")

	suite.createInputs()

	suite.assertStatus(etcd.MembershipPhaseJoining, "")

	// the snapshot is missing
	request := etcd.NewBootstrapRequest()
	request.TypedSpec().RecoverFromSnapshot = true
	suite.Create(request)

	// the recovery is not retried: the request is done with, and the failure is reported
	suite.assertBootstrapRequestConsumed()

	ctest.AssertResource(suite, etcd.MembershipStatusID, func(status *etcd.MembershipStatus, asrt *assert.Assertions) {
		asrt.Equal(etcd.MembershipPhaseJoining, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Error, "failed to recover etcd from the snapshot")
		asrt.Contains(status.TypedSpec().Error, "no cluster")
	})
	ctest.AssertNoResource[*etcd.LaunchSpec](suite, etcd.LaunchSpecID)
	suite.assertDataDir()

	// the bootstrap is requested once again
	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), request.Metadata()))
	suite.Create(etcd.NewBootstrapRequest())

	suite.assertLaunch("new", "node1=https://10.0.0.1:2380")
	suite.assertStatus(etcd.MembershipPhaseMember, "")
}

func (suite *MembershipSuite) TestBootstrapIsOneShot() {
	suite.createInputs()

	suite.assertStatus(etcd.MembershipPhaseLearner, testLearnerIDString)

	// the bootstrap request is ignored, as the machine has joined the cluster already
	suite.Create(etcd.NewBootstrapRequest())

	suite.assertBootstrapRequestConsumed()

	suite.Create(v1alpha1.NewService("etcd"))

	suite.assertStatus(etcd.MembershipPhaseMember, testLearnerIDString)
	suite.assertLaunch("existing", "node0=https://10.0.0.0:2380,node1=https://10.0.0.1:2380")
}

func (suite *MembershipSuite) assertBootstrapRequestConsumed() {
	ctest.AssertResource(suite, etcd.BootstrapRequestID, func(request *etcd.BootstrapRequest, asrt *assert.Assertions) {
		asrt.True(request.Metadata().Finalizers().Has("etcd.MembershipController"))
	})
}

func (suite *MembershipSuite) TestBootstrapRequestStale() {
	suite.cluster.promoteErr = errors.New("learner is not in sync")
	suite.cluster.memberStates = map[uint64]etcdcli.MemberState{testLearnerID: etcdcli.MemberStateLearner}

	suite.createInputs()

	suite.assertStatus(etcd.MembershipPhaseLearner, testLearnerIDString)

	// the bootstrap request is too late, as the machine has joined the cluster already
	request := etcd.NewBootstrapRequest()
	suite.Create(request)

	suite.assertBootstrapRequestConsumed()

	// etcd is started by the service controller, and it crashes while being a learner
	serviceSpec := etcd.NewServiceSpec()
	suite.Create(serviceSpec)

	service := v1alpha1.NewService("etcd")
	suite.Create(service)

	ctest.AssertResource(suite, etcd.MembershipStatusID, func(status *etcd.MembershipStatus, asrt *assert.Assertions) {
		asrt.Equal("learner is not in sync", status.TypedSpec().Error)
	})

	suite.cluster.set(func(c *fakeCluster) { c.joinErr = errors.New("no cluster") })

	suite.Destroy(service)
	ctest.AssertNoResource[*etcd.LaunchSpec](suite, etcd.LaunchSpecID)
	suite.Destroy(serviceSpec)

	// the etcd data is wiped, but the stale bootstrap request doesn't bootstrap a new cluster
	ctest.AssertResource(suite, etcd.MembershipStatusID, func(status *etcd.MembershipStatus, asrt *assert.Assertions) {
		asrt.Equal(etcd.MembershipPhaseJoining, status.TypedSpec().Phase)
		asrt.Equal("no cluster", status.TypedSpec().Error)
	})
	suite.assertDataDir()
	ctest.AssertNoResource[*etcd.LaunchSpec](suite, etcd.LaunchSpecID)

	// a new bootstrap request replaces the stale one: the controller releases the stale request
	suite.Require().NoError(suite.State().TeardownAndDestroy(suite.Ctx(), request.Metadata()))
	suite.Create(etcd.NewBootstrapRequest())

	suite.assertLaunch("new", "node1=https://10.0.0.1:2380")
	suite.assertStatus(etcd.MembershipPhaseMember, "")
	suite.assertBootstrapRequestConsumed()
}

func (suite *MembershipSuite) TestAutoBootstrap() {
	suite.createInputs(func(spec *etcd.MembershipSpecSpec) {
		spec.InitNode = true
		spec.AutoBootstrap = true
	})

	// machine of type init bootstraps the cluster on its own
	suite.assertLaunch("new", "node1=https://10.0.0.1:2380")
	suite.assertStatus(etcd.MembershipPhaseMember, "")

	suite.Zero(suite.cluster.joins)
}

func (suite *MembershipSuite) TestInitUpgraded() {
	suite.createInputs(func(spec *etcd.MembershipSpecSpec) {
		spec.InitNode = true
	})

	// machine of type init joins the cluster back if it lost the data on upgrade
	suite.assertLaunch("existing", "node0=https://10.0.0.0:2380,node1=https://10.0.0.1:2380")
	suite.assertStatus(etcd.MembershipPhaseLearner, testLearnerIDString)
}

func (suite *MembershipSuite) TestManualInitialCluster() {
	suite.createInputs(func(spec *etcd.MembershipSpecSpec) {
		spec.ManualInitialCluster = true
	})

	// the cluster membership is managed outside of Talos
	suite.assertLaunch("existing", "")
	suite.assertStatus(etcd.MembershipPhaseMember, "")

	suite.Zero(suite.cluster.joins)
}

func (suite *MembershipSuite) TestManualInitialClusterInit() {
	suite.createInputs(func(spec *etcd.MembershipSpecSpec) {
		spec.ManualInitialCluster = true
		spec.InitNode = true
	})

	// machine of type init starts the cluster which is managed outside of Talos
	suite.assertLaunch("new", "")
	suite.assertStatus(etcd.MembershipPhaseMember, "")

	suite.Zero(suite.cluster.joins)
}

func (suite *MembershipSuite) TestJoinAndPromote() {
	suite.cluster.promoteErr = errors.New("learner is not in sync")

	suite.createInputs()

	suite.assertLaunch("existing", "node0=https://10.0.0.0:2380,node1=https://10.0.0.1:2380")
	suite.assertStatus(etcd.MembershipPhaseLearner, testLearnerIDString)

	// the data is marked as disposable
	suite.assertDataDir(etcdcli.LearnerMarkerFilename)

	// etcd is running, but it is still catching up
	suite.Create(v1alpha1.NewService("etcd"))

	ctest.AssertResource(suite, etcd.MembershipStatusID, func(status *etcd.MembershipStatus, asrt *assert.Assertions) {
		asrt.Equal(etcd.MembershipPhaseLearner, status.TypedSpec().Phase)
		asrt.Equal("learner is not in sync", status.TypedSpec().Error)
	})

	suite.cluster.set(func(c *fakeCluster) { c.promoteErr = nil })

	suite.assertStatus(etcd.MembershipPhaseMember, testLearnerIDString)

	// voting member data is not disposable
	suite.assertDataDir()

	suite.cluster.set(func(c *fakeCluster) {
		suite.Equal([]uint64{testLearnerID}, c.promoted)
		suite.Equal(1, c.joins)
		suite.Empty(c.removed)
	})
}

func (suite *MembershipSuite) TestLearnerInterrupted() {
	suite.cluster.promoteErr = errors.New("learner is not in sync")
	suite.cluster.memberStates = map[uint64]etcdcli.MemberState{testLearnerID: etcdcli.MemberStateLearner}

	suite.createInputs()

	suite.assertStatus(etcd.MembershipPhaseLearner, testLearnerIDString)

	// etcd is started by the service controller
	serviceSpec := etcd.NewServiceSpec()
	suite.Create(serviceSpec)

	service := v1alpha1.NewService("etcd")
	suite.Create(service)

	suite.createMemberData()

	ctest.AssertResource(suite, etcd.MembershipStatusID, func(status *etcd.MembershipStatus, asrt *assert.Assertions) {
		asrt.Equal("learner is not in sync", status.TypedSpec().Error)
	})

	// etcd crashes (or gets restarted)
	suite.Destroy(service)

	// etcd should be stopped
	ctest.AssertNoResource[*etcd.LaunchSpec](suite, etcd.LaunchSpecID)
	suite.assertStatus(etcd.MembershipPhaseJoining, testLearnerIDString)

	// the data is not touched until etcd is stopped
	suite.assertDataDir("member", etcdcli.LearnerMarkerFilename)

	suite.cluster.set(func(c *fakeCluster) { suite.Empty(c.removed) })

	// the service controller stops etcd
	suite.Destroy(serviceSpec)

	// the learner is removed, the data is wiped, and the machine joins the cluster once again
	suite.assertLaunch("existing", "node0=https://10.0.0.0:2380,node1=https://10.0.0.1:2380")
	suite.assertStatus(etcd.MembershipPhaseLearner, testLearnerIDString)
	suite.assertDataDir(etcdcli.LearnerMarkerFilename)

	suite.cluster.set(func(c *fakeCluster) {
		suite.Equal([]uint64{testLearnerID}, c.removed)
		suite.Equal(2, c.joins)
	})
}

func (suite *MembershipSuite) TestLearnerNotStarted() {
	suite.cluster.memberStates = map[uint64]etcdcli.MemberState{testLearnerID: etcdcli.MemberStateLearner}

	suite.createInputs()

	suite.assertStatus(etcd.MembershipPhaseLearner, testLearnerIDString)

	// the service controller attempts to start etcd, but it never comes up
	serviceSpec := etcd.NewServiceSpec()
	suite.Create(serviceSpec)

	// the controller gives up on the learner
	ctest.AssertNoResource[*etcd.LaunchSpec](suite, etcd.LaunchSpecID)

	ctest.AssertResource(suite, etcd.MembershipStatusID, func(status *etcd.MembershipStatus, asrt *assert.Assertions) {
		asrt.Equal(etcd.MembershipPhaseJoining, status.TypedSpec().Phase)
		asrt.Equal("etcd failed to start as a learner", status.TypedSpec().Error)
	})

	// the service controller stops etcd
	suite.Destroy(serviceSpec)

	// the learner is removed from the cluster, and the machine joins the cluster once again
	suite.assertStatus(etcd.MembershipPhaseLearner, testLearnerIDString)
	suite.assertDataDir(etcdcli.LearnerMarkerFilename)

	suite.cluster.set(func(c *fakeCluster) {
		suite.Equal([]uint64{testLearnerID}, c.removed)
		suite.Equal(2, c.joins)
	})
}

func (suite *MembershipSuite) TestLearnerMarkerOnStart() {
	// the machine was restarted while etcd was a learner
	suite.createMemberData()
	suite.Require().NoError(etcdcli.NewDataDir(suite.dataDir).WriteLearnerMarker(testLearnerID))

	suite.cluster.joinErr = errors.New("not yet")
	suite.cluster.memberStates = map[uint64]etcdcli.MemberState{testLearnerID: etcdcli.MemberStateLearner}

	suite.createInputs()

	// the learner is removed, and the data is wiped
	suite.assertDataDir()
	suite.assertStatus(etcd.MembershipPhaseJoining, "")

	suite.cluster.set(func(c *fakeCluster) {
		suite.Equal([]uint64{testLearnerID}, c.removed)

		c.joinErr = nil
	})

	suite.assertStatus(etcd.MembershipPhaseLearner, testLearnerIDString)
}

func (suite *MembershipSuite) TestLearnerMarkerPromoted() {
	// the machine was restarted right after the promotion, but before the marker was removed
	suite.createMemberData()
	suite.Require().NoError(etcdcli.NewDataDir(suite.dataDir).WriteLearnerMarker(testLearnerID))

	suite.cluster.memberStates = map[uint64]etcdcli.MemberState{testLearnerID: etcdcli.MemberStateVoting}

	suite.createInputs()

	// the data of a voting member is kept
	suite.assertLaunch("existing", "")
	suite.assertStatus(etcd.MembershipPhaseMember, "")
	suite.assertDataDir("member")

	suite.cluster.set(func(c *fakeCluster) {
		suite.Empty(c.removed)
		suite.Zero(c.joins)
	})
}

func (suite *MembershipSuite) TestLeave() {
	suite.createMemberData()
	suite.createInputs()

	member := etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID)
	member.TypedSpec().MemberID = testMemberIDString
	suite.Create(member)

	suite.assertStatus(etcd.MembershipPhaseMember, testMemberIDString)

	// etcd is started by the service controller
	serviceSpec := etcd.NewServiceSpec()
	suite.Create(serviceSpec)

	suite.Create(etcd.NewLeaveRequest())

	// the member is removed from the cluster, and etcd should be stopped
	ctest.AssertNoResource[*etcd.LaunchSpec](suite, etcd.LaunchSpecID)
	suite.assertStatus(etcd.MembershipPhaseLeaving, testMemberIDString)

	// the request is past the point of no return, so it can't be withdrawn
	ctest.AssertResource(suite, etcd.LeaveRequestID, func(request *etcd.LeaveRequest, asrt *assert.Assertions) {
		asrt.True(request.Metadata().Finalizers().Has("etcd.MembershipController"))
	})
	suite.Require().Error(suite.State().Destroy(suite.Ctx(), etcd.NewLeaveRequest().Metadata()))

	// the data is not touched until etcd is stopped
	suite.assertDataDir("member")

	// the service controller stops etcd
	suite.Destroy(serviceSpec)

	suite.assertStatus(etcd.MembershipPhaseLeft, "")
	suite.assertDataDir()

	memberID, err := etcd.ParseMemberID(testMemberIDString)
	suite.Require().NoError(err)

	suite.cluster.set(func(c *fakeCluster) {
		suite.Equal([]uint64{memberID}, c.removed)
	})

	// the machine doesn't join the cluster back
	suite.Destroy(member)

	suite.assertStatus(etcd.MembershipPhaseLeft, "")
	ctest.AssertNoResource[*etcd.LaunchSpec](suite, etcd.LaunchSpecID)

	suite.cluster.set(func(c *fakeCluster) {
		suite.Zero(c.joins)
	})
}

func (suite *MembershipSuite) TestLeaveWithdrawn() {
	suite.cluster.removeErr = errors.New("no quorum")

	suite.createMemberData()
	suite.createInputs()

	member := etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID)
	member.TypedSpec().MemberID = testMemberIDString
	suite.Create(member)

	suite.assertStatus(etcd.MembershipPhaseMember, testMemberIDString)

	request := etcd.NewLeaveRequest()
	suite.Create(request)

	// the member can't be removed from the cluster
	ctest.AssertResource(suite, etcd.MembershipStatusID, func(status *etcd.MembershipStatus, asrt *assert.Assertions) {
		asrt.Equal(etcd.MembershipPhaseLeaving, status.TypedSpec().Phase)
		asrt.Equal("no quorum", status.TypedSpec().Error)
	})

	// the request is withdrawn, as the member was not removed
	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), request.Metadata()))

	suite.assertStatus(etcd.MembershipPhaseMember, testMemberIDString)

	// the cluster is back, but the machine stays in the cluster
	suite.cluster.set(func(c *fakeCluster) { c.removeErr = nil })

	suite.assertLaunch("existing", "")
	suite.assertDataDir("member")

	suite.cluster.set(func(c *fakeCluster) { suite.Empty(c.removed) })
}

func (suite *MembershipSuite) TestLeaveWithdrawnLearner() {
	suite.cluster.promoteErr = errors.New("learner is not in sync")
	suite.cluster.removeErr = errors.New("no quorum")

	suite.createInputs()

	suite.assertStatus(etcd.MembershipPhaseLearner, testLearnerIDString)

	suite.Create(v1alpha1.NewService("etcd"))

	request := etcd.NewLeaveRequest()
	suite.Create(request)

	ctest.AssertResource(suite, etcd.MembershipStatusID, func(status *etcd.MembershipStatus, asrt *assert.Assertions) {
		asrt.Equal(etcd.MembershipPhaseLeaving, status.TypedSpec().Phase)
		asrt.Equal("no quorum", status.TypedSpec().Error)
	})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), request.Metadata()))

	// the machine is still a learner (and not a voting member), so it gets promoted
	suite.assertStatus(etcd.MembershipPhaseLearner, testLearnerIDString)
	suite.assertDataDir(etcdcli.LearnerMarkerFilename)

	suite.cluster.set(func(c *fakeCluster) { c.promoteErr = nil })

	suite.assertStatus(etcd.MembershipPhaseMember, testLearnerIDString)
	suite.assertDataDir()

	suite.cluster.set(func(c *fakeCluster) {
		suite.Equal([]uint64{testLearnerID}, c.promoted)
		suite.Empty(c.removed)
	})
}

func (suite *MembershipSuite) TestLeaveNotMember() {
	suite.cluster.joinErr = errors.New("no cluster")

	suite.createInputs()

	suite.assertStatus(etcd.MembershipPhaseJoining, "")

	suite.Create(etcd.NewLeaveRequest())

	// nothing to remove from the cluster
	suite.assertStatus(etcd.MembershipPhaseLeft, "")

	suite.cluster.set(func(c *fakeCluster) {
		suite.Empty(c.removed)
	})
}

func (suite *MembershipSuite) TestVolumeUnmounted() {
	suite.createMemberData()
	suite.createInputs()

	suite.assertLaunch("existing", "")

	mountStatus := block.NewVolumeMountStatus(block.NamespaceName, "etcd.MembershipController-"+constants.EtcdDataVolumeID)

	// the controller holds the volume
	ctest.AssertResource(suite, mountStatus.Metadata().ID(), func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Has("etcd.MembershipController"))
	})

	// the volume is being unmounted: etcd should be stopped, and the volume is released
	_, err := suite.State().Teardown(suite.Ctx(), mountStatus.Metadata())
	suite.Require().NoError(err)

	ctest.AssertNoResource[*etcd.LaunchSpec](suite, etcd.LaunchSpecID)
	suite.assertStatus(etcd.MembershipPhasePreparing, "")

	ctest.AssertResource(suite, mountStatus.Metadata().ID(), func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(status.Metadata().Finalizers().Empty())
	})
}
