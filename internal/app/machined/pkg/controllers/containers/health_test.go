// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"context"
	"errors"
	"io"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const healthTestContainer = "web"

// testCgroupPath is the cgroup the task of healthTestContainer runs in, as machined sees it.
const testCgroupPath = "/taloscontainers/" + healthTestContainer

// fakeProber hands out probes whose outcome the test controls, and records what was probed.
type fakeProber struct {
	mu         sync.Mutex
	err        error
	provideErr error
	panic      bool
	targets    []containersctrl.ProbeTarget
	closed     int
	checks     int
}

func (prober *fakeProber) provide(target containersctrl.ProbeTarget) (containersctrl.Probe, io.Closer, error) {
	prober.mu.Lock()
	defer prober.mu.Unlock()

	prober.targets = append(prober.targets, target)

	if prober.provideErr != nil {
		return nil, nil, prober.provideErr
	}

	return func(context.Context) error {
		prober.mu.Lock()
		defer prober.mu.Unlock()

		prober.checks++

		if prober.panic {
			panic("probe exploded")
		}

		return prober.err
	}, prober, nil
}

// Close counts the probes released by the controller.
func (prober *fakeProber) Close() error {
	prober.mu.Lock()
	defer prober.mu.Unlock()

	prober.closed++

	return nil
}

func (prober *fakeProber) setProvideErr(err error) {
	prober.mu.Lock()
	defer prober.mu.Unlock()

	prober.provideErr = err
}

func (prober *fakeProber) setPanic() {
	prober.mu.Lock()
	defer prober.mu.Unlock()

	prober.panic = true
}

func (prober *fakeProber) closedCount() int {
	prober.mu.Lock()
	defer prober.mu.Unlock()

	return prober.closed
}

func (prober *fakeProber) setErr(err error) {
	prober.mu.Lock()
	defer prober.mu.Unlock()

	prober.err = err
}

// probeCount returns a condition on the number of probes handed out.
//
// It closes over prober rather than the suite: testify may still evaluate a condition after the test
// returned, while the next test replaces the suite's prober.
func (prober *fakeProber) probeCount(check func(int) bool) func() bool {
	return func() bool { return check(len(prober.probedTargets())) }
}

// checkCount returns a condition on the number of checks run; see probeCount.
func (prober *fakeProber) checkCount(check func(int) bool) func() bool {
	return func() bool {
		prober.mu.Lock()
		defer prober.mu.Unlock()

		return check(prober.checks)
	}
}

func (prober *fakeProber) checksRun() int {
	prober.mu.Lock()
	defer prober.mu.Unlock()

	return prober.checks
}

// probed returns a condition on target having been probed; see probeCount.
func (prober *fakeProber) probed(target containersctrl.ProbeTarget) func() bool {
	return func() bool { return slices.Contains(prober.probedTargets(), target) }
}

func (prober *fakeProber) probedTargets() []containersctrl.ProbeTarget {
	prober.mu.Lock()
	defer prober.mu.Unlock()

	return slices.Clone(prober.targets)
}

type HealthSuite struct {
	ctest.DefaultSuite

	prober *fakeProber
}

func TestHealthSuite(t *testing.T) {
	t.Parallel()

	healthSuite := &HealthSuite{}

	healthSuite.DefaultSuite = ctest.DefaultSuite{
		Timeout: 10 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			healthSuite.prober = &fakeProber{}

			suite.Require().NoError(suite.Runtime().RegisterController(&containersctrl.HealthController{
				ProbeProvider: healthSuite.prober.provide,
			}))
		},
	}

	suite.Run(t, healthSuite)
}

var testHealthCheck = containers.ContainerHealthCheckSpec{
	Kind:             string(config.ContainerHealthCheckKindTCP),
	Port:             80,
	Interval:         10 * time.Millisecond,
	Timeout:          10 * time.Millisecond,
	FailureThreshold: 2,
}

func (suite *HealthSuite) createSpec(healthCheck containers.ContainerHealthCheckSpec) *containers.ContainerSpec {
	spec := containers.NewContainerSpec(containers.NamespaceName, healthTestContainer)
	spec.TypedSpec().HealthCheck = healthCheck

	suite.Create(spec)

	return spec
}

