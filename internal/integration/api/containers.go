// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	configcontainer "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	cfgcontainer "github.com/siderolabs/talos/pkg/machinery/config/container"
	machinetype "github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	containercfg "github.com/siderolabs/talos/pkg/machinery/config/types/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	// containerPauseImage never writes anything and never exits, which is what the lifecycle tests
	// want.
	//
	// Note: this image is part of the default image cache in `talosctl image integration`.
	containerPauseImage = images.DefaultSandboxImage

	// containerShellImage is used wherever a test needs the container to run a command and say
	// something about itself. The pause image has no shell and produces no output, so it cannot
	// carry any assertion about what the container actually got.
	//
	// Note: this image is part of the default image cache in `talosctl image integration`.
	containerShellImage = "docker.io/library/alpine:3.23"

	// containerSocatImage carries a unix-socket client, needed for machined socket access testing.
	//
	// Note: this image is part of the default image cache in `talosctl image integration`.
	containerSocatImage = "docker.io/alpine/socat:1.8.1.3"
)

// containerStartTimeout covers an image pull on a cold node plus the controller chain.
const containerStartTimeout = 5 * time.Minute

// To help track finalizers.
const containerMountControllerName = "containers.MountController"

// containerMountRequestID builds the ID of the volume mount request, and so of the resulting
// block.VolumeMountStatus, for one container mounting one user volume.
//
// Per container rather than per volume, which is what lets two containers hold the same volume
// independently.
func containerMountRequestID(containerName, volumeName string) string {
	return containerMountControllerName + "/" + containerName + "/" + constants.UserVolumePrefix + volumeName
}

// ContainersSuite verifies containers declared via ContainerConfig.
type ContainersSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	// ownedVolumes maps a container name to the names of the volumes declared for it.
	ownedVolumes map[string][]string
}

// SuiteName ...
func (suite *ContainersSuite) SuiteName() string {
	return "api.ContainersSuite"
}

// SetupTest ...
func (suite *ContainersSuite) SetupTest() {
	// Enough for an image pull; the reboot test extends its own deadline.
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 10*time.Minute)
}

// TearDownTest ...
func (suite *ContainersSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestContainerLifecycle covers the life of a declared container that does not involve a reboot: it
// starts, it is replaced rather than mutated when its configuration changes, and it is gone once
// that configuration is removed.
func (suite *ContainersSuite) TestContainerLifecycle() {
	ctx, name, _ := suite.setupContainer("lifecycle")

	suite.applyContainers(ctx, suite.newContainer(name, containerPauseImage))

	oldInstanceID, oldInstance := suite.assertContainerRunning(ctx, name, "initial start")

	suite.T().Logf("adding an environment variable to container config %q", name)

	const envVar = "FOO=BAR"

	updated := suite.newContainer(name, containerPauseImage)
	updated.ContainerEnvironment = []string{envVar}

	suite.applyContainers(ctx, updated)

	rtestutils.AssertNoResource[*containers.ContainerInstanceSpec](ctx, suite.T(), suite.Client.State(), oldInstanceID)

	newInstanceID, newInstance := suite.assertContainerRunning(ctx, name, "after the environment change")

	suite.Assert().NotEqual(oldInstanceID, newInstanceID, "instance was reused across the change")
	suite.Assert().Greater(newInstance.Generation, oldInstance.Generation,
		"expected a generation later than %d, got %d", oldInstance.Generation, newInstance.Generation)
	suite.Assert().NotEqual(oldInstance.PID, newInstance.PID,
		"container %q is still running as PID %d, so it was not restarted", name, oldInstance.PID)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), newInstanceID,
		func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
			asrt.Contains(instance.TypedSpec().Environment, envVar)
		})

	suite.T().Logf("removing container config %q", name)

	suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, name)

	rtestutils.AssertNoResource[*containers.ContainerSpec](ctx, suite.T(), suite.Client.State(), name)

	suite.assertNoContainerdContainer(ctx, name)
}

// TestSurvivesReboot verifies that a declared container is stopped on the way down and started again
// as a new task on the way up.
func (suite *ContainersSuite) TestSurvivesReboot() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	// A reboot is slower than anything else in this suite, so this test gets a deadline of its own.
	suite.ctxCancel()
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 20*time.Minute)

	ctx, name, node := suite.setupContainer("reboot")

	suite.applyContainers(ctx, suite.newContainer(name, containerPauseImage))

	_, beforeInstance := suite.assertContainerRunning(ctx, name, "before the reboot")

	suite.T().Logf("rebooting node %s", node)

	// AssertRebooted reads boot_id either side of the reboot, so the reboot itself is verified here.
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 10*time.Minute,
	)

	_, afterInstance := suite.assertContainerRunning(ctx, name, "after the reboot")

	// What came back is a new task, not the one that was running before the node went down.
	//
	// Neither the instance ID nor the generation can show this: COSI state does not survive a reboot,
	// so the instance controller finds no instances on the way up and numbers the first one 0 again,
	// reusing the ID it had before. StartedAt is the one field that distinguishes them.
	//
	// Compared for inequality rather than order: the node's clock can step backwards early in boot,
	// before it has synced, which is the whole reason dependsOn.time exists. Requiring the later
	// timestamp to be greater would turn that into a failure of this test.
	suite.Assert().NotEqual(beforeInstance.StartedAt, afterInstance.StartedAt,
		"container %q reports the same start time as before the reboot, so this is the pre-reboot task", name)
}

// TestRestartAfterTermination verifies that a container which exits on its own is replaced by a new
// instance once the restart interval has elapsed, whether it exited cleanly or not.
func (suite *ContainersSuite) TestRestartAfterTermination() {
	for _, test := range []struct {
		name     string
		command  string
		exitCode int32
	}{
		{name: "clean-exit", command: "exit 0", exitCode: 0},
		{name: "failed-exit", command: "exit 3", exitCode: 3},
	} {
		suite.Run(test.name, func() {
			ctx, name, _ := suite.setupContainer(test.name)

			suite.applyContainers(ctx, suite.shellContainer(name, test.command))

			firstID, first := suite.assertNewestInstance(ctx, name, "first exit",
				func(status *containers.ContainerInstanceStatusSpec, asrt *assert.Assertions) bool {
					return asrt.Equal(containers.ContainerInstancePhaseTerminated, status.Phase,
						"phase is %s (error %q)", status.Phase, status.Error) &&
						asrt.Equal(test.exitCode, status.ExitCode)
				})

			suite.Assert().False(first.FinishedAt.IsZero(), "FinishedAt was not recorded")

			// The replacement is due one RestartInterval after the exit, so this wait has to outlast
			// it by a comfortable margin.
			suite.T().Logf("waiting for container %q to be restarted", name)

			suite.assertNewestInstance(ctx, name, "restart",
				func(status *containers.ContainerInstanceStatusSpec, asrt *assert.Assertions) bool {
					return asrt.Greater(status.Generation, first.Generation,
						"still on generation %d, no replacement instance yet", first.Generation)
				})

			// And the instance that exited is not kept around: zero instances are retained.
			rtestutils.AssertNoResource[*containers.ContainerInstanceSpec](ctx, suite.T(), suite.Client.State(), firstID)
		})
	}
}

// TestContainerStatus verifies the aggregated, user-facing ContainerStatus.
func (suite *ContainersSuite) TestContainerStatus() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	ctx, name, _ := suite.setupContainer("status")

	// Runs long enough for "running" to be observable, then exits with a code nothing else here
	// produces, so the exit reported afterwards is unambiguously this container's. shellContainer
	// restarts it, so every stage below recurs and a missed window is caught on the next cycle.
	const exitCode = 7

	suite.applyContainers(ctx, suite.shellContainer(name, fmt.Sprintf("sleep 15; exit %d", exitCode)))

	running := suite.assertContainerStatus(ctx, name, "running",
		func(status *containers.ContainerStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerStateRunning, status.State,
				"state is %s (error %q)", status.State, status.Error)
			asrt.Equal(containers.ContainerHealthHealthy, status.Health)
			asrt.NotZero(status.PID, "no PID reported")
			asrt.Empty(status.WaitingFor, "a running container is waiting on something")
			asrt.Empty(status.Error)
		})

	// What is reported is what the pull resolved to, not the reference that was asked for: the
	// instance runs the digest, so that is what the status has to show.
	suite.Assert().True(strings.HasPrefix(running.Image, "sha256:"),
		"reported image %q is not the resolved digest", running.Image)

	exited := suite.assertContainerStatus(ctx, name, "exited",
		func(status *containers.ContainerStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerStateExited, status.State,
				"state is %s (error %q)", status.State, status.Error)
			asrt.Equal(containers.ContainerHealthDegraded, status.Health)
			asrt.Equal(int32(exitCode), status.ExitCode)
			asrt.Zero(status.PID, "a container that has exited still reports a PID")
		})

	// The replacement instance is created a restart interval after the exit, and for that whole
	// window there is no instance status to fold at all. The counter still has to climb across it:
	// the last execution's outcome is exactly what an operator is looking at while the restart is
	// pending.
	suite.T().Logf("waiting for container %q to be restarted", name)

	suite.assertContainerStatus(ctx, name, "restarted",
		func(status *containers.ContainerStatusSpec, asrt *assert.Assertions) {
			asrt.Greater(status.RestartCount, exited.RestartCount,
				"still at restart %d, no replacement instance yet", exited.RestartCount)
		})

	suite.T().Logf("removing container config %q", name)

	suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, name)

	rtestutils.AssertNoResource[*containers.ContainerStatus](ctx, suite.T(), suite.Client.State(), name)
}

// TestContainerStatusWaitingFor verifies that the gates holding a container back reach the
// aggregated status, which is the only place an operator can see why it has not started.
func (suite *ContainersSuite) TestContainerStatusWaitingFor() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	ctx, name, node := suite.setupContainer("status-gates")

	// Declared, but never found, so the mount never becomes ready and the container is withheld. The
	// same gate TestVolumeMountGate covers at the mount-status level; what is asserted here is that
	// the verdict is relayed all the way to the user-facing status.
	volumeName := fmt.Sprintf("itv-stgate-%04x", rand.Int31())

	doc := suite.newContainer(name, containerPauseImage)
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			VolumeMount: &containercfg.VolumeMount{
				VolumeName:       volumeName,
				MountDestination: "/mnt/data",
			},
		},
	}

	suite.declareUnmountableVolume(ctx, node, name, volumeName, doc)

	suite.assertContainerStatus(ctx, name, "pending on the volume which is never mounted",
		func(status *containers.ContainerStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerStatePending, status.State,
				"state is %s (error %q)", status.State, status.Error)
			asrt.Equal(containers.ContainerHealthPending, status.Health)
			asrt.Contains(status.WaitingFor, "mounts")
			asrt.Zero(status.PID)
		})

	suite.replaceWithUserVolume(ctx, node, volumeName)

	// The state and health of the started container are asserted by the helper; that the gate list
	// empties out again is what this adds.
	suite.assertContainerRunning(ctx, name, "once the volume was mounted")

	suite.assertContainerStatus(ctx, name, "no gates left",
		func(status *containers.ContainerStatusSpec, asrt *assert.Assertions) {
			asrt.Empty(status.WaitingFor)
		})
}

