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
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	configcontainer "github.com/siderolabs/talos/pkg/machinery/config/config"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	containercfg "github.com/siderolabs/talos/pkg/machinery/config/types/container"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	// containerPauseImage never writes anything and never exits, which is what the lifecycle tests
	// want.
	containerPauseImage = images.DefaultSandboxImage

	// containerShellImage is used wherever a test needs the container to run a command and say
	// something about itself. The pause image has no shell and produces no output, so it cannot
	// carry any assertion about what the container actually got.
	containerShellImage = "docker.io/library/alpine:3.23"

	// containerSocatImage carries a unix-socket client, needed for machined socket access testing.
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
}

// SuiteName ...
func (suite *ContainersSuite) SuiteName() string {
	return "api.ContainersSuite"
}

// SetupTest ...
func (suite *ContainersSuite) SetupTest() {
	if suite.Airgapped {
		suite.T().Skip("skipping test in airgapped mode, the tests pull images")
	}

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

	rtestutils.AssertNoResource[*containers.ContainerInstanceSpec](ctx, suite.T(), suite.Client.COSI, oldInstanceID)

	newInstanceID, newInstance := suite.assertContainerRunning(ctx, name, "after the environment change")

	suite.Assert().NotEqual(oldInstanceID, newInstanceID, "instance was reused across the change")
	suite.Assert().Greater(newInstance.Generation, oldInstance.Generation,
		"expected a generation later than %d, got %d", oldInstance.Generation, newInstance.Generation)
	suite.Assert().NotEqual(oldInstance.PID, newInstance.PID,
		"container %q is still running as PID %d, so it was not restarted", name, oldInstance.PID)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, newInstanceID,
		func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
			asrt.Contains(instance.TypedSpec().Environment, envVar)
		})

	suite.T().Logf("removing container config %q", name)

	suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, name)

	rtestutils.AssertNoResource[*containers.ContainerSpec](ctx, suite.T(), suite.Client.COSI, name)

	suite.assertNoContainerdContainer(ctx, name)
}

// TestSurvivesReboot verifies that a declared container is stopped on the way down and started again
// as a new task on the way up.
func (suite *ContainersSuite) TestSurvivesReboot() {
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
			rtestutils.AssertNoResource[*containers.ContainerInstanceSpec](ctx, suite.T(), suite.Client.COSI, firstID)
		})
	}
}

// TestUnresolvableImage verifies that a container whose image cannot be pulled is withheld while the
// pull keeps retrying, and starts once the reference is corrected.
func (suite *ContainersSuite) TestUnresolvableImage() {
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

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerImagePhasePulling, status.TypedSpec().Phase,
				"image is in phase %s (error %q)", status.TypedSpec().Phase, status.TypedSpec().Error)
			asrt.Empty(status.TypedSpec().Digest, "an unresolvable reference produced a digest")
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
	const probeRootfs = "touch /rootfs-probe && echo WRITABLE || echo READONLY"

	suite.Run("restricted", func() {
		ctx, name, _ := suite.setupContainer("rootfs-restricted")

		suite.applyContainers(ctx, suite.shellContainer(name, probeRootfs))

		logs := suite.assertContainerLogged(ctx, name, "READONLY", "WRITABLE")

		suite.Assert().Contains(logs, "READONLY", "the restricted profile should mount the rootfs read-only")
	})

	suite.Run("privileged", func() {
		ctx, name, _ := suite.setupContainer("rootfs-privileged")

		doc := suite.shellContainer(name, probeRootfs)
		doc.SecurityConfig = &containercfg.ContainerSecurity{
			SecurityProfile: configcontainer.ContainerSecurityProfilePrivileged,
		}

		suite.applyContainers(ctx, doc)

		logs := suite.assertContainerLogged(ctx, name, "READONLY", "WRITABLE")

		suite.Assert().Contains(logs, "WRITABLE", "the privileged profile should not mount the rootfs read-only")
	})
}