// createInstance creates a generation's instance spec and a running instance status for it.
func (suite *HealthSuite) createInstance(generation uint64, pid uint32) {
	id := containers.InstanceID(healthTestContainer, generation)

	instanceSpec := containers.NewContainerInstanceSpec(containers.NamespaceName, id)
	instanceSpec.TypedSpec().ContainerID = healthTestContainer
	instanceSpec.TypedSpec().Generation = generation
	suite.Create(instanceSpec)

	instanceStatus := containers.NewContainerInstanceStatus(containers.NamespaceName, id)
	instanceStatus.TypedSpec().ContainerID = healthTestContainer
	instanceStatus.TypedSpec().Generation = generation
	instanceStatus.TypedSpec().Phase = containers.ContainerInstancePhaseRunning
	instanceStatus.TypedSpec().PID = pid
	instanceStatus.Metadata().Labels().Set(containers.ContainerSpecIdLabel, healthTestContainer)
	suite.Create(instanceStatus)
}

func (suite *HealthSuite) updateInstanceStatus(generation uint64, mutate func(*containers.ContainerInstanceStatusSpec)) {
	md := containers.NewContainerInstanceStatus(containers.NamespaceName, containers.InstanceID(healthTestContainer, generation)).Metadata()

	existing, err := suite.State().Get(suite.Ctx(), md)
	suite.Require().NoError(err)

	instanceStatus := existing.(*containers.ContainerInstanceStatus) //nolint:forcetypeassert,errcheck
	mutate(instanceStatus.TypedSpec())

	suite.Update(instanceStatus)
}

func (suite *HealthSuite) TestReportsOutcome() {
	suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	id := containers.InstanceID(healthTestContainer, 0)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
		asrt.Empty(status.TypedSpec().Message)

		label, _ := status.Metadata().Labels().Get(containers.ContainerSpecIdLabel)
		asrt.Equal(healthTestContainer, label)
	})

	suite.Equal([]containersctrl.ProbeTarget{{PID: 42, CgroupPath: testCgroupPath, HealthCheck: testHealthCheck}}, suite.prober.probedTargets())

	suite.prober.setErr(errors.New("connection refused"))

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Healthy)
		asrt.Equal("connection refused", status.TypedSpec().Message)
	})
}

// TestUndecidedHasNoStatus covers a container that has not passed a check yet: nothing is reported,
// which is what keeps it pending.
func (suite *HealthSuite) TestUndecidedHasNoStatus() {
	suite.prober.setErr(errors.New("connection refused"))

	healthCheck := testHealthCheck
	healthCheck.FailureThreshold = 1000

	suite.createSpec(healthCheck)
	suite.createInstance(0, 42)

	suite.Eventually(suite.prober.probeCount(func(n int) bool { return n == 1 }), time.Second, 10*time.Millisecond)

	ctest.AssertNoResource[*containers.ContainerHealthStatus](suite, containers.InstanceID(healthTestContainer, 0))
}

func (suite *HealthSuite) TestNoHealthCheckNoProbe() {
	suite.createSpec(containers.ContainerHealthCheckSpec{})
	suite.createInstance(0, 42)

	suite.Never(suite.prober.probeCount(func(n int) bool { return n > 0 }), 200*time.Millisecond, 10*time.Millisecond)

	ctest.AssertNoResource[*containers.ContainerHealthStatus](suite, containers.InstanceID(healthTestContainer, 0))
}

func (suite *HealthSuite) TestStopsWhenInstanceStops() {
	suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	id := containers.InstanceID(healthTestContainer, 0)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
	})

	suite.updateInstanceStatus(0, func(spec *containers.ContainerInstanceStatusSpec) {
		spec.Phase = containers.ContainerInstancePhaseTerminated
		spec.PID = 0
	})

	ctest.AssertNoResource[*containers.ContainerHealthStatus](suite, id)
}

// TestFollowsNewGeneration covers a restart: the next generation gets its own probe and status, and
// the previous one's goes away with it.
func (suite *HealthSuite) TestFollowsNewGeneration() {
	suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	ctest.AssertResource(suite, containers.InstanceID(healthTestContainer, 0), func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
	})

	suite.updateInstanceStatus(0, func(spec *containers.ContainerInstanceStatusSpec) {
		spec.Phase = containers.ContainerInstancePhaseTerminated
		spec.PID = 0
	})

	suite.createInstance(1, 43)

	ctest.AssertResource(suite, containers.InstanceID(healthTestContainer, 1), func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
	})

	ctest.AssertNoResource[*containers.ContainerHealthStatus](suite, containers.InstanceID(healthTestContainer, 0))

	suite.Contains(suite.prober.probedTargets(), containersctrl.ProbeTarget{PID: 43, CgroupPath: testCgroupPath, HealthCheck: testHealthCheck})
}