// TestUnresolvableImage verifies that a container whose image cannot be pulled is withheld while the
// pull keeps retrying, and starts once the reference is corrected.
func (suite *ContainersSuite) TestUnresolvableImage() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	ctx, name, _ := suite.setupContainer("bad-image")

	// Built off the pause image rather than the shell one: the corrected container has to stay up
	// long enough for "running" to be observable, and alpine's default command exits at once.
	const badImage = containerPauseImage + "-no-such-tag"

	suite.applyContainers(ctx, suite.newContainer(name, badImage))

	// Pulling, not failed. Retries live inside the pull and run for image.PullTimeout, twenty
	// minutes, so that a registry which is briefly unreachable or a mirror which comes up late
	// resolves without anything upstream re-triggering. A reference that will never resolve is
	// indistinguishable from those until the timeout expires, which is well beyond any deadline this
	// test should have, so the failed phase is not what gets asserted here.
	suite.T().Logf("waiting for the image pull of %q to be under way", name)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), name,
		func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerImagePhasePulling, status.TypedSpec().Phase,
				"image is in phase %s (error %q)", status.TypedSpec().Phase, status.TypedSpec().Error)
			asrt.Empty(status.TypedSpec().Digest, "an unresolvable reference produced a digest")
		})

	// The image phase is what the aggregate reports in its own right here: there is no instance to
	// fold, so pulling is read straight off the image status rather than from the gate list.
	suite.assertContainerStatus(ctx, name, "pulling an image that will not resolve",
		func(status *containers.ContainerStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerStatePulling, status.State,
				"state is %s (error %q)", status.State, status.Error)
			asrt.Equal(containers.ContainerHealthPulling, status.Health)
		})

	// The point of the test: ContainerSpec.Ready withholds the container while the digest is
	// unresolved, so no instance is created however long the pull goes on.
	suite.assertNoInstance(ctx, name)

	suite.T().Logf("correcting the image reference of %q", name)

	// Changing the reference invalidates the in-flight pull, so this does not wait out the retries.
	suite.applyContainers(ctx, suite.newContainer(name, containerPauseImage))

	suite.assertContainerRunning(ctx, name, "after the image reference was corrected")
}

// TestEntrypointAndArgs verifies that entrypoint and args reach the container's process.
func (suite *ContainersSuite) TestEntrypointAndArgs() {
	suite.Run("entrypoint-and-args", func() {
		ctx, name, _ := suite.setupContainer("entrypoint-args")

		doc := suite.newContainer(name, containerShellImage)
		doc.ContainerEntrypoint = []string{"/bin/echo"}
		doc.ContainerArgs = []string{"entrypoint-and-args-ok"}

		suite.applyContainers(ctx, doc)

		suite.assertContainerLogged(ctx, name, "entrypoint-and-args-ok")
	})

	// The alpine image declares no ENTRYPOINT, so args alone become the command. The complementary
	// case -- args overriding CMD while the image's ENTRYPOINT is preserved -- needs an image that
	// declares one, and no such image is available to this suite.
	suite.Run("args-only", func() {
		ctx, name, _ := suite.setupContainer("args-only")

		doc := suite.newContainer(name, containerShellImage)
		doc.ContainerArgs = []string{"/bin/echo", "args-only-ok"}

		suite.applyContainers(ctx, doc)

		suite.assertContainerLogged(ctx, name, "args-only-ok")
	})
}

// TestWorkingDir verifies that workingDir is where the container's process starts.
func (suite *ContainersSuite) TestWorkingDir() {
	ctx, name, _ := suite.setupContainer("workingdir")

	doc := suite.shellContainer(name, "echo cwd=$(pwd)")
	doc.ContainerWorkingDir = "/tmp"

	suite.applyContainers(ctx, doc)

	logs := suite.assertContainerLogged(ctx, name, "cwd=")

	suite.Assert().Contains(logs, "cwd=/tmp", "the container did not start in its configured workingDir")
}

// TestRunAs verifies that runAs sets the container process's user and group.
func (suite *ContainersSuite) TestRunAs() {
	ctx, name, _ := suite.setupContainer("runas")

	var (
		uid int32 = 1234
		gid int32 = 5678
	)

	doc := suite.shellContainer(name, "echo uid=$(id -u) gid=$(id -g)")
	doc.RunAsConfig = &containercfg.ContainerRunAs{
		RunAsUID: &uid,
		RunAsGID: &gid,
	}

	suite.applyContainers(ctx, doc)

	logs := suite.assertContainerLogged(ctx, name, "uid=")

	suite.Assert().Contains(logs, fmt.Sprintf("uid=%d gid=%d", uid, gid),
		"the container process did not run as the configured user and group")
}

// TestCapabilities verifies that the capability set the container ends up with follows the profile
// and the explicit add/drop lists.
func (suite *ContainersSuite) TestCapabilities() {
	// Reading CapEff rather than probing a privileged operation: it is the effective set itself, so
	// the assertion cannot pass for an unrelated reason.
	const readCapEff = "grep '^CapEff:' /proc/self/status"

	suite.Run("privileged-has-capabilities", func() {
		ctx, name, _ := suite.setupContainer("caps-privileged")

		doc := suite.shellContainer(name, readCapEff)
		doc.SecurityConfig = &containercfg.ContainerSecurity{
			SecurityProfile: configcontainer.ContainerSecurityProfilePrivileged,
		}

		suite.applyContainers(ctx, doc)

		logs := suite.assertContainerLogged(ctx, name, "CapEff:")
		suite.Assert().NotContains(logs, "CapEff:\t0000000000000000",
			"privileged container has an empty effective capability set")
	})

	// drop is applied on top of the profile, so pairing it with privileged is the only way to tell a
	// working drop apart from a profile that had dropped everything anyway.
	suite.Run("drop-all-overrides-privileged", func() {
		ctx, name, _ := suite.setupContainer("caps-drop-all")

		doc := suite.shellContainer(name, readCapEff)
		doc.SecurityConfig = &containercfg.ContainerSecurity{
			SecurityProfile: configcontainer.ContainerSecurityProfilePrivileged,
			SecurityCapabilities: &containercfg.ContainerCapabilities{
				CapabilitiesDropConfig: []string{"ALL"},
			},
		}

		suite.applyContainers(ctx, doc)

		logs := suite.assertContainerLogged(ctx, name, "CapEff:")

		suite.Assert().Contains(logs, "CapEff:\t0000000000000000",
			"dropping ALL did not clear the capability set the privileged profile granted")
	})

	suite.Run("restricted-has-no-capabilities", func() {
		ctx, name, _ := suite.setupContainer("caps-restricted")

		// No security stanza at all: restricted is the default.
		suite.applyContainers(ctx, suite.shellContainer(name, readCapEff))

		logs := suite.assertContainerLogged(ctx, name, "CapEff:")

		suite.Assert().Contains(logs, "CapEff:\t0000000000000000",
			"the restricted profile left capabilities in the effective set")
	})
}

// TestSecurityProfileRootfs verifies that the restricted profile mounts the rootfs read-only and that
// privileged does not.
func (suite *ContainersSuite) TestSecurityProfileRootfs() {
	suite.assertSecurityProfileProbe("rootfs",
		"touch /rootfs-probe && echo WRITABLE || echo READONLY",
		"READONLY", "WRITABLE",
		"the restricted profile should mount the rootfs read-only",
		"the privileged profile should not mount the rootfs read-only",
	)
}

// TestSecurityProfileSysfs verifies that the restricted profile mounts sysfs read-only and that
// privileged mounts it read-write.
//
// Reading the mount options rather than writing a sysfs attribute: a successful write would change
// the host, since sysfs is not namespaced.
func (suite *ContainersSuite) TestSecurityProfileSysfs() {
	suite.assertSecurityProfileProbe("sysfs",
		`awk '$2 == "/sys" { split($4, o, ","); print "SYSFS_" toupper(o[1]) }' /proc/mounts`,
		"SYSFS_RO", "SYSFS_RW",
		"the restricted profile should mount sysfs read-only",
		"the privileged profile should mount sysfs read-write",
	)
}

// assertSecurityProfileProbe runs probe in a restricted and in a privileged container, and asserts
// that each logs its expected marker. The probe must print exactly one of the two markers.
func (suite *ContainersSuite) assertSecurityProfileProbe(
	prefix, probe, restrictedWant, privilegedWant, restrictedMsg, privilegedMsg string,
) {
	suite.Run("restricted", func() {
		ctx, name, _ := suite.setupContainer(prefix + "-restricted")

		suite.applyContainers(ctx, suite.shellContainer(name, probe))

		logs := suite.assertContainerLogged(ctx, name, restrictedWant, privilegedWant)

		suite.Assert().Contains(logs, restrictedWant, restrictedMsg)
	})

	suite.Run("privileged", func() {
		ctx, name, _ := suite.setupContainer(prefix + "-privileged")

		doc := suite.shellContainer(name, probe)
		doc.SecurityConfig = &containercfg.ContainerSecurity{
			SecurityProfile: configcontainer.ContainerSecurityProfilePrivileged,
		}

		suite.applyContainers(ctx, doc)

		logs := suite.assertContainerLogged(ctx, name, restrictedWant, privilegedWant)

		suite.Assert().Contains(logs, privilegedWant, privilegedMsg)
	})
}

// TestResourceLimits verifies that resource limits reach the instance spec and the container's cgroup.
func (suite *ContainersSuite) TestResourceLimits() {
	ctx, name, _ := suite.setupContainer("resources")

	const (
		memoryLimit      = "64Mi"
		memoryLimitBytes = 64 * 1024 * 1024
		swapLimit        = "16Mi"
		swapLimitBytes   = 16 * 1024 * 1024
		cpuLimit         = "500m"
		// 500 millicores as a quota over the 100000us period the runner uses.
		cpuMaxWant = "50000 100000"
	)

	doc := suite.newContainer(name, containerPauseImage)
	doc.ResourcesConfig = &containercfg.ContainerResources{
		Limits: &containercfg.ContainerResourceLimits{
			CPU:    cpuLimit,
			Memory: memoryLimit,
			Swap:   swapLimit,
		},
	}

	suite.applyContainers(ctx, doc)

	instanceID, _ := suite.assertContainerRunning(ctx, name, "with resource limits")

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), instanceID,
		func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
			asrt.EqualValues(memoryLimitBytes, instance.TypedSpec().Resources.MemoryLimit)
			asrt.NotZero(instance.TypedSpec().Resources.CPULimit)

			if asrt.NotNil(instance.TypedSpec().Resources.SwapLimit) {
				asrt.EqualValues(swapLimitBytes, *instance.TypedSpec().Resources.SwapLimit)
			}
		})

	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Log("skipping the cgroup assertions: cgroups are nested in container mode, so the " +
			"container's cgroup is not at a path this test can predict")

		return
	}

	// The cgroup is keyed by container, not by instance, so the path is stable across restarts.
	cgroupPath := fmt.Sprintf("/sys/fs/cgroup/%s/%s", constants.CgroupTalosContainersRoot, name)

	suite.Assert().Equal(fmt.Sprintf("%d", memoryLimitBytes), suite.ReadFile(ctx, cgroupPath+"/memory.max"))
	suite.Assert().Equal(fmt.Sprintf("%d", swapLimitBytes), suite.ReadFile(ctx, cgroupPath+"/memory.swap.max"))
	suite.Assert().Equal(cpuMaxWant, suite.ReadFile(ctx, cgroupPath+"/cpu.max"))
}