// TestResourceLimits verifies that resource limits reach the instance spec and the container's cgroup.
func (suite *ContainersSuite) TestResourceLimits() {
	ctx, name, _ := suite.setupContainer("resources")

	const (
		memoryLimit      = "64Mi"
		memoryLimitBytes = 64 * 1024 * 1024
		cpuLimit         = "500m"
		// 500 millicores as a quota over the 100000us period the runner uses.
		cpuMaxWant = "50000 100000"
	)

	doc := suite.newContainer(name, containerPauseImage)
	doc.ResourcesConfig = &containercfg.ContainerResources{
		Limits: &containercfg.ContainerResourceLimits{
			CPU:    cpuLimit,
			Memory: memoryLimit,
		},
	}

	suite.applyContainers(ctx, doc)

	instanceID, _ := suite.assertContainerRunning(ctx, name, "with resource limits")

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, instanceID,
		func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
			asrt.EqualValues(memoryLimitBytes, instance.TypedSpec().Resources.MemoryLimit)
			asrt.NotZero(instance.TypedSpec().Resources.CPULimit)
		})

	if !suite.Capabilities().RunsTalosKernel {
		suite.T().Log("skipping the cgroup assertions: cgroups are nested in container mode, so the " +
			"container's cgroup is not at a path this test can predict")

		return
	}

	// The cgroup is keyed by container, not by instance, so the path is stable across restarts.
	cgroupPath := fmt.Sprintf("/sys/fs/cgroup/%s/%s", constants.CgroupTalosContainersRoot, name)

	suite.Assert().Equal(fmt.Sprintf("%d", memoryLimitBytes), suite.ReadFile(ctx, cgroupPath+"/memory.max"))
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

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, instanceID,
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

	rtestutils.AssertNoResource[*containers.ContainerSpec](ctx, suite.T(), suite.Client.COSI, first)
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
	volumeName := suite.setupUserVolume(ctx, node, "uservol")

	const destination = "/mnt/data"

	// The write is what proves the resolved host path is really mounted here: the resource assertions
	// below only show what the controllers agreed on, not what the container can reach. Parked with
	// sleep afterwards because a container that exits is restarted, which would race the assertion
	// that it is running.
	doc := suite.shellContainer(name,
		"echo volume-ok > "+destination+"/probe && cat "+destination+"/probe && sleep 3600")
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			UserVolumeMount: &containercfg.UserVolumeMount{
				VolumeName:       volumeName,
				MountDestination: destination,
			},
		},
	}

	suite.applyContainers(ctx, doc)

	// The host path is only knowable once the volume is actually mounted, which is what the mount
	// status reports.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
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

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, requestID,
		func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
			asrt.False(request.TypedSpec().ReadOnly, "volume was requested read-only despite the rw default")
		})

	// The hold: the finalizer on the mount status is what stops the volume being unmounted from under
	// the running container.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, requestID,
		func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
			asrt.Equal(filepath.Join(constants.UserVolumeMountPoint, volumeName), status.TypedSpec().Target)
			asrt.True(status.Metadata().Finalizers().Has(containerMountControllerName),
				"the mount is not held for the container, finalizers: %v", status.Metadata().Finalizers())
		})

	suite.T().Logf("removing container config %q", name)

	suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, name)

	// The hold has to be given back, or tearing the volume down would block on our finalizer.
	rtestutils.AssertNoResource[*containers.ContainerMountStatus](ctx, suite.T(), suite.Client.COSI, name)
	rtestutils.AssertNoResource[*block.VolumeMountRequest](ctx, suite.T(), suite.Client.COSI, requestID)

	suite.T().Logf("removing user volume %q", volumeName)

	suite.RemoveMachineConfigDocumentsByName(ctx, blockcfg.UserVolumeConfigKind, volumeName)

	// And the volume itself is then really unmounted, which is only possible because the hold was
	// released above.
	rtestutils.AssertNoResource[*block.MountStatus](ctx, suite.T(), suite.Client.COSI,
		constants.UserVolumePrefix+volumeName)
}

// TestUserVolumeMountWritableByDefault verifies that a user volume mounted without options is
// writable, both as requested of the block subsystem and as the container finds it.
func (suite *ContainersSuite) TestUserVolumeMountWritableByDefault() {
	ctx, name, node := suite.setupContainer("volume-rw")
	volumeName := suite.setupUserVolume(ctx, node, "volrw")

	const destination = "/mnt/data"

	// The destination is checked first: the image does not ship it, so a mount that never happened
	// would make the write fail and read as READONLY, passing this test for the wrong reason.
	doc := suite.shellContainer(name,
		"[ -d "+destination+" ] || { echo NOTMOUNTED; exit 0; }; "+
			"touch "+destination+"/probe 2>/dev/null && echo WRITABLE || echo READONLY")
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			UserVolumeMount: &containercfg.UserVolumeMount{
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
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, containerMountRequestID(name, volumeName),
		func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
			asrt.False(request.TypedSpec().ReadOnly, "volume %q was requested read-only", volumeName)
		})
}

// TestUserVolumeMountGate verifies that a container whose volume has not been declared is withheld,
// and starts once the volume shows up.
//
// The pause image is used because it is already on every node, so nothing here waits on a pull; what
// is being timed is the gate, not the registry.
func (suite *ContainersSuite) TestUserVolumeMountGate() {
	ctx, name, node := suite.setupContainer("volume-gate")

	// Named but not declared: the volume config is applied only further down.
	volumeName := fmt.Sprintf("itv-gate-%04x", rand.Int31())

	doc := suite.newContainer(name, containerPauseImage)
	doc.MountsConfig = []containercfg.ContainerMount{
		{
			UserVolumeMount: &containercfg.UserVolumeMount{
				VolumeName:       volumeName,
				MountDestination: "/mnt/data",
			},
		},
	}

	suite.applyContainers(ctx, doc)

	suite.T().Logf("verifying container %q waits for the undeclared volume %q", name, volumeName)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
			asrt.False(status.TypedSpec().Ready, "mounts are ready without the volume being declared")
			asrt.Contains(status.TypedSpec().Error, volumeName)
		})

	// The user-visible half of the gate: an unresolvable mount withholds the container itself.
	suite.assertNoInstance(ctx, name)

	suite.declareUserVolume(ctx, node, volumeName)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(status *containers.ContainerMountStatus, asrt *assert.Assertions) {
			asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)
		})

	suite.assertContainerRunning(ctx, name, "once the volume was declared")
}