// TestRetargetsOnSpecChange covers editing the health check of a running container: the probe is
// replaced, the execution is left alone.
func (suite *HealthSuite) TestRetargetsOnSpecChange() {
	spec := suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	ctest.AssertResource(suite, containers.InstanceID(healthTestContainer, 0), func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
	})

	edited := testHealthCheck
	edited.Port = 8080

	existing, err := suite.State().Get(suite.Ctx(), spec.Metadata())
	suite.Require().NoError(err)

	spec = existing.(*containers.ContainerSpec) //nolint:forcetypeassert,errcheck
	spec.TypedSpec().HealthCheck = edited
	suite.Update(spec)

	suite.Eventually(suite.prober.probed(containersctrl.ProbeTarget{PID: 42, CgroupPath: testCgroupPath, HealthCheck: edited}), 5*time.Second, 10*time.Millisecond)

	// The replaced probe is released.
	prober := suite.prober

	suite.Eventually(func() bool { return prober.closedCount() >= 1 }, 5*time.Second, 10*time.Millisecond)
}

// TestRetargetKeepsOutcome covers editing the schedule of a container that already passed: the new
// probe carries the outcome over, so the status is never withdrawn, which would read as pending.
func (suite *HealthSuite) TestRetargetKeepsOutcome() {
	spec := suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	id := containers.InstanceID(healthTestContainer, 0)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
	})

	// An undecided probe would wait out the hour before publishing anything; one carrying the
	// previous outcome over neither waits nor withdraws it.
	edited := testHealthCheck
	edited.InitialDelay = time.Hour
	edited.Interval = 20 * time.Millisecond

	existing, err := suite.State().Get(suite.Ctx(), spec.Metadata())
	suite.Require().NoError(err)

	spec = existing.(*containers.ContainerSpec) //nolint:forcetypeassert,errcheck
	spec.TypedSpec().HealthCheck = edited
	suite.Update(spec)

	suite.Eventually(suite.prober.probed(containersctrl.ProbeTarget{PID: 42, CgroupPath: testCgroupPath, HealthCheck: edited}), 5*time.Second, 10*time.Millisecond)

	md := containers.NewContainerHealthStatus(containers.NamespaceName, id).Metadata()

	// Captured: Never's condition may still be running once the next test has replaced them.
	ctx, st := suite.Ctx(), suite.State()

	suite.Never(func() bool {
		_, err := st.Get(ctx, md)

		return err != nil
	}, 300*time.Millisecond, 10*time.Millisecond)
}

// TestRetargetResetsOutcome covers changing what a container that already passed is checked on: the
// outcome of the previous check says nothing about the new one, so it is withdrawn, which reads as
// pending, until the new check decides.
func (suite *HealthSuite) TestRetargetResetsOutcome() {
	spec := suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	id := containers.InstanceID(healthTestContainer, 0)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
	})

	// Nothing listens on the new port, and the new check never fails enough times to decide.
	suite.prober.setErr(errors.New("connection refused"))

	edited := testHealthCheck
	edited.Port = 8080
	edited.FailureThreshold = 1000

	existing, err := suite.State().Get(suite.Ctx(), spec.Metadata())
	suite.Require().NoError(err)

	spec = existing.(*containers.ContainerSpec) //nolint:forcetypeassert,errcheck
	spec.TypedSpec().HealthCheck = edited
	suite.Update(spec)

	suite.Eventually(suite.prober.probed(containersctrl.ProbeTarget{PID: 42, CgroupPath: testCgroupPath, HealthCheck: edited}), 5*time.Second, 10*time.Millisecond)

	ctest.AssertNoResource[*containers.ContainerHealthStatus](suite, id)

	// The new check passing decides it again.
	suite.prober.setErr(nil)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
	})
}