// TestNetworkModes verifies that the network mode decides both the container's network namespace and
// whether the host's network files are visible in it.
func (suite *ContainersSuite) TestNetworkModes() {
	// Interfaces are listed rather than counted, and routes are reported separately: a count says
	// nothing about why it is wrong, and /proc/net/dev being unreadable would look the same as a
	// namespace that was never created. END terminates the list so it can be matched as a whole.
	//
	// An empty namespace is not just lo: the kernel auto-creates a fallback tunnel device per
	// namespace for every tunnel module that is loaded, so lo, tunl0, sit0 and ip6tnl0 all show up in
	// one. What distinguishes it is that none of the host's own interfaces are there, and that it has
	// no routes.
	const describeNetwork = "echo IFACES=$(grep -o '^[^:]*:' /proc/net/dev | tr -d ' :' | tr '\n' ',')END; " +
		"if [ \"$(wc -l < /proc/net/route)\" -le 1 ]; then echo NO-ROUTES; else echo HAS-ROUTES; fi; " +
		"grep -q nameserver /etc/resolv.conf 2>/dev/null && echo RESOLVCONF || echo NO-RESOLVCONF"

	suite.Run("none", func() {
		ctx, name, _ := suite.setupContainer("net-none")

		hostOnly := suite.hostOnlyInterfaces(ctx)

		// No network stanza at all: none is the default.
		suite.applyContainers(ctx, suite.shellContainer(name, describeNetwork))

		logs := suite.assertContainerLogged(ctx, name, "IFACES=")

		for _, iface := range hostOnly {
			suite.Assert().NotContains(logs, iface+",",
				"a container in network mode none should not see the host interface %q", iface)
		}

		suite.Assert().Contains(logs, "NO-ROUTES",
			"a container in network mode none should have no routes")
		suite.Assert().Contains(logs, "NO-RESOLVCONF",
			"a container with its own network namespace should not see the host's resolv.conf")
	})

	suite.Run("host", func() {
		ctx, name, _ := suite.setupContainer("net-host")

		hostOnly := suite.hostOnlyInterfaces(ctx)

		doc := suite.shellContainer(name, describeNetwork)
		doc.NetworkConfig = &containercfg.ContainerNetwork{
			NetworkMode: configcontainer.ContainerNetworkModeHost,
		}

		suite.applyContainers(ctx, doc)

		logs := suite.assertContainerLogged(ctx, name, "IFACES=")

		// Any one of them is proof that the namespace is shared, and only one is asserted because the
		// host's list includes pod veth devices which come and go: requiring all of them would fail
		// whenever one was torn down between reading the list and running the container.
		seesHostInterface := slices.ContainsFunc(hostOnly, func(iface string) bool {
			return strings.Contains(logs, iface+",")
		})

		suite.Assert().True(seesHostInterface,
			"a container sharing the host network saw none of the host's own interfaces %v", hostOnly)

		suite.Assert().Contains(logs, "HAS-ROUTES",
			"a container sharing the host network should see the host's routes")
		suite.Assert().Contains(logs, "RESOLVCONF",
			"a container sharing the host network should see the host's resolv.conf")
	})
}

// perNamespaceInterfaces are created by the kernel in every network namespace for which the matching
// tunnel module is loaded, so their presence says nothing about which namespace a process is in.
var perNamespaceInterfaces = []string{
	"lo", "tunl0", "sit0", "ip6tnl0", "erspan0", "gre0", "gretap0", "ip_vti0", "ip6_vti0", "ip6gre0",
}

// hostOnlyInterfaces returns the node's network interfaces which would not exist in an empty
// namespace, i.e. the ones whose presence in a container proves it shares the host's namespace.
//
// Read from the node rather than hardcoded because the set depends on the platform and on what the
// cluster has brought up.
func (suite *ContainersSuite) hostOnlyInterfaces(ctx context.Context) []string {
	var hostOnly []string

	for line := range strings.SplitSeq(suite.ReadFile(ctx, "/proc/net/dev"), "\n") {
		name, _, found := strings.Cut(line, ":")
		if !found {
			// One of the two header lines, neither of which contains a colon.
			continue
		}

		name = strings.TrimSpace(name)

		if name == "" || slices.Contains(perNamespaceInterfaces, name) {
			continue
		}

		hostOnly = append(hostOnly, name)
	}

	suite.Require().NotEmpty(hostOnly, "the node reports no interfaces of its own, so nothing distinguishes the namespaces")

	suite.T().Logf("host-only interfaces: %v", hostOnly)

	return hostOnly
}

// TestTmpfsMount verifies that a tmpfs mount is present and writable even under the restricted
// profile, which mounts the rootfs read-only.
func (suite *ContainersSuite) TestTmpfsMount() {
	ctx, name, _ := suite.setupContainer("tmpfs")

	const destination = "/scratch"

	doc := suite.shellContainer(name, "echo tmpfs-ok > "+destination+"/probe && cat "+destination+"/probe")
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			TmpfsMount: &containercfg.TmpfsMount{
				MountDestination: destination,
				MountSize:        "1Mi",
			},
		},
	}

	suite.applyContainers(ctx, doc)

	suite.assertContainerLogged(ctx, name, "tmpfs-ok")

	instanceID, _ := suite.assertNewestInstanceID(ctx, name)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), instanceID,
		func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
			if asrt.Len(instance.TypedSpec().Mounts, 1) {
				asrt.Equal(destination, instance.TypedSpec().Mounts[0].Destination)
			}
		})
}

// TestHostPathMount verifies that a host path is bind-mounted into the container, and that it is
// read-only unless asked otherwise.
func (suite *ContainersSuite) TestHostPathMount() {
	ctx, name, _ := suite.setupContainer("hostpath")

	// /var/log is on the ephemeral partition, which is the one label the container's SELinux domain
	// is granted access to; /etc would be refused.
	const (
		source      = "/var/log"
		destination = "/host-log"
	)

	doc := suite.shellContainer(name,
		"ls "+destination+" > /dev/null && echo MOUNTED; "+
			"touch "+destination+"/probe 2>/dev/null && echo WRITABLE || echo READONLY")
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			HostPathMount: &containercfg.HostPathMount{
				MountSource:      source,
				MountDestination: destination,
			},
		},
	}

	suite.applyContainers(ctx, doc)

	logs := suite.assertContainerLogged(ctx, name, "READONLY", "WRITABLE")

	suite.Assert().Contains(logs, "MOUNTED", "host path was not mounted into the container")

	// Host path mounts default to rw.
	suite.Assert().Contains(logs, "WRITABLE", "host path mount is read-only despite no ro option being given")
}

// TestDependsOnPaths verifies that a container declaring dependsOn.paths waits for the path to exist
// and starts once it does.
func (suite *ContainersSuite) TestDependsOnPaths() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	// Redeclaring the container against a path that exists, rather than making the original path
	// appear. This covers the gate being re-evaluated when the spec changes; the subtest below covers
	// an unmet path being noticed without any config change.
	suite.Run("gate-opens-on-config-change", func() {
		ctx, name, _ := suite.setupContainer("dependson-repoint")

		doc := suite.newContainer(name, containerPauseImage)
		doc.DependsOnConfig = &containercfg.ContainerDependsOn{
			PathsConfig: []string{fmt.Sprintf("/var/%s-absent", name)},
		}

		suite.applyContainers(ctx, doc)

		// Waiting for the image first matters: without it "no instance yet" would also be true of a
		// container held back by a pull still in flight, and the test would pass without the path
		// gate doing anything.
		suite.assertImageReady(ctx, name)
		suite.assertNoInstance(ctx, name)

		suite.T().Logf("redeclaring container %q against a path that exists", name)

		// The document has to be removed rather than patched over: strategic merge appends list
		// fields unless they are tagged merge:"replace", and none of the container config's lists
		// are, so patching a new paths list would leave the unmet path in place alongside it. Paths
		// are an AND, so the container would stay pending forever.
		suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, name)

		// /etc/hosts is managed by EtcFileController and is present on any node serving the API.
		// Note that /etc/hostname is not: Talos does not manage one, despite what the controller's
		// doc comment says.
		doc = suite.newContainer(name, containerPauseImage)
		doc.DependsOnConfig = &containercfg.ContainerDependsOn{
			PathsConfig: []string{"/etc/hosts"},
		}

		suite.applyContainers(ctx, doc)

		suite.assertContainerRunning(ctx, name, "after being redeclared against a path that exists")
	})

	// The waiting container's own configuration never changes here: the path it waits for is created
	// on the host by a second container. That is the case an operator actually has -- something else
	// produces the file -- and it cannot pass by the config change alone.
	suite.Run("gate-opens-when-the-path-appears", func() {
		ctx, base, node := suite.setupContainer("dependson-appears")

		waiter, writer := base+"-waiter", base+"-writer"

		// Under /var because that is the ephemeral mount, the one tree a container's SELinux domain
		// is granted write access to. The marker outlives the test; the name is unique to this run.
		markerName := base + "-marker"
		marker := "/var/" + markerName

		suite.T().Cleanup(func() {
			suite.RemoveMachineConfigDocumentsByName(
				client.WithNode(context.Background(), node),
				containercfg.ContainerConfigKind, waiter, writer,
			)
		})

		waiterDoc := suite.newContainer(waiter, containerPauseImage)
		waiterDoc.DependsOnConfig = &containercfg.ContainerDependsOn{
			PathsConfig: []string{marker},
		}

		suite.applyContainers(ctx, waiterDoc)

		suite.assertImageReady(ctx, waiter)
		suite.assertNoInstance(ctx, waiter)

		suite.T().Logf("starting container %q to create %s", writer, marker)

		const writerMountPoint = "/hostvar"

		writerDoc := suite.shellContainer(writer,
			"touch "+writerMountPoint+"/"+markerName+" && echo marker-created")
		writerDoc.MountsConfig = []containercfg.ContainerMount{
			{
				HostPathMount: &containercfg.HostPathMount{
					MountSource:      "/var",
					MountDestination: writerMountPoint,
				},
			},
		}

		suite.applyContainers(ctx, writerDoc)

		suite.assertContainerLogged(ctx, writer, "marker-created")

		suite.assertContainerRunning(ctx, waiter, "after the path it waits for appeared")
	})
}