// TestAllowMachinedAccess verifies that security.machinedAccess publishes the container's PID as a
// ServicePID resource and mounts the machined API socket into the container, and that neither
// happens when it is left unset.
func (suite *ContainersSuite) TestAllowMachinedAccess() {
	suite.Run("enabled", func() {
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

		rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, constants.ContainerServicePIDPrefix+name,
			func(servicePID *runtimeres.ServicePID, asrt *assert.Assertions) {
				asrt.EqualValues(instance.PID, servicePID.TypedSpec().PID)
			})

		suite.T().Logf("removing container config %q", name)

		suite.RemoveMachineConfigDocumentsByName(ctx, containercfg.ContainerConfigKind, name)

		rtestutils.AssertNoResource[*runtimeres.ServicePID](ctx, suite.T(), suite.Client.COSI, constants.ContainerServicePIDPrefix+name)
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

		rtestutils.AssertNoResource[*runtimeres.ServicePID](ctx, suite.T(), suite.Client.COSI, constants.ContainerServicePIDPrefix+name)
	})
}

// TestAllowMachinedSocketConnect verifies that a container granted security.machinedAccess can
// actually connect() to the machined socket, not merely see it.
func (suite *ContainersSuite) TestAllowMachinedSocketConnect() {
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
func (suite *ContainersSuite) setupContainer(purpose string) (context.Context, string, string) {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	name := fmt.Sprintf("test-%s-%d", purpose, time.Now().UnixNano())

	suite.T().Logf("using container config %q on node %s", name, node)

	suite.T().Cleanup(func() {
		// suite.ctx is likely canceled by the time this runs.
		cleanupCtx := client.WithNode(context.Background(), node)

		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, containercfg.ContainerConfigKind, name)
	})

	return ctx, name, node
}

// setupUserVolume declares a user volume unique to this test and returns its name.
//
// Directory-backed, so it needs no spare disk and these tests run on any cluster, unlike the
// disk-provisioned volumes in the volumes suite.
func (suite *ContainersSuite) setupUserVolume(ctx context.Context, node, purpose string) string {
	// Kept short deliberately: a user volume name becomes a partition label, so it is bounded, unlike
	// the container config names the other tests derive from a timestamp.
	name := fmt.Sprintf("itv-%s-%04x", purpose, rand.Int31())

	suite.declareUserVolume(ctx, node, name)

	return name
}

// declareUserVolume applies a directory-backed user volume by name, and registers its removal.
//
// Separate from setupUserVolume so that a test can name a volume first and declare it later, which is
// what covers a container waiting on a volume that does not exist yet.
func (suite *ContainersSuite) declareUserVolume(ctx context.Context, node, name string) {
	suite.T().Logf("declaring user volume %q on node %s", name, node)

	doc := blockcfg.NewUserVolumeConfigV1Alpha1()
	doc.MetaName = name
	doc.VolumeType = new(block.VolumeTypeDirectory)

	suite.T().Cleanup(func() {
		// suite.ctx is likely canceled by the time this runs.
		cleanupCtx := client.WithNode(context.Background(), node)

		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, blockcfg.UserVolumeConfigKind, name)
	})

	suite.PatchMachineConfig(ctx, doc)
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
	return suite.assertNewestInstance(ctx, containerName, stage,
		func(status *containers.ContainerInstanceStatusSpec, asrt *assert.Assertions) bool {
			return asrt.Equal(containers.ContainerInstancePhaseRunning, status.Phase,
				"phase is %s (error %q)", status.Phase, status.Error) &&
				asrt.NotZero(status.PID, "no PID reported")
		})
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
	statuses, err := safe.StateListAll[*containers.ContainerInstanceStatus](ctx, suite.Client.COSI)
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

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, containerName,
		func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
			asrt.Equal(containers.ContainerImagePhaseReady, status.TypedSpec().Phase,
				"image is in phase %s (error %q)", status.TypedSpec().Phase, status.TypedSpec().Error)
			asrt.NotEmpty(status.TypedSpec().Digest)
		})
}

// assertNoInstance verifies that no instance is created for containerName, and keeps checking for long
// enough that a merely slow creation would be caught.
func (suite *ContainersSuite) assertNoInstance(ctx context.Context, containerName string) {
	suite.T().Logf("verifying container %q stays pending", containerName)

	suite.Require().Never(func() bool {
		specs, err := safe.StateListAll[*containers.ContainerInstanceSpec](ctx, suite.Client.COSI)
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
	}, 30*time.Second, time.Second)
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
	statuses, err := safe.StateListAll[*containers.ContainerInstanceStatus](ctx, suite.Client.COSI)
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

	rcv, err := suite.Client.ImageClient.List(ctx, &machine.ImageServiceListRequest{
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

func init() {
	allSuites = append(allSuites, new(ContainersSuite))
}