// TestPanicReportsUnhealthy covers a probe that panics: it is not restarted while its target is
// unchanged, so the container is reported unhealthy rather than pending for good.
func (suite *HealthSuite) TestPanicReportsUnhealthy() {
	suite.prober.setPanic()

	suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	ctest.AssertResource(suite, containers.InstanceID(healthTestContainer, 0), func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Healthy)
		asrt.Equal("goroutine panicked: probe exploded", status.TypedSpec().Message)
	})
}

// TestIgnoresTearingDownInstance covers an instance on its way out: it is not probed any more.
func (suite *HealthSuite) TestIgnoresTearingDownInstance() {
	suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	id := containers.InstanceID(healthTestContainer, 0)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
	})

	instanceSpecMd := containers.NewContainerInstanceSpec(containers.NamespaceName, id).Metadata()
	suite.AddFinalizer(instanceSpecMd, "test")

	_, err := suite.State().Teardown(suite.Ctx(), resource.NewMetadata(containers.NamespaceName, containers.ContainerInstanceSpecType, id, resource.VersionUndefined))
	suite.Require().NoError(err)

	ctest.AssertNoResource[*containers.ContainerHealthStatus](suite, id)
}

// TestRecoversAfterFailure covers a container whose check starts passing again after it was reported
// unhealthy.
func (suite *HealthSuite) TestRecoversAfterFailure() {
	suite.prober.setErr(errors.New("connection refused"))

	suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	id := containers.InstanceID(healthTestContainer, 0)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Healthy)
		asrt.Equal("connection refused", status.TypedSpec().Message)
	})

	suite.prober.setErr(nil)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
		asrt.Empty(status.TypedSpec().Message)
	})
}

// TestFailureMessageChangeNotRepublished covers a container which keeps failing with a different
// message on every check, e.g. one carrying the ephemeral source port: its status is written once, with
// the failure which made it unhealthy, rather than on every interval.
func (suite *HealthSuite) TestFailureMessageChangeNotRepublished() {
	suite.prober.setErr(errors.New("read tcp 127.0.0.1:53817->127.0.0.1:8080: connection reset by peer"))

	suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	id := containers.InstanceID(healthTestContainer, 0)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Healthy)
		asrt.Equal("read tcp 127.0.0.1:53817->127.0.0.1:8080: connection reset by peer", status.TypedSpec().Message)
	})

	published, err := safe.StateGetByID[*containers.ContainerHealthStatus](suite.Ctx(), suite.State(), id)
	suite.Require().NoError(err)

	suite.prober.setErr(errors.New("read tcp 127.0.0.1:53818->127.0.0.1:8080: connection reset by peer"))

	checks := suite.prober.checksRun()
	suite.Eventually(suite.prober.checkCount(func(n int) bool { return n >= checks+5 }), time.Second, 10*time.Millisecond)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.Equal(published.Metadata().Version(), status.Metadata().Version())
		asrt.Equal("read tcp 127.0.0.1:53817->127.0.0.1:8080: connection reset by peer", status.TypedSpec().Message)
	})
}

// TestProbeProviderErrorReportsUnhealthy covers a probe which cannot be built, for example because
// the network namespace of the task is gone: every check fails with that error, so the container
// is reported unhealthy rather than pending for good.
func (suite *HealthSuite) TestProbeProviderErrorReportsUnhealthy() {
	suite.prober.setProvideErr(errors.New("failed to open network namespace"))

	suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	ctest.AssertResource(suite, containers.InstanceID(healthTestContainer, 0), func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Healthy)
		asrt.Equal("failed to open network namespace", status.TypedSpec().Message)
	})

	// Nothing was opened, so nothing is closed.
	suite.Zero(suite.prober.closedCount())
}

// TestUnknownPIDIsProbed covers a running task whose PID the runner could not resolve: it is probed
// all the same, so that a failure to build the probe surfaces as unhealthy rather than the container
// staying pending, and its dependents waiting, for good.
func (suite *HealthSuite) TestUnknownPIDIsProbed() {
	suite.prober.setProvideErr(errors.New("container PID is not known"))

	suite.createSpec(testHealthCheck)
	suite.createInstance(0, 0)

	ctest.AssertResource(suite, containers.InstanceID(healthTestContainer, 0), func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Healthy)
		asrt.Equal("container PID is not known", status.TypedSpec().Message)
	})

	suite.Contains(suite.prober.probedTargets(), containersctrl.ProbeTarget{PID: 0, CgroupPath: testCgroupPath, HealthCheck: testHealthCheck})
}