// TestDependsOnContainers verifies that a container declaring dependsOn.containers waits for the
// named container to report healthy, and starts once it does.
func (suite *ContainersSuite) TestDependsOnContainers() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	ctx, base, node := suite.setupContainer("dependson-containers")

	dependency, waiter, writer := base+"-dep", base+"-waiter", base+"-writer"

	// The dependency has to be configured from the start. A dependsOn.containers entry naming a
	// container that is not in the configuration is rejected by config validation.
	markerName := base + "-marker"
	marker := "/var/" + markerName

	suite.T().Cleanup(func() {
		suite.RemoveMachineConfigDocumentsByName(
			client.WithNode(context.Background(), node),
			containercfg.ContainerConfigKind, dependency, waiter, writer,
		)
	})

	dependencyDoc := suite.newContainer(dependency, containerPauseImage)
	dependencyDoc.DependsOnConfig = &containercfg.ContainerDependsOn{
		PathsConfig: []string{marker},
	}

	waiterDoc := suite.newContainer(waiter, containerPauseImage)
	waiterDoc.DependsOnConfig = &containercfg.ContainerDependsOn{
		ContainersConfig: []string{dependency},
	}

	suite.applyContainers(ctx, dependencyDoc, waiterDoc)

	// Wait for the image. We want the instance to be blocked by dependsOn.containers, not by an unfinished image pull.
	suite.assertImageReady(ctx, waiter)
	suite.assertNoInstance(ctx, dependency)
	suite.assertNoInstance(ctx, waiter)

	suite.T().Logf("starting container %q to create %s, which unblocks %q", writer, marker, dependency)

	const writerMountPoint = "/hostvar"

	writerDoc := suite.shellContainer(writer,
		"touch "+writerMountPoint+"/"+markerName+" && echo marker-created")
	writerDoc.MountsConfig = []containercfg.ContainerMount{
		{
			HostPathMount: &containercfg.HostPathMount{
				MountSource:      "/var",
				MountDestination: writerMountPoint,
			},
		},
	}

	suite.applyContainers(ctx, writerDoc)

	suite.assertContainerLogged(ctx, writer, "marker-created")

	suite.assertContainerRunning(ctx, dependency, "the dependency, after the path it waits for appeared")

	suite.assertContainerRunning(ctx, waiter, "after its dependency became healthy")
}

// TestDependsOnFailingContainer verifies that a container declaring dependsOn.containers stays
// blocked while the named dependency keeps failing to start, rather than racing into Running the
// instant the dependency's process is forked and before its exit is observed.
func (suite *ContainersSuite) TestDependsOnFailingContainer() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	ctx, base, node := suite.setupContainer("dependson-failing")

	dependency, waiter := base+"-dep", base+"-waiter"

	suite.T().Cleanup(func() {
		suite.RemoveMachineConfigDocumentsByName(
			client.WithNode(context.Background(), node),
			containercfg.ContainerConfigKind, dependency, waiter,
		)
	})

	// Fails immediately every run: mirrors a misconfigured entrypoint (bad CLI flag, missing
	// capability) rather than a slow-starting one. shellContainer restarts it in a crash loop.
	dependencyDoc := suite.shellContainer(dependency, "exit 1")

	waiterDoc := suite.newContainer(waiter, containerPauseImage)
	waiterDoc.DependsOnConfig = &containercfg.ContainerDependsOn{
		ContainersConfig: []string{dependency},
	}

	suite.applyContainers(ctx, dependencyDoc, waiterDoc)

	// Wait for the image first: without it "no instance yet" would also be true of a pull still in
	// flight, and the test would pass without the dependsOn gate doing anything.
	suite.assertImageReady(ctx, waiter)

	// The dependency never reports sustained health -- it restarts in a crash loop. Held long enough
	// to span several restart/backoff cycles, so a start that only wins the race after one particular
	// restart is still caught, not just the first one.
	suite.assertNoInstanceFor(ctx, waiter, 30*time.Second)

	// And the reason is legible: the waiter names the dependency it is held on rather than just
	// sitting in pending with nothing to explain it.
	suite.assertContainerStatus(ctx, waiter, "held on its failing dependency",
		func(status *containers.ContainerStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerStatePending, status.State,
				"state is %s (error %q)", status.State, status.Error)
			asrt.Contains(status.WaitingFor, "container: "+dependency)
		})
}

// TestMultipleContainers verifies that containers declared side by side are independent of each other.
func (suite *ContainersSuite) TestMultipleContainers() {
	ctx, base, node := suite.setupContainer("multi")

	first, second := base+"-a", base+"-b"

	suite.applyContainers(ctx,
		suite.newContainer(first, containerPauseImage),
		suite.newContainer(second, containerPauseImage),
	)

	suite.T().Cleanup(func() {
		suite.RemoveMachineConfigDocumentsByName(
			client.WithNode(context.Background(), node),
			containercfg.ContainerConfigKind, first, second,
		)
	})

	suite.assertContainerRunning(ctx, first, "first of two")
	secondID, secondInstance := suite.assertContainerRunning(ctx, second, "second of two")

	suite.T().Logf("removing container config %q, leaving %q alone", first, second)

	suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, first)

	rtestutils.AssertNoResource[*containers.ContainerSpec](ctx, suite.T(), suite.Client.State(), first)
	suite.assertNoContainerdContainer(ctx, first)

	// The surviving container is not merely still declared: it is the same running instance, so
	// removing its neighbor did not restart it.
	survivingID, survivingInstance := suite.assertContainerRunning(ctx, second, "after its neighbor was removed")

	suite.Assert().Equal(secondID, survivingID, "container %q was replaced when %q was removed", second, first)
	suite.Assert().Equal(secondInstance.PID, survivingInstance.PID,
		"container %q was restarted when %q was removed", second, first)
}

// TestLogs verifies that a container's output is readable through the logs API under the name the
// container runtime registers it as.
func (suite *ContainersSuite) TestLogs() {
	ctx, name, _ := suite.setupContainer("logs")

	const marker = "log-line-marker"

	suite.applyContainers(ctx, suite.shellContainer(name, "echo "+marker))

	suite.assertContainerLogged(ctx, name, marker)

	// The log is registered under a prefixed ID so it cannot collide with a service of the same name.
	stream, err := suite.Client.Logs(ctx, constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD,
		name, false, -1)
	suite.Require().NoError(err)

	reader, err := client.ReadStream(stream)
	if err == nil {
		//nolint:errcheck
		defer reader.Close()

		body, readErr := io.ReadAll(reader)
		if readErr == nil {
			suite.Assert().NotContains(string(body), marker,
				"container logs are reachable under the unprefixed name %q", name)
		}
	}
}

// TestContainerUserVolumeMount covers a container mounting a user volume: the volume is requested and
// held while the container runs, the resolved host path is a working mount inside the container, and
// the hold is given back once the container is gone.
func (suite *ContainersSuite) TestContainerUserVolumeMount() {
	ctx, name, node := suite.setupContainer("uservolume")
	volumeName := suite.setupUserVolume(ctx, node, name, "uservol")

	const destination = "/mnt/data"

	// The write is what proves the resolved host path is really mounted here: the resource assertions
	// below only show what the controllers agreed on, not what the container can reach. Parked with
	// sleep afterwards because a container that exits is restarted, which would race the assertion
	// that it is running.
	doc := suite.shellContainer(name,
		"echo volume-ok > "+destination+"/probe && cat "+destination+"/probe && sleep 3600")
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			VolumeMount: &containercfg.VolumeMount{
				VolumeName:       volumeName,
				MountDestination: destination,
			},
		},
	}

	suite.applyContainers(ctx, doc)

	// The host path is only knowable once the volume is actually mounted, which is what the mount
	// status reports.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), name,
		func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)

			if !asrt.Len(status.TypedSpec().Mounts, 1) {
				return
			}

			asrt.Equal(filepath.Join(constants.UserVolumeMountPoint, volumeName), status.TypedSpec().Mounts[0].Source)
			asrt.Equal(destination, status.TypedSpec().Mounts[0].Destination)
		})

	suite.assertContainerLogged(ctx, name, "volume-ok")

	// The mount gate is what was blocking this before the controller existed.
	suite.assertContainerRunning(ctx, name, "with the user volume mounted")

	requestID := containerMountRequestID(name, volumeName)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), requestID,
		func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
			asrt.False(request.TypedSpec().ReadOnly, "volume was requested read-only despite the rw default")
		})

	// The hold: the finalizer on the mount status is what stops the volume being unmounted from under
	// the running container.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), requestID,
		func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
			asrt.Equal(filepath.Join(constants.UserVolumeMountPoint, volumeName), status.TypedSpec().Target)
			asrt.True(status.Metadata().Finalizers().Has(containerMountControllerName),
				"the mount is not held for the container, finalizers: %v", status.Metadata().Finalizers())
		})

	suite.T().Logf("removing container config %q", name)

	suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, name)

	// The hold has to be given back, or tearing the volume down would block on our finalizer.
	rtestutils.AssertNoResource[*containers.ContainerMountStatus](ctx, suite.T(), suite.Client.State(), name)
	rtestutils.AssertNoResource[*block.VolumeMountRequest](ctx, suite.T(), suite.Client.State(), requestID)

	suite.T().Logf("removing user volume %q", volumeName)

	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.UserVolumeConfigKind, volumeName)

	// And the volume itself is then really unmounted, which is only possible because the hold was
	// released above.
	rtestutils.AssertNoResource[*block.MountStatus](ctx, suite.T(), suite.Client.State(),
		constants.UserVolumePrefix+volumeName)
}

// TestUserVolumeMountWritableByDefault verifies that a user volume mounted without options is
// writable, both as requested of the block subsystem and as the container finds it.
func (suite *ContainersSuite) TestUserVolumeMountWritableByDefault() {
	ctx, name, node := suite.setupContainer("volume-rw")
	volumeName := suite.setupUserVolume(ctx, node, name, "volrw")

	const destination = "/mnt/data"

	// The destination is checked first: the image does not ship it, so a mount that never happened
	// would make the write fail and read as READONLY, passing this test for the wrong reason.
	doc := suite.shellContainer(name,
		"[ -d "+destination+" ] || { echo NOTMOUNTED; exit 0; }; "+
			"touch "+destination+"/probe 2>/dev/null && echo WRITABLE || echo READONLY")
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			VolumeMount: &containercfg.VolumeMount{
				VolumeName:       volumeName,
				MountDestination: destination,
			},
		},
	}

	suite.applyContainers(ctx, doc)

	logs := suite.assertContainerLogged(ctx, name, "READONLY", "WRITABLE", "NOTMOUNTED")

	suite.Assert().NotContains(logs, "NOTMOUNTED", "the user volume was not mounted into the container")
	suite.Assert().Contains(logs, "WRITABLE", "user volume mount is read-only despite no ro option being given")

	// The other half of the contract: writable has to reach the block subsystem as well, so that this
	// container counts as a writable holder of the volume.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), containerMountRequestID(name, volumeName),
		func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
			asrt.False(request.TypedSpec().ReadOnly, "volume %q was requested read-only", volumeName)
		})
}

// TestRawVolumeDevice verifies that a restricted container can do direct IO on an encrypted raw volume
// handed to it as a device, and can't open another block device, and that the raw volume is held while
// the container runs.
func (suite *ContainersSuite) TestRawVolumeDevice() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping test for non-qemu provisioner")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machinetype.TypeWorker)

	userDisks := suite.UserDisks(suite.ctx, node)
	if len(userDisks) < 1 {
		suite.T().Skipf("not enough user disks on %s: %q", node, userDisks)
	}

	var (
		rawVolumeName string
		partition     string
	)

	// Registered before setupContainerOn, so it runs after the container is removed: a raw volume
	// handed to a container can't be removed on its own.
	suite.T().Cleanup(func() {
		if rawVolumeName == "" {
			return
		}

		cleanupCtx := client.WithNode(context.Background(), node)

		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, blockcfg.RawVolumeConfigKind, rawVolumeName)

		if partition == "" {
			return
		}

		suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
			assert.NoError(collect, suite.Client.BlockDeviceWipe(cleanupCtx, &storage.BlockDeviceWipeRequest{
				Devices: []*storage.BlockDeviceWipeDescriptor{
					{
						Device:        filepath.Base(partition),
						Method:        storage.BlockDeviceWipeDescriptor_FAST,
						DropPartition: true,
					},
				},
			}))
		}, time.Minute, time.Second)
	})

	ctx, name := suite.setupContainerOn("rawdevice", node)

	disk, err := safe.StateGetByID[*block.Disk](ctx, suite.Client.State(), filepath.Base(userDisks[0]))
	suite.Require().NoError(err)
	suite.Require().NotEmpty(disk.TypedSpec().Symlinks)

	rawVolumeName = fmt.Sprintf("itv-raw-%04x", rand.Int31())
	rawVolumeID := constants.RawVolumePrefix + rawVolumeName

	rawVolume := blockcfg.NewRawVolumeConfigV1Alpha1()
	rawVolume.MetaName = rawVolumeName
	rawVolume.ProvisioningSpec.DiskSelectorSpec.Match = cel.MustExpression(
		cel.ParseBooleanExpression(fmt.Sprintf("'%s' in disk.symlinks", disk.TypedSpec().Symlinks[0]), celenv.DiskLocator()),
	)
	rawVolume.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("100MiB")
	rawVolume.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("1GiB")
	rawVolume.EncryptionSpec = blockcfg.EncryptionSpec{
		EncryptionProvider: block.EncryptionProviderLUKS2,
		EncryptionKeys: []blockcfg.EncryptionKey{
			{
				KeySlot:   0,
				KeyStatic: &blockcfg.EncryptionKeyStatic{KeyData: "topsecretpassphrase"},
			},
		},
	}

	// The disk holding the raw volume is mounted in as well: its node is there, but the device cgroup
	// must refuse to open it.
	doc := suite.shellContainer(name, strings.Join([]string{
		"dd if=/dev/zero of=/dev/data bs=4096 count=1 oflag=direct",
		"dd if=/dev/data of=/dev/null bs=4096 count=1 iflag=direct",
		"echo device-ok",
		"dd if=/dev/other of=/dev/null bs=4096 count=1 || echo other-refused",
		"sleep 3600",
	}, " && "))
	doc.DevicesConfig = []containercfg.ContainerDevice{
		{
			RawVolumeDevice: &containercfg.RawVolumeDevice{
				VolumeName:        rawVolumeName,
				DeviceDestination: "/dev/data",
			},
		},
	}
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			HostPathMount: &containercfg.HostPathMount{
				MountSource:      userDisks[0],
				MountDestination: "/dev/other",
			},
		},
	}

	suite.PatchMachineConfig(ctx, rawVolume, doc)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), rawVolumeID,
		func(status *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, status.TypedSpec().Phase)
		}, rtestutils.WithReportInterval(30*time.Second))

	volumeStatus, err := safe.StateGetByID[*block.VolumeStatus](ctx, suite.Client.State(), rawVolumeID)
	suite.Require().NoError(err)

	partition = volumeStatus.TypedSpec().Location

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), name,
		func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)

			if !asrt.Len(status.TypedSpec().Devices, 1) {
				return
			}

			// The unlocked device, not the encrypted partition.
			asrt.Equal(volumeStatus.TypedSpec().MountLocation, status.TypedSpec().Devices[0].Source)
			asrt.NotEqual(partition, status.TypedSpec().Devices[0].Source)
		})

	suite.assertContainerLogged(ctx, name, "device-ok", "Operation not permitted", "other-refused")
	suite.assertContainerRunning(ctx, name, "with the raw volume device")

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), rawVolumeID,
		func(status *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.True(status.Metadata().Finalizers().Has(containerMountControllerName+"/"+name),
				"the raw volume is not held for the container, finalizers: %v", status.Metadata().Finalizers())
		})
}

// TestVolumeMountGate verifies that a container mounting a volume nothing declares is rejected when
// the configuration is applied, that a container whose volume is declared but not mounted is
// withheld, and that it starts once the name resolves to a volume which mounts.
//
// The pause image is used because it is already on every node, so nothing here waits on a pull; what
// is being timed is the gate, not the registry.
func (suite *ContainersSuite) TestVolumeMountGate() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	ctx, name, node := suite.setupContainer("volume-gate")

	volumeName := fmt.Sprintf("itv-gate-%04x", rand.Int31())

	doc := suite.newContainer(name, containerPauseImage)
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			VolumeMount: &containercfg.VolumeMount{
				VolumeName:       volumeName,
				MountDestination: "/mnt/data",
			},
		},
	}

	suite.T().Logf("verifying container %q naming the undeclared volume %q is rejected", name, volumeName)

	cfg, err := suite.ReadConfigFromNode(ctx)
	suite.Require().NoError(err)

	marshaled, err := yaml.Marshal(doc)
	suite.Require().NoError(err)

	patch, err := configpatcher.LoadPatch(marshaled)
	suite.Require().NoError(err)

	patched, err := configpatcher.Apply(configpatcher.WithConfig(cfg), []configpatcher.Patch{patch})
	suite.Require().NoError(err)

	patchedBytes, err := patched.Bytes()
	suite.Require().NoError(err)

	_, err = suite.Client.ApplyConfiguration(ctx, &machine.ApplyConfigurationRequest{
		Data: patchedBytes,
		Mode: machine.ApplyConfigurationRequest_AUTO,
	})
	suite.Require().ErrorContains(err, fmt.Sprintf("declares volume %q", volumeName))

	suite.T().Logf("verifying container %q waits for the volume %q which is never mounted", name, volumeName)

	suite.declareUnmountableVolume(ctx, node, name, volumeName, doc)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), name,
		func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
			asrt.False(status.TypedSpec().Ready, "mounts are ready without the volume being mounted")
			asrt.Contains(status.TypedSpec().Error, volumeName)
		})

	// The user-visible half of the gate: an unmounted volume withholds the container itself.
	suite.assertNoInstance(ctx, name)

	suite.replaceWithUserVolume(ctx, node, volumeName)

	suite.T().Logf("verifying container %q starts once the volume %q is mounted", name, volumeName)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), name,
		func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)

			if asrt.Len(status.TypedSpec().Mounts, 1) {
				asrt.Equal(constants.UserVolumePrefix+volumeName, status.TypedSpec().Mounts[0].VolumeID)
			}
		})

	suite.assertContainerRunning(ctx, name, "once the volume was mounted")
}

// TestVolumeRemovalRejectedWhileMounted verifies that a volume a container mounts cannot be removed
// from the configuration, and that the rejected apply leaves the running container and its volume
// alone.
func (suite *ContainersSuite) TestVolumeRemovalRejectedWhileMounted() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	ctx, name, node := suite.setupContainer("volume-reject")
	volumeName := suite.setupUserVolume(ctx, node, name, "volrej")

	instanceID := suite.startWithUserVolume(ctx, name, volumeName)

	suite.T().Logf("verifying user volume %q mounted by container %q cannot be removed", volumeName, name)

	cfg, err := suite.ReadConfigFromNode(ctx)
	suite.Require().NoError(err)

	withoutVolume, err := cfgcontainer.New(xslices.Filter(cfg.Documents(), func(doc configcontainer.Document) bool {
		named, ok := doc.(configcontainer.NamedDocument)

		return !ok || doc.Kind() != blockcfg.UserVolumeConfigKind || named.Name() != volumeName
	})...)
	suite.Require().NoError(err)

	withoutVolumeBytes, err := withoutVolume.Bytes()
	suite.Require().NoError(err)

	_, err = suite.Client.ApplyConfiguration(ctx, &machine.ApplyConfigurationRequest{
		Data: withoutVolumeBytes,
		Mode: machine.ApplyConfigurationRequest_AUTO,
	})
	suite.Require().ErrorContains(err, fmt.Sprintf("declares volume %q", volumeName))

	// Nothing was applied, so the same instance keeps running with the volume still mounted.
	stillRunningID, _ := suite.assertContainerRunning(ctx, name, "after the rejected volume removal")
	suite.Assert().Equal(instanceID, stillRunningID, "the container was restarted by a rejected apply")

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), constants.UserVolumePrefix+volumeName,
		func(*block.MountStatus, *assert.Assertions) {})
}

// TestVolumeHeldAcrossKindChange verifies that a volume a running container mounts stays mounted
// when the configuration stops pointing the volume name at it, and is unmounted once the container
// goes away.
//
// Swapping the user volume for an existing volume of the same name is the one way to take the volume
// away from the container which validation allows: the name stays declared, but resolves to a
// different volume ID. The existing volume matches no partition, so the swap cannot be mistaken for
// the container moving onto a new volume.
func (suite *ContainersSuite) TestVolumeHeldAcrossKindChange() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	ctx, name, node := suite.setupContainer("volume-held")
	volumeName := suite.setupUserVolume(ctx, node, name, "volheld")

	instanceID := suite.startWithUserVolume(ctx, name, volumeName)

	requestID := containerMountRequestID(name, volumeName)

	suite.replaceWithUnmountableVolume(ctx, node, volumeName)

	// The container now waits on a volume which is never mounted, which is how the swap is known to
	// have been seen by the mount controller.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), name,
		func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
			asrt.False(status.TypedSpec().Ready, "mounts are ready without the volume being mounted")
			asrt.Contains(status.TypedSpec().Error, volumeName)
		})

	suite.T().Logf("verifying user volume %q stays mounted while container %q runs", volumeName, name)

	stillRunningID, _ := suite.assertContainerRunning(ctx, name, "after its volume was swapped")
	suite.Assert().Equal(instanceID, stillRunningID, "the container was restarted by the volume swap")

	// The hold: still on the old mount, which keeps the user volume mounted although nothing declares
	// it any more.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), requestID,
		func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
			asrt.True(status.Metadata().Finalizers().Has(containerMountControllerName),
				"the mount is not held for the container, finalizers: %v", status.Metadata().Finalizers())
		})

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), constants.UserVolumePrefix+volumeName,
		func(*block.MountStatus, *assert.Assertions) {})

	suite.T().Logf("removing container config %q", name)

	suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, name)

	// With the container gone the hold is given back, and the user volume is really unmounted.
	rtestutils.AssertNoResource[*block.VolumeMountRequest](ctx, suite.T(), suite.Client.State(), requestID)
	rtestutils.AssertNoResource[*block.MountStatus](ctx, suite.T(), suite.Client.State(),
		constants.UserVolumePrefix+volumeName)
}