// TestNoProbeWithoutInstanceSpec covers a running instance status whose instance spec is gone.
func (suite *HealthSuite) TestNoProbeWithoutInstanceSpec() {
	suite.createSpec(testHealthCheck)

	id := containers.InstanceID(healthTestContainer, 0)

	instanceStatus := containers.NewContainerInstanceStatus(containers.NamespaceName, id)
	instanceStatus.TypedSpec().ContainerID = healthTestContainer
	instanceStatus.TypedSpec().Phase = containers.ContainerInstancePhaseRunning
	instanceStatus.TypedSpec().PID = 42
	instanceStatus.Metadata().Labels().Set(containers.ContainerSpecIdLabel, healthTestContainer)
	suite.Create(instanceStatus)

	suite.Never(suite.prober.probeCount(func(n int) bool { return n > 0 }), 200*time.Millisecond, 10*time.Millisecond)

	ctest.AssertNoResource[*containers.ContainerHealthStatus](suite, id)
}

// TestNoProbeWithoutContainerSpec covers a running instance whose container spec is gone.
func (suite *HealthSuite) TestNoProbeWithoutContainerSpec() {
	suite.createInstance(0, 42)

	suite.Never(suite.prober.probeCount(func(n int) bool { return n > 0 }), 200*time.Millisecond, 10*time.Millisecond)

	ctest.AssertNoResource[*containers.ContainerHealthStatus](suite, containers.InstanceID(healthTestContainer, 0))
}

// TestStopsDuringInitialDelay covers an instance which stops before its first check is due: the
// probe is released without ever reporting anything.
func (suite *HealthSuite) TestStopsDuringInitialDelay() {
	healthCheck := testHealthCheck
	healthCheck.InitialDelay = time.Hour

	suite.createSpec(healthCheck)
	suite.createInstance(0, 42)

	prober := suite.prober

	suite.Eventually(prober.probeCount(func(n int) bool { return n == 1 }), time.Second, 10*time.Millisecond)

	suite.updateInstanceStatus(0, func(spec *containers.ContainerInstanceStatusSpec) {
		spec.Phase = containers.ContainerInstancePhaseTerminated
		spec.PID = 0
	})

	suite.Eventually(func() bool { return prober.closedCount() == 1 }, 5*time.Second, 10*time.Millisecond)

	ctest.AssertNoResource[*containers.ContainerHealthStatus](suite, containers.InstanceID(healthTestContainer, 0))
}

// TestProbeProviderRetried covers a probe which cannot be built at first: building it is retried on
// every check, so the container turns healthy once it can be, without its target changing.
func (suite *HealthSuite) TestProbeProviderRetried() {
	suite.prober.setProvideErr(errors.New("failed to open network namespace"))

	suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	id := containers.InstanceID(healthTestContainer, 0)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Healthy)
	})

	suite.prober.setProvideErr(nil)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
		asrt.Empty(status.TypedSpec().Message)
	})
}

// TestProbeSurvivesCRIRestart covers the CRI containerd restarting while its tasks keep running: the
// task PIDs are already in machined's PID namespace, so the probes neither stop nor lose their
// outcome.
func (suite *HealthSuite) TestProbeSurvivesCRIRestart() {
	criPID := runtimeres.NewServicePID("cri")
	criPID.TypedSpec().PID = 7
	suite.Create(criPID)

	suite.createSpec(testHealthCheck)
	suite.createInstance(0, 42)

	id := containers.InstanceID(healthTestContainer, 0)

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
	})

	suite.Destroy(criPID)

	restarted := runtimeres.NewServicePID("cri")
	restarted.TypedSpec().PID = 8
	suite.Create(restarted)

	suite.Never(func() bool { return suite.prober.closedCount() > 0 }, 200*time.Millisecond, 10*time.Millisecond)
	suite.Equal([]containersctrl.ProbeTarget{{PID: 42, CgroupPath: testCgroupPath, HealthCheck: testHealthCheck}}, suite.prober.probedTargets())

	ctest.AssertResource(suite, id, func(status *containers.ContainerHealthStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Healthy)
	})
}