// startWithUserVolume applies a container mounting the user volume, waits for it to run with the
// mount held, and returns the ID of its running instance.
func (suite *ContainersSuite) startWithUserVolume(ctx context.Context, name, volumeName string) resource.ID {
	// Parked with sleep because a container that exits is restarted, which would race the assertions
	// that it keeps running.
	doc := suite.shellContainer(name, "sleep 3600")
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			VolumeMount: &containercfg.VolumeMount{
				VolumeName:       volumeName,
				MountDestination: "/mnt/data",
			},
		},
	}

	suite.applyContainers(ctx, doc)

	instanceID, _ := suite.assertContainerRunning(ctx, name, "with the user volume mounted")

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), containerMountRequestID(name, volumeName),
		func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
			asrt.True(status.Metadata().Finalizers().Has(containerMountControllerName),
				"the mount is not held for the container, finalizers: %v", status.Metadata().Finalizers())
		})

	return instanceID
}

// TestAllowMachinedAccess verifies that security.machinedAccess publishes the container's PID as a
// ServicePID resource and mounts the machined API socket into the container, and that neither
// happens when it is left unset.
func (suite *ContainersSuite) TestAllowMachinedAccess() {
	suite.Run("enabled", func() {
		if testing.Short() {
			suite.T().Skip("skipping the test in short mode")
		}

		ctx, name, _ := suite.setupContainer("allow-machined-enabled")
		script := fmt.Sprintf(
			`if [ -S %s ]; then echo SOCKET_OK; else echo SOCKET_MISSING; fi
sleep 3600`,
			constants.MachineSocketPath,
		)

		doc := suite.shellContainer(name, script)
		doc.SecurityConfig = &containercfg.ContainerSecurity{
			SecurityProfile:        configcontainer.ContainerSecurityProfilePrivileged,
			SecurityMachinedAccess: true,
		}

		suite.applyContainers(ctx, doc)

		_, instance := suite.assertContainerRunning(ctx, name, "with security.machinedAccess")

		logs := suite.assertContainerLogged(ctx, name, "SOCKET_OK", "SOCKET_MISSING")

		suite.Assert().Contains(logs, "SOCKET_OK",
			"the machined socket is not mounted into the container: %s", logs)

		rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), constants.ContainerServicePIDPrefix+name,
			func(servicePID *runtimeres.ServicePID, asrt *assert.Assertions) {
				asrt.EqualValues(instance.PID, servicePID.TypedSpec().PID)
			})

		suite.T().Logf("removing container config %q", name)

		suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, name)

		rtestutils.AssertNoResource[*runtimeres.ServicePID](ctx, suite.T(), suite.Client.State(), constants.ContainerServicePIDPrefix+name)
	})

	suite.Run("disabled", func() {
		ctx, name, _ := suite.setupContainer("allow-machined-disabled")

		script := fmt.Sprintf(
			`ls %s >/dev/null 2>&1 && echo SOCKET_FOUND || echo SOCKET_ABSENT
sleep 3600`,
			constants.MachineSocketPath,
		)

		suite.applyContainers(ctx, suite.shellContainer(name, script))

		suite.assertContainerRunning(ctx, name, "without security.machinedAccess")

		logs := suite.assertContainerLogged(ctx, name, "SOCKET_ABSENT", "SOCKET_FOUND")

		suite.Assert().Contains(logs, "SOCKET_ABSENT",
			"the machined socket must not be mounted without security.machinedAccess")

		rtestutils.AssertNoResource[*runtimeres.ServicePID](ctx, suite.T(), suite.Client.State(), constants.ContainerServicePIDPrefix+name)
	})
}

// TestAllowMachinedSocketConnect verifies that a container granted security.machinedAccess can
// actually connect() to the machined socket, not merely see it.
func (suite *ContainersSuite) TestAllowMachinedSocketConnect() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	ctx, name, _ := suite.setupContainer("allow-machined-connect")

	script := fmt.Sprintf(
		`out=$(timeout 10 socat -u OPEN:/dev/null UNIX-CONNECT:%s 2>&1); rc=$?
echo "CONNECT rc=$rc out=$out"
sleep 3600`,
		constants.MachineSocketPath,
	)

	doc := suite.socatContainer(name, script)
	doc.SecurityConfig = &containercfg.ContainerSecurity{
		SecurityProfile:        configcontainer.ContainerSecurityProfilePrivileged,
		SecurityMachinedAccess: true,
	}

	suite.applyContainers(ctx, doc)

	suite.assertContainerRunning(ctx, name, "with security.machinedAccess")

	logs := suite.assertContainerLogged(ctx, name, "CONNECT rc=")

	suite.Assert().Contains(logs, "CONNECT rc=0",
		"the container could not connect to the machined socket: %s", logs)
}

// setupContainer returns a node-scoped context and a container name unique to this test, and registers
// removal of that container's config. Registering the cleanup here rather than after the container is
// applied means a failure part-way through does not leave the container running for the rest of the
// run.
//
// The volumes declared for the container are removed in the same apply: a container may not name a
// volume which is not declared, so neither can be removed on its own while the other is still there.
func (suite *ContainersSuite) setupContainer(purpose string) (context.Context, string, string) {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx, name := suite.setupContainerOn(purpose, node)

	return ctx, name, node
}

// setupContainerOn is setupContainer on the given node.
func (suite *ContainersSuite) setupContainerOn(purpose, node string) (context.Context, string) {
	ctx := client.WithNode(suite.ctx, node)

	name := fmt.Sprintf("test-%s-%d", purpose, time.Now().UnixNano())

	suite.T().Logf("using container config %q on node %s", name, node)

	suite.T().Cleanup(func() {
		// suite.ctx is likely canceled by the time this runs.
		cleanupCtx := client.WithNode(context.Background(), node)

		volumes := suite.ownedVolumes[name]
		delete(suite.ownedVolumes, name)

		suite.UpdateMachineConfig(cleanupCtx,
			func(*machine.ApplyConfigurationRequest) {},
			func(cfg config.Provider) (config.Provider, error) {
				return cfgcontainer.New(xslices.Filter(cfg.Documents(), func(doc configcontainer.Document) bool {
					named, ok := doc.(configcontainer.NamedDocument)
					if !ok {
						return true
					}

					switch doc.Kind() {
					case containercfg.ContainerConfigKind:
						return named.Name() != name
					case blockcfg.UserVolumeConfigKind, blockcfg.ExistingVolumeConfigKind, blockcfg.ExternalVolumeConfigKind:
						// Matched by name across the volume kinds, as a test may swap one kind for another.
						return !slices.Contains(volumes, named.Name())
					default:
						return true
					}
				})...)
			})
	})

	return ctx, name
}

// ownVolume records that the volume is removed together with the owning container.
func (suite *ContainersSuite) ownVolume(owner, name string) {
	if suite.ownedVolumes == nil {
		suite.ownedVolumes = map[string][]string{}
	}

	suite.ownedVolumes[owner] = append(suite.ownedVolumes[owner], name)
}

// setupUserVolume declares a user volume unique to this test, owned by the given container, and
// returns its name.
//
// Directory-backed, so it needs no spare disk and these tests run on any cluster, unlike the
// disk-provisioned volumes in the volumes suite.
func (suite *ContainersSuite) setupUserVolume(ctx context.Context, node, owner, purpose string) string {
	// Kept short deliberately: a user volume name becomes a partition label, so it is bounded, unlike
	// the container config names the other tests derive from a timestamp.
	name := fmt.Sprintf("itv-%s-%04x", purpose, rand.Int31())

	suite.T().Logf("declaring user volume %q on node %s", name, node)

	suite.ownVolume(owner, name)

	suite.PatchMachineConfig(ctx, newDirectoryUserVolume(name))

	return name
}

// newDirectoryUserVolume builds a directory-backed user volume document.
func newDirectoryUserVolume(name string) *blockcfg.UserVolumeConfigV1Alpha1 {
	doc := blockcfg.NewUserVolumeConfigV1Alpha1()
	doc.MetaName = name
	doc.VolumeType = new(block.VolumeTypeDirectory)

	return doc
}

// declareUnmountableVolume applies an existing volume which no volume on the node matches, owned by
// the given container, together with the given container documents.
//
// The volume is declared, so the configuration validates, but it is never found and so never
// mounted, which is what covers a container waiting on its volume. The container documents are
// applied in the same patch, as a container may not name a volume which is not declared.
func (suite *ContainersSuite) declareUnmountableVolume(
	ctx context.Context, node, owner, name string, docs ...*containercfg.ContainerConfigV1Alpha1,
) {
	suite.T().Logf("declaring unmountable existing volume %q on node %s", name, node)

	suite.ownVolume(owner, name)

	patches := make([]any, 0, 1+len(docs))
	patches = append(patches, newUnmountableExistingVolume(name))

	for _, containerDoc := range docs {
		patches = append(patches, containerDoc)
	}

	suite.PatchMachineConfig(ctx, patches...)
}

// replaceWithUserVolume swaps the unmountable existing volume declared by declareUnmountableVolume
// for a directory-backed user volume of the same name.
//
// Both documents are swapped in a single apply, so the name stays declared throughout and the
// containers mounting it remain valid. The name now resolves to a different volume ID, so what this
// covers is a configuration change alone waking the mount controller up and releasing the gate.
//
// The user volume keeps the name, so it is removed along with the owning container, like the
// existing volume it replaces.
func (suite *ContainersSuite) replaceWithUserVolume(ctx context.Context, node, name string) {
	suite.T().Logf("replacing existing volume %q with a user volume on node %s", name, node)

	suite.swapVolume(ctx, blockcfg.ExistingVolumeConfigKind, name, newDirectoryUserVolume(name))
}

// replaceWithUnmountableVolume is the reverse of replaceWithUserVolume: it swaps the user volume for
// an existing volume of the same name which no volume on the node matches.
//
// Both documents are swapped in a single apply, so the name stays declared throughout and the
// containers mounting it remain valid, while the user volume itself is no longer declared.
func (suite *ContainersSuite) replaceWithUnmountableVolume(ctx context.Context, node, name string) {
	suite.T().Logf("replacing user volume %q with an unmountable existing volume on node %s", name, node)

	suite.swapVolume(ctx, blockcfg.UserVolumeConfigKind, name, newUnmountableExistingVolume(name))
}

// swapVolume replaces the volume document of the given kind and name with replacement in a single
// apply, so the name stays declared throughout.
func (suite *ContainersSuite) swapVolume(ctx context.Context, kind, name string, replacement configcontainer.Document) {
	suite.UpdateMachineConfig(ctx,
		func(*machine.ApplyConfigurationRequest) {},
		func(cfg config.Provider) (config.Provider, error) {
			documents := xslices.Filter(cfg.Documents(), func(doc configcontainer.Document) bool {
				named, ok := doc.(configcontainer.NamedDocument)

				return !ok || doc.Kind() != kind || named.Name() != name
			})

			return cfgcontainer.New(append(documents, replacement)...)
		})
}

// newUnmountableExistingVolume builds an existing volume document which no volume on the node
// matches, as its selector wants a partition labeled with the volume's own name.
func newUnmountableExistingVolume(name string) *blockcfg.ExistingVolumeConfigV1Alpha1 {
	doc := blockcfg.NewExistingVolumeConfigV1Alpha1()
	doc.MetaName = name
	doc.VolumeDiscoverySpec.VolumeSelectorConfig.Match = cel.MustExpression(
		cel.ParseBooleanExpression(fmt.Sprintf("volume.partition_label == %q", name), celenv.VolumeLocator()),
	)

	return doc
}

// newContainer builds a ContainerConfig document.
func (suite *ContainersSuite) newContainer(name, image string) *containercfg.ContainerConfigV1Alpha1 {
	doc := containercfg.NewContainerConfigV1Alpha1()
	doc.MetaName = name
	doc.ContainerImage = image

	return doc
}

// shellContainer builds a ContainerConfig document that runs command in a shell. The container exits
// when the command does, and is then restarted, so its log accumulates one run's output per restart.
func (suite *ContainersSuite) shellContainer(name, command string) *containercfg.ContainerConfigV1Alpha1 {
	doc := suite.newContainer(name, containerShellImage)
	doc.ContainerEntrypoint = []string{"/bin/sh", "-c"}
	doc.ContainerArgs = []string{command}

	return doc
}

// socatContainer is shellContainer against an image that also carries socat, for the tests that have
// to talk to a unix socket rather than just look at one. The image's own entrypoint is socat, so the
// shell has to be named explicitly here as well.
func (suite *ContainersSuite) socatContainer(name, command string) *containercfg.ContainerConfigV1Alpha1 {
	doc := suite.newContainer(name, containerSocatImage)
	doc.ContainerEntrypoint = []string{"/bin/sh", "-c"}
	doc.ContainerArgs = []string{command}

	return doc
}

// applyContainers applies the given container documents to the node.
func (suite *ContainersSuite) applyContainers(ctx context.Context, docs ...*containercfg.ContainerConfigV1Alpha1) {
	patches := make([]any, 0, len(docs))

	for _, doc := range docs {
		patches = append(patches, doc)
	}

	suite.PatchMachineConfig(ctx, patches...)
}

// assertContainerRunning waits for the newest instance of containerName to report running.
func (suite *ContainersSuite) assertContainerRunning(
	ctx context.Context, containerName, stage string,
) (resource.ID, containers.ContainerInstanceStatusSpec) {
	instanceID, instanceStatus := suite.assertNewestInstance(ctx, containerName, stage,
		func(status *containers.ContainerInstanceStatusSpec, asrt *assert.Assertions) bool {
			return asrt.Equal(containers.ContainerInstancePhaseRunning, status.Phase,
				"phase is %s (error %q)", status.Phase, status.Error) &&
				asrt.NotZero(status.PID, "no PID reported")
		})

	// The aggregate is folded from the instance status by another controller, so it trails it by a
	// pass and needs a wait of its own rather than a bare read here. Asserting it from the one helper
	// every test starting a container goes through is what keeps the user-facing resource covered
	// wherever the instance-level one already is.
	suite.assertContainerStatus(ctx, containerName, stage+", aggregated status",
		func(status *containers.ContainerStatusSpec, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerStateRunning, status.State,
				"state is %s (error %q)", status.State, status.Error)
			asrt.Equal(containers.ContainerHealthHealthy, status.Health)
			asrt.NotZero(status.PID, "no PID reported")
		})

	return instanceID, instanceStatus
}

// assertNewestInstanceID waits for any instance of containerName to be reported and returns the newest.
func (suite *ContainersSuite) assertNewestInstanceID(
	ctx context.Context, containerName string,
) (resource.ID, containers.ContainerInstanceStatusSpec) {
	return suite.assertNewestInstance(ctx, containerName, "any instance",
		func(*containers.ContainerInstanceStatusSpec, *assert.Assertions) bool { return true })
}

// assertNewestInstance waits until the newest instance status of containerName satisfies check, and
// returns it.
//
// The newest generation is picked rather than a predicted ID because generations are spent on restarts
// as well as on configuration changes, so which one is current cannot be computed up front.
func (suite *ContainersSuite) assertNewestInstance(
	ctx context.Context,
	containerName, stage string,
	check func(*containers.ContainerInstanceStatusSpec, *assert.Assertions) bool,
) (resource.ID, containers.ContainerInstanceStatusSpec) {
	suite.T().Logf("waiting for container %q: %s", containerName, stage)

	var (
		instanceID   resource.ID
		instanceSpec containers.ContainerInstanceStatusSpec
	)

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		newest, ok := suite.newestInstance(ctx, containerName, asrt)
		if !ok {
			return
		}

		if !asrt.NotNil(newest, "no instance status for container %q", containerName) {
			return
		}

		if !check(newest.TypedSpec(), asrt) {
			return
		}

		instanceID, instanceSpec = newest.Metadata().ID(), *newest.TypedSpec()
	}, containerStartTimeout, time.Second)

	return instanceID, instanceSpec
}

// newestInstance returns the highest-generation instance status for containerName, if any.
func (suite *ContainersSuite) newestInstance(
	ctx context.Context, containerName string, asrt *assert.Assertions,
) (*containers.ContainerInstanceStatus, bool) {
	statuses, err := safe.StateListAll[*containers.ContainerInstanceStatus](ctx, suite.Client.State())
	if !asrt.NoError(err) {
		return nil, false
	}

	var newest *containers.ContainerInstanceStatus

	for status := range statuses.All() {
		if status.TypedSpec().ContainerID != containerName {
			continue
		}

		if newest == nil || status.TypedSpec().Generation > newest.TypedSpec().Generation {
			newest = status
		}
	}

	return newest, true
}

// assertImageReady waits for containerName's image pull to resolve.
func (suite *ContainersSuite) assertImageReady(ctx context.Context, containerName string) {
	suite.T().Logf("waiting for the image of container %q to be pulled", containerName)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), containerName,
		func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerImagePhaseReady, status.TypedSpec().Phase,
				"image is in phase %s (error %q)", status.TypedSpec().Phase, status.TypedSpec().Error)
			asrt.NotEmpty(status.TypedSpec().Digest)
		})
}

// assertContainerStatus waits until the aggregated ContainerStatus of containerName satisfies check,
// and returns it.
//
// Unlike the instance statuses there is exactly one of these per container and it is named after the
// container, so this needs none of the newest-generation bookkeeping assertNewestInstance does.
func (suite *ContainersSuite) assertContainerStatus(
	ctx context.Context,
	containerName, stage string,
	check func(*containers.ContainerStatusSpec, *assert.Assertions),
) containers.ContainerStatusSpec {
	suite.T().Logf("waiting for the status of container %q: %s", containerName, stage)

	var status containers.ContainerStatusSpec

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), containerName,
		func(res *containers.ContainerStatus, asrt *assert.Assertions) {
			check(res.TypedSpec(), asrt)

			// Only the last, satisfied iteration is what the caller gets: rtestutils stops retrying
			// once check records no failures.
			status = *res.TypedSpec()
		})

	return status
}

// assertNoInstance verifies that no instance is created for containerName, and keeps checking for long
// enough that a merely slow creation would be caught.
func (suite *ContainersSuite) assertNoInstance(ctx context.Context, containerName string) {
	suite.assertNoInstanceFor(ctx, containerName, 30*time.Second)
}

// assertNoInstanceFor is assertNoInstance with an explicit observation window, for callers that need to
// span more restart/backoff cycles of a crash-looping dependency than the default window would catch.
func (suite *ContainersSuite) assertNoInstanceFor(ctx context.Context, containerName string, duration time.Duration) {
	suite.T().Logf("verifying container %q stays pending", containerName)

	suite.Require().Never(func() bool {
		specs, err := safe.StateListAll[*containers.ContainerInstanceSpec](ctx, suite.Client.State())
		if err != nil {
			return false
		}

		for spec := range specs.All() {
			if spec.TypedSpec().ContainerID == containerName {
				suite.T().Logf("unexpected instance %q for container %q", spec.Metadata().ID(), containerName)

				return true
			}
		}

		return false
	}, duration, time.Second)
}

// assertContainerLogged waits until the container's log contains any of wants, and returns the log.
//
// A container that runs a command to completion is restarted, so the log grows one run at a time and
// the wait covers the image pull, the first run and any restart in between.
//
// Where the container prints one of several mutually exclusive outcomes, pass all of them and assert
// which one it was afterwards: waiting on the expected one alone turns a wrong outcome into a wait
// that burns its whole deadline and reports only that the log did not contain what was wanted.
func (suite *ContainersSuite) assertContainerLogged(ctx context.Context, containerName string, wants ...string) string {
	suite.T().Logf("waiting for the log of container %q to contain one of %q", containerName, wants)

	var logs string

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		// A container that failed to start will never write anything, so without this the wait would
		// burn its whole deadline and report only that the log did not contain what was wanted.
		suite.requireInstanceNotFailed(ctx, containerName)

		read, err := suite.readContainerLog(ctx, containerName)
		if !asrt.NoError(err) {
			return
		}

		matched := false

		for _, want := range wants {
			if strings.Contains(read, want) {
				matched = true

				break
			}
		}

		if !asrt.True(matched, "none of %q in the log of %q so far (%s); log is:\n%s",
			wants, containerName, suite.describeNewestInstance(ctx, containerName), read) {
			return
		}

		logs = read
	}, containerStartTimeout, 2*time.Second)

	return logs
}

// requireInstanceNotFailed fails the test immediately if containerName's newest instance could not be
// started, rather than letting the caller wait for output that will never come.
func (suite *ContainersSuite) requireInstanceNotFailed(ctx context.Context, containerName string) {
	newest := suite.lookupNewestInstance(ctx, containerName)
	if newest == nil {
		return
	}

	suite.Require().NotEqual(containers.ContainerInstancePhaseFailed, newest.TypedSpec().Phase,
		"container %q failed to start: %s", containerName, newest.TypedSpec().Error)
}

// describeNewestInstance summarizes what the container's newest instance is doing, for failure
// messages: a log that never gets what it waits for is usually explained by the instance's state.
func (suite *ContainersSuite) describeNewestInstance(ctx context.Context, containerName string) string {
	newest := suite.lookupNewestInstance(ctx, containerName)
	if newest == nil {
		return "no instance yet"
	}

	spec := newest.TypedSpec()

	return fmt.Sprintf("instance %s: phase %s, exit code %d, error %q",
		newest.Metadata().ID(), spec.Phase, spec.ExitCode, spec.Error)
}

// lookupNewestInstance returns the highest-generation instance status for containerName, or nil.
//
// Unlike newestInstance this swallows the read error: the callers are building diagnostics, where a
// failure to read is not itself worth reporting.
func (suite *ContainersSuite) lookupNewestInstance(
	ctx context.Context, containerName string,
) *containers.ContainerInstanceStatus {
	statuses, err := safe.StateListAll[*containers.ContainerInstanceStatus](ctx, suite.Client.State())
	if err != nil {
		return nil
	}

	var newest *containers.ContainerInstanceStatus

	for status := range statuses.All() {
		if status.TypedSpec().ContainerID != containerName {
			continue
		}

		if newest == nil || status.TypedSpec().Generation > newest.TypedSpec().Generation {
			newest = status
		}
	}

	return newest
}

// readContainerLog reads the whole log of a declared container.
//
// The container runtime registers the log with the logging manager under a prefixed ID, which makes it
// a service log as far as the API is concerned, hence the system namespace here.
func (suite *ContainersSuite) readContainerLog(ctx context.Context, containerName string) (string, error) {
	stream, err := suite.Client.Logs(
		ctx,
		constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD,
		constants.TalosContainersLogPrefix+containerName,
		false,
		-1,
	)
	if err != nil {
		return "", err
	}

	reader, err := client.ReadStream(stream)
	if err != nil {
		return "", err
	}

	//nolint:errcheck
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// containerdImages lists the names of the images stored in a containerd namespace.
func (suite *ContainersSuite) containerdImages(ctx context.Context, namespace common.ContainerdNamespace) ([]string, error) {
	driver := common.ContainerDriver_CONTAINERD
	if namespace == common.ContainerdNamespace_NS_SYSTEM || namespace == common.ContainerdNamespace_NS_CRI {
		driver = common.ContainerDriver_CRI
	}

	rcv, err := suite.Client.ImageService().List(ctx, &machine.ImageServiceListRequest{
		Containerd: &common.ContainerdInstance{
			Driver:    driver,
			Namespace: namespace,
		},
	})
	if err != nil {
		return nil, err
	}

	var imageNames []string

	for {
		msg, err := rcv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return imageNames, nil
			}

			return nil, err
		}

		imageNames = append(imageNames, msg.GetName())
	}
}

// assertContainerdImages returns the image names in a containerd namespace, failing the test if
// they cannot be read.
func (suite *ContainersSuite) assertContainerdImages(ctx context.Context, namespace common.ContainerdNamespace) []string {
	imageNames, err := suite.containerdImages(ctx, namespace)
	suite.Require().NoError(err)

	return imageNames
}

// TestImageNotGarbageCollectedWhileReferenced covers the image GC instance which collects the
// taloscontainers namespace: a declared container's image lands there and not in the system
// namespace, and is not collected out from under a container that is still using it.
//
// The deletion half cannot be covered here: an unreferenced image only becomes eligible after
// cri.ImageGCGracePeriod, an hour, which is far longer than any node can be held for. That path is
// covered by TestImageGCTalosContainers in the controller's own tests, on synthetic time. This is
// the same kind of blind spot as the one assertNoContainerdContainer documents.
func (suite *ContainersSuite) TestImageNotGarbageCollectedWhileReferenced() {
	if testing.Short() {
		suite.T().Skip("skipping the test in short mode")
	}

	ctx, name, _ := suite.setupContainer("image-gc")

	suite.applyContainers(ctx, suite.shellContainer(name, "sleep 3600"))

	suite.assertContainerRunning(ctx, name, "before inspecting the image store")

	suite.Assert().Contains(suite.assertContainerdImages(ctx, common.ContainerdNamespace_NS_TALOSCONTAINERS), containerShellImage,
		"the container's image must be pulled into the taloscontainers namespace")

	suite.Assert().NotContains(suite.assertContainerdImages(ctx, common.ContainerdNamespace_NS_SYSTEM), containerShellImage,
		"the container's image must not leak into the system namespace, which is collected against a different expected set")

	suite.T().Logf("removing container config %q", name)

	suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, name)

	suite.assertNoContainerdContainer(ctx, name)

	// The image is unreferenced from here on, and must survive until the grace period elapses.
	suite.Require().Never(func() bool {
		imageNames, err := suite.containerdImages(ctx, common.ContainerdNamespace_NS_TALOSCONTAINERS)
		if err != nil {
			// A failed read is not the image having been collected.
			return false
		}

		return !slices.Contains(imageNames, containerShellImage)
	}, 30*time.Second, 5*time.Second, "image was collected before the grace period elapsed")
}

// assertNoContainerdContainer verifies that containerd is running no task for containerName.
//
// This cannot detect an orphaned containerd record: the inspector behind the Containers API skips
// containers with no live task, so a record whose task is gone is invisible here. What it does cover
// is a task that outlived its configuration.
func (suite *ContainersSuite) assertNoContainerdContainer(ctx context.Context, containerName string) {
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		asrt := assert.New(collect)

		resp, err := suite.Client.Containers(ctx, constants.TalosContainersContainerdNamespace, common.ContainerDriver_CONTAINERD)
		if !asrt.NoError(err) {
			return
		}

		for _, msg := range resp.GetMessages() {
			for _, container := range msg.GetContainers() {
				asrt.NotContains(container.GetId(), containerName,
					"containerd still holds container %q for %q", container.GetId(), containerName)
			}
		}
	}, time.Minute, time.Second)
}

// TestWorkloadMemoryLimitEnforced verifies that the kernel enforces the aggregate taloscontainers
// limit on a container which has no memory limit of its own: it is OOM-killed by the root's memcg,
// and only there.
//
// The victim fills a tmpfs it mounts: tmpfs pages are charged to the writer's cgroup. It is kept out
// of swap, as tmpfs pages are swappable: on a node with swap (the e2e workers carry a swap volume),
// the root limit would otherwise be met by swapping the victim out rather than by killing it.
func (suite *ContainersSuite) TestWorkloadMemoryLimitEnforced() {
	if testing.Short() {
		suite.T().Skip("skipping machine configuration changes in short mode")
	}

	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Skip("cgroups are nested in container mode, so the workload roots are not managed")
	}

	// The ordered cleanup below runs before setupContainer's idempotent config removal.
	ctx, name, node := suite.setupContainer("memcap")

	originalBytes := suite.RequireNoWorkloadResourceConfig(ctx, node)

	suite.Require().Equal("max", suite.ReadFile(ctx, filepath.Join(constants.CgroupMountPath, constants.CgroupTalosContainersRoot, "memory.max")))

	idle := suite.RequireWorkloadRootIdle(ctx, constants.CgroupTalosContainersRoot)

	// MiB-aligned, so the kernel applies it exactly; fits the smallest (2 GiB) e2e workers.
	const (
		limit       = uint64(256 << 20)
		victimBytes = 2 * limit
	)

	suite.Require().Less(idle, limit/4, "root %s retains %d bytes without a container", constants.CgroupTalosContainersRoot, idle)

	suite.T().Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cleanupCancel()

		cleanupCtx = client.WithNode(cleanupCtx, node)

		// Keep removal and restoration in one callback: a fatal removal/wait failure must not lift the cap.
		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, containercfg.ContainerConfigKind, name)
		suite.assertNoContainerdContainer(cleanupCtx, name)

		_, applyErr := suite.Client.ApplyConfiguration(cleanupCtx, &machine.ApplyConfigurationRequest{
			Data: originalBytes,
			Mode: machine.ApplyConfigurationRequest_NO_REBOOT,
		})
		suite.Require().NoError(applyErr, "restore original configuration on node %s", node)
		suite.AssertCgroupFile(cleanupCtx, constants.CgroupTalosContainersRoot, "memory.max", "max")
	})

	doc := runtimecfg.NewWorkloadResourceConfigV1Alpha1()
	doc.TalosContainersConfig = workloadMemoryRoot(limit)

	suite.PatchMachineConfig(ctx, doc)
	suite.AssertCgroupFile(ctx, constants.CgroupTalosContainersRoot, "memory.max", strconv.FormatUint(limit, 10))

	unrelated := []string{constants.CgroupVirtualMachines, constants.CgroupSystem, constants.CgroupPodRuntimeRoot}
	before := suite.MemoryEventsSnapshot(ctx, append([]string{constants.CgroupTalosContainersRoot}, unrelated...)...)
	started := time.Now()

	// No per-container memory limit, only a swap ceiling of zero. The tmpfs holds the whole write,
	// so ENOSPC cannot end the run before the memcg does; the completion marker must never be logged.
	victim := suite.shellContainer(name, "dd if=/dev/zero of=/scratch/fill bs=1M count="+strconv.FormatUint(victimBytes>>20, 10)+" && echo FILL_COMPLETED")
	victim.ResourcesConfig = &containercfg.ContainerResources{
		Limits: &containercfg.ContainerResourceLimits{Swap: "0"},
	}
	victim.MountsConfig = []containercfg.ContainerMount{
		{
			TmpfsMount: &containercfg.TmpfsMount{
				MountDestination: "/scratch",
				MountSize:        strconv.FormatUint(victimBytes>>20+1, 10) + "MiB",
			},
		},
	}

	suite.applyContainers(ctx, victim)

	_, first := suite.assertNewestInstanceID(ctx, name)

	// The victim carries no memory limit of its own, so only the root can stop it, and no swap, so
	// the root cannot swap it out instead. This is checked on the resolved spec rather than on the
	// victim's cgroup: the kill comes within milliseconds of each start and takes the cgroup with
	// it, so the files exist for a sliver of every restart interval. TestResourceLimits covers the
	// spec reaching the cgroup.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), name,
		func(spec *containers.ContainerSpec, asrt *assert.Assertions) {
			asrt.Zero(spec.TypedSpec().Resources.MemoryLimit, "victim carries its own memory limit")

			if asrt.NotNil(spec.TypedSpec().Resources.SwapLimit, "victim may swap") {
				asrt.Zero(*spec.TypedSpec().Resources.SwapLimit, "victim may swap")
			}
		})

	after := suite.AssertMemcgOOMKill(ctx, constants.CgroupTalosContainersRoot, before[constants.CgroupTalosContainersRoot], 2*time.Minute)

	suite.assertNewestInstance(ctx, name, "victim terminated by the root limit",
		func(status *containers.ContainerInstanceStatusSpec, asrt *assert.Assertions) bool {
			return asrt.NotEqual(containers.ContainerInstancePhaseRunning, status.Phase, "victim still running") &&
				(status.Generation > first.Generation || asrt.NotZero(status.ExitCode, "victim exited cleanly"))
		})

	logs, err := suite.readContainerLog(ctx, name)
	suite.Require().NoError(err)
	suite.Assert().NotContains(logs, "FILL_COMPLETED", "victim completed its write despite the root limit")

	// Stop the restart/kill loop before checking the blast radius.
	suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, name)
	suite.assertNoContainerdContainer(ctx, name)

	suite.AssertNoNewOOMKills(ctx, before, unrelated...)
	suite.AssertNoUserspaceOOMSince(ctx, started)

	current, err := suite.ReadCgroupUint(ctx, constants.CgroupTalosContainersRoot, "memory.current")
	suite.Require().NoError(err)
	suite.Assert().LessOrEqual(current, limit, "root %s charge %d still above the limit after the victim was removed", constants.CgroupTalosContainersRoot, current)

	suite.T().Logf("root %s OOM-killed the victim %d time(s)", constants.CgroupTalosContainersRoot,
		after.Hierarchical["oom_kill"]-before[constants.CgroupTalosContainersRoot].Hierarchical["oom_kill"])
}

func init() {
	allSuites = append(allSuites, new(ContainersSuite))
}
