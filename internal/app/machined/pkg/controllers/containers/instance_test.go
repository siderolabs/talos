// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	timeres "github.com/siderolabs/talos/pkg/machinery/resources/time"
)

const (
	// testContainer is the container name every test in this suite uses.
	testContainer = "nginx"
	// clockContainer is an ungated container used to observe reconcile passes; see tick.
	//
	// The name has to sort after testContainer: a pass walks the specs in ID order, so the clock only
	// means "the pass is done with testContainer" if it comes last.
	clockContainer = "zz-clock"
	// testImageRef is the image the test container spec declares.
	testImageRef = "docker.io/library/nginx:latest"
	// otherImageRef is a second reference, for tests that edit the spec's image.
	otherImageRef = "docker.io/library/nginx:1.29"
	// testDigest is the digest the faked image controller resolves testImageRef to.
	testDigest = "sha256:abc"
	// movedDigest is what a re-pull resolves to, standing in for a tag that has moved.
	movedDigest = "sha256:def"
)

type InstanceSuite struct {
	ctest.DefaultSuite

	clockGeneration uint64
}

func TestInstanceSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &InstanceSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&containersctrl.InstanceController{}))
				suite.Require().NoError(suite.Runtime().RegisterController(&containersctrl.MountController{}))
			},
		},
	})
}

// createSpec creates a ContainerSpec for testContainer, applying any mutators.
func (suite *InstanceSuite) createSpec(mutate ...func(*containers.ContainerSpecSpec)) {
	suite.createNamedSpec(testContainer, mutate...)
}

// createNamedSpec creates a ContainerSpec with a fixed image, applying any mutators.
func (suite *InstanceSuite) createNamedSpec(name string, mutate ...func(*containers.ContainerSpecSpec)) {
	spec := containers.NewContainerSpec(containers.NamespaceName, name)
	spec.TypedSpec().Image = containers.ContainerImageSpec{Ref: testImageRef}

	for _, m := range mutate {
		m(spec.TypedSpec())
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), spec))
}

// markImageReady fakes the image controller's output for testContainer.
func (suite *InstanceSuite) markImageReady() {
	suite.markNamedImageReady(testContainer, testDigest)
}

// markNamedImageReady fakes the image controller's output, with digest as it resolved the tag.
func (suite *InstanceSuite) markNamedImageReady(name, digest string) {
	status := containers.NewContainerImageStatus(containers.NamespaceName, name)
	status.TypedSpec().Phase = containers.ContainerImagePhaseReady
	status.TypedSpec().Image = testImageRef
	status.TypedSpec().Digest = digest

	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))
}

// updateSpec applies a mutation to testContainer's ContainerSpec.
func (suite *InstanceSuite) updateSpec(mutate func(*containers.ContainerSpecSpec)) {
	ctest.UpdateWithConflicts(suite, containers.NewContainerSpec(containers.NamespaceName, testContainer),
		func(spec *containers.ContainerSpec) error {
			mutate(spec.TypedSpec())

			return nil
		})
}

// updateImageStatus applies a mutation to testContainer's ContainerImageStatus, standing in for a
// re-pull by the image controller.
func (suite *InstanceSuite) updateImageStatus(mutate func(*containers.ContainerImageStatusSpec)) {
	ctest.UpdateWithConflicts(suite, containers.NewContainerImageStatus(containers.NamespaceName, testContainer),
		func(status *containers.ContainerImageStatus) error {
			mutate(status.TypedSpec())

			return nil
		})
}

// assertInstance asserts that the given generation of testContainer's instance exists.
func (suite *InstanceSuite) assertInstance(generation uint64) {
	ctest.AssertResource(suite, containers.InstanceID(testContainer, generation),
		func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
			asrt.Equal(generation, instance.TypedSpec().Generation)
		})
}

// setInstanceStatus fakes the runtime controller's output for a generation of testContainer's
// instance, creating or updating the status as needed.
func (suite *InstanceSuite) setInstanceStatus(generation uint64, phase containers.ContainerInstancePhase, finishedAt time.Time) {
	id := containers.InstanceID(testContainer, generation)

	status := containers.NewContainerInstanceStatus(containers.NamespaceName, id)
	status.TypedSpec().ContainerID = testContainer
	status.TypedSpec().Generation = generation
	status.TypedSpec().Phase = phase
	status.TypedSpec().FinishedAt = finishedAt

	if err := suite.State().Create(suite.Ctx(), status); err == nil {
		return
	}

	ctest.UpdateWithConflicts(suite, status, func(res *containers.ContainerInstanceStatus) error {
		*res.TypedSpec() = *status.TypedSpec()

		return nil
	})
}

// assertNoInstance asserts that the given generation of testContainer's instance does not exist.
//
// On its own this proves little: a resource that has not been created yet looks exactly like one
// that never will be, so it also passes before the controller has run at all. Pair it with tick when
// the point is that the container was considered and rejected.
func (suite *InstanceSuite) assertNoInstance(generation uint64) {
	ctest.AssertNoResource[*containers.ContainerInstanceSpec](suite, containers.InstanceID(testContainer, generation))
}

// tick returns once the controller has provably completed a reconcile pass over the current state.
//
// It works by changing the spec of an ungated container and waiting for the next generation of its
// instance to appear. A pass reads every spec and status fresh, so a pass that produced that
// generation also evaluated everything else written before this call, which is what makes a
// subsequent assertNoInstance mean "considered and rejected" rather than "not yet reached".
//
// This relies on clockContainer sorting last: a pass walks the specs in ID order, so a clock instance
// appearing mid-pass would say nothing about specs the pass had yet to reach.
func (suite *InstanceSuite) tick() {
	if suite.clockGeneration == 0 {
		suite.createNamedSpec(clockContainer)
		suite.markNamedImageReady(clockContainer, testDigest)
	} else {
		ctest.UpdateWithConflicts(suite, containers.NewContainerSpec(containers.NamespaceName, clockContainer),
			func(spec *containers.ContainerSpec) error {
				// Unique per tick, so the spec genuinely differs and the generation advances.
				spec.TypedSpec().Args = []string{strconv.FormatUint(suite.clockGeneration, 10)}

				return nil
			})
	}

	ctest.AssertResource(suite, containers.InstanceID(clockContainer, suite.clockGeneration),
		func(*containers.ContainerInstanceSpec, *assert.Assertions) {})

	suite.clockGeneration++
}

// SetupTest resets the per-test clock on top of the default setup.
func (suite *InstanceSuite) SetupTest() {
	suite.DefaultSuite.SetupTest()

	suite.clockGeneration = 0

	// MountController treats a missing barrier as the node going down and does nothing, so nothing
	// would ever become ready without it.
	suite.Require().NoError(suite.State().Create(suite.Ctx(),
		containers.NewContainerLifecycle(containers.NamespaceName, containers.ContainerLifecycleID)))
}

func (suite *InstanceSuite) TestNoInstanceUntilImageReady() {
	suite.createSpec()

	// No image status yet: the container stays pending.
	suite.tick()
	suite.assertNoInstance(0)

	suite.markImageReady()

	// Once ready, the instance appears; see TestInstanceMirrorsMinimalSpec for its contents.
	suite.assertInstance(0)
}

// TestInstanceMirrorsFullSpec asserts the whole resulting instance at once, for a spec with every
// field populated.
//
// Comparing the entire struct is the point: a field the controller forgets to carry over is not a
// crash, it is a container quietly running without the setting the user asked for, and a
// field-by-field test only covers the fields someone thought to list.
func (suite *InstanceSuite) TestInstanceMirrorsFullSpec() {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Entrypoint = []string{"/docker-entrypoint.sh"}
		spec.Args = []string{"nginx", "-g", "daemon off;"}
		spec.WorkingDir = "/srv"
		spec.RunAs = containers.ContainerRunAsSpec{UID: new(int32(65534)), GID: new(int32(65533))}
		spec.Environment = []string{"NGINX_PORT=8080", "TZ=UTC"}
		spec.Mounts = []containers.ContainerMountSpec{
			{
				Kind:        containers.MountKindHostPath,
				Source:      "/dev",
				Destination: "/host/dev",
				Options:     []string{"ro"},
			},
			{
				Kind:        containers.MountKindTmpfs,
				Destination: "/tmp",
				Size:        64 << 20,
				Options:     []string{"nosuid"},
			},
		}
		spec.Security = containers.ContainerSecuritySpec{
			Privileged:       true,
			CapabilitiesAdd:  []string{"NET_ADMIN"},
			CapabilitiesDrop: []string{"ALL"},
		}
		spec.Network = containers.ContainerNetworkSpec{HostNetwork: true}
		spec.Resources = containers.ContainerResourcesSpec{MemoryLimit: 1 << 29, CPULimit: 1500}
		// DependsOn gates whether the instance exists; it is not carried onto it.
		spec.DependsOn = containers.ContainerDependsOnSpec{Time: true}
	})
	suite.markImageReady()

	timeStatus := timeres.NewStatus()
	timeStatus.TypedSpec().Synced = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), timeStatus))

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 0), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstanceSpecSpec{
			ContainerID: testContainer,
			Generation:  0,
			// The resolved digest, not the reference the spec names.
			Image:       testDigest,
			Entrypoint:  []string{"/docker-entrypoint.sh"},
			Args:        []string{"nginx", "-g", "daemon off;"},
			WorkingDir:  "/srv",
			RunAs:       containers.ContainerRunAsSpec{UID: new(int32(65534)), GID: new(int32(65533))},
			Environment: []string{"NGINX_PORT=8080", "TZ=UTC"},
			Mounts: []containers.ResolvedMountSpec{
				{
					Kind:        containers.MountKindHostPath,
					Source:      "/dev",
					Destination: "/host/dev",
					Options:     []string{"ro"},
				},
				{
					Kind:        containers.MountKindTmpfs,
					Destination: "/tmp",
					Size:        64 << 20,
					Options:     []string{"nosuid"},
				},
			},
			Security: containers.ContainerSecuritySpec{
				Privileged:       true,
				CapabilitiesAdd:  []string{"NET_ADMIN"},
				CapabilitiesDrop: []string{"ALL"},
			},
			Network:   containers.ContainerNetworkSpec{HostNetwork: true},
			Resources: containers.ContainerResourcesSpec{MemoryLimit: 1 << 29, CPULimit: 1500},
		}, *instance.TypedSpec())
	})
}

// TestInstanceMirrorsMinimalSpec is the same assertion for a spec that sets nothing beyond the
// image, so an invented default shows up as a difference.
func (suite *InstanceSuite) TestInstanceMirrorsMinimalSpec() {
	suite.createSpec()
	suite.markImageReady()

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 0), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerInstanceSpecSpec{
			ContainerID: testContainer,
			Image:       testDigest,
		}, *instance.TypedSpec())
	})
}

// TestGenerationCarriesOntoInstance pins the generation in the resource ID and the one in the spec
// together: the ID is what the runtime keys off, the field is what it reports.
func (suite *InstanceSuite) TestGenerationCarriesOntoInstance() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	for generation := uint64(1); generation <= 3; generation++ {
		suite.updateSpec(func(spec *containers.ContainerSpecSpec) {
			spec.Args = []string{strconv.FormatUint(generation, 10)}
		})

		ctest.AssertResource(suite, containers.InstanceID(testContainer, generation), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
			asrt.Equal(generation, instance.TypedSpec().Generation)
			asrt.Equal(testContainer, instance.TypedSpec().ContainerID)
			asrt.Equal([]string{strconv.FormatUint(generation, 10)}, instance.TypedSpec().Args)
		})
	}
}

// TestFailedImageStatusStaysPending pins the phase half of what "resolved" means.
//
// A digest alone must not start a container: a status that reports a failure has not produced
// anything runnable, whatever digest it happens to still carry from an earlier pull.
func (suite *InstanceSuite) TestFailedImageStatusStaysPending() {
	suite.createSpec()

	status := containers.NewContainerImageStatus(containers.NamespaceName, testContainer)
	status.TypedSpec().Phase = containers.ContainerImagePhaseFailed
	status.TypedSpec().Image = testImageRef
	status.TypedSpec().Digest = testDigest
	status.TypedSpec().Error = "signature verification denied"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))

	suite.tick()
	suite.assertNoInstance(0)

	// A later pull succeeds, and only now may the container start.
	suite.updateImageStatus(func(status *containers.ContainerImageStatusSpec) {
		status.Phase = containers.ContainerImagePhaseReady
		status.Error = ""
	})

	suite.assertInstance(0)
}

// TestReadyWithoutDigestStaysPending covers a ready image status that carries no digest.
//
// A ready phase is not on its own something to start from: the instance runs a digest, so until there
// is one the container waits rather than falling back to the mutable reference.
func (suite *InstanceSuite) TestReadyWithoutDigestStaysPending() {
	suite.createSpec()
	suite.markNamedImageReady(testContainer, "")

	suite.tick()
	suite.assertNoInstance(0)

	// The digest lands, and only now is there something to run.
	suite.updateImageStatus(func(status *containers.ContainerImageStatusSpec) {
		status.Digest = testDigest
	})

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 0), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal(testDigest, instance.TypedSpec().Image)
	})
}

func (suite *InstanceSuite) TestGatesOnNetworkAndTime() {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.DependsOn.Networks = []string{"addresses"}
		spec.DependsOn.Time = true
	})
	suite.markImageReady()

	suite.tick()
	suite.assertNoInstance(0)

	netStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	netStatus.TypedSpec().AddressReady = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), netStatus))

	// Network alone is not enough while time is also required.
	suite.tick()
	suite.assertNoInstance(0)

	timeStatus := timeres.NewStatus()
	timeStatus.TypedSpec().Synced = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), timeStatus))

	suite.assertInstance(0)
}

// assertNetworkConditionStarts pins one dependsOn.networks condition to the network.Status field it
// reads.
//
// Only that one field is ever set, so a condition reading the wrong field never becomes ready and
// the container never starts. Asserting the whole set at once would not catch that: once every
// field is true, a mis-wired condition is satisfied too.
func (suite *InstanceSuite) assertNetworkConditionStarts(condition string, satisfy func(*network.StatusSpec)) {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.DependsOn.Networks = []string{condition}
	})
	suite.markImageReady()

	suite.tick()
	suite.assertNoInstance(0)

	netStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	satisfy(netStatus.TypedSpec())
	suite.Require().NoError(suite.State().Create(suite.Ctx(), netStatus))

	suite.assertInstance(0)
}

func (suite *InstanceSuite) TestGatesOnAddresses() {
	suite.assertNetworkConditionStarts("addresses", func(status *network.StatusSpec) { status.AddressReady = true })
}

func (suite *InstanceSuite) TestGatesOnConnectivity() {
	suite.assertNetworkConditionStarts("connectivity", func(status *network.StatusSpec) { status.ConnectivityReady = true })
}

func (suite *InstanceSuite) TestGatesOnHostname() {
	suite.assertNetworkConditionStarts("hostname", func(status *network.StatusSpec) { status.HostnameReady = true })
}

func (suite *InstanceSuite) TestGatesOnEtcFiles() {
	suite.assertNetworkConditionStarts("etcfiles", func(status *network.StatusSpec) { status.EtcFilesReady = true })
}

// TestTimeSyncDisabledBlocksTimeGate covers a node with time sync disabled: the dependsOn.time gate
// was declared explicitly, so it stays unmet rather than silently letting the container start
// without a synced clock.
func (suite *InstanceSuite) TestTimeSyncDisabledBlocksTimeGate() {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.DependsOn.Time = true
	})
	suite.markImageReady()

	timeStatus := timeres.NewStatus()
	timeStatus.TypedSpec().SyncDisabled = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), timeStatus))

	suite.tick()
	suite.assertNoInstance(0)
}

// TestGatesOnPath covers the one dependency with no COSI equivalent, which the controller has to
// poll for.
func (suite *InstanceSuite) TestGatesOnPath() {
	path := filepath.Join(suite.T().TempDir(), "ready")

	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.DependsOn.Paths = []string{path}
	})
	suite.markImageReady()

	suite.tick()
	suite.assertNoInstance(0)

	suite.Require().NoError(os.WriteFile(path, nil, 0o600))

	// Creating the file produces no resource event, so only the poll can make this pass.
	suite.assertInstance(0)
}

func (suite *InstanceSuite) TestUserVolumeMountStaysPending() {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Mounts = []containers.ContainerMountSpec{
			{
				Kind:        containers.MountKindUserVolume,
				VolumeID:    "u-web-content",
				Destination: "/usr/share/nginx/html",
			},
		}
	})
	suite.markImageReady()

	// The volume's host path is only known once it is actually mounted, so the container waits.
	suite.tick()
	suite.assertNoInstance(0)
}

// TestStartsOnceUserVolumeIsMounted is the other half: the container starts once the volume is
// mounted, with the resolved host path on the instance it runs from.
func (suite *InstanceSuite) TestStartsOnceUserVolumeIsMounted() {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Mounts = []containers.ContainerMountSpec{
			{
				Kind:        containers.MountKindUserVolume,
				VolumeID:    "u-web-content",
				Destination: "/usr/share/nginx/html",
			},
		}
	})
	suite.markImageReady()

	requestID := "containers.MountController/" + testContainer + "/u-web-content"

	ctest.AssertResource(suite, requestID, func(*block.VolumeMountRequest, *assert.Assertions) {})

	mountStatus := block.NewVolumeMountStatus(block.NamespaceName, requestID)
	mountStatus.TypedSpec().VolumeID = "u-web-content"
	mountStatus.TypedSpec().Requester = "containers.MountController"
	mountStatus.TypedSpec().Target = "/var/mnt/web-content"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), mountStatus))

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 0), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		if !asrt.Len(instance.TypedSpec().Mounts, 1) {
			return
		}

		asrt.Equal("/var/mnt/web-content", instance.TypedSpec().Mounts[0].Source)
		asrt.Equal("/usr/share/nginx/html", instance.TypedSpec().Mounts[0].Destination)
	})
}

// TestResolvesTmpfsAndHostPathMounts covers the mount kinds that need no MountController.
//
// Asserting the resolved slice exactly is the point: each kind carries a different subset of the
// fields, and the instance is what a runtime would build an OCI spec from.
func (suite *InstanceSuite) TestResolvesTmpfsAndHostPathMounts() {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Mounts = []containers.ContainerMountSpec{
			{
				Kind:        containers.MountKindHostPath,
				Source:      "/dev",
				Destination: "/host/dev",
				Options:     []string{"ro"},
			},
			{
				Kind:        containers.MountKindTmpfs,
				Destination: "/tmp",
				Size:        64 << 20,
				Options:     []string{"nosuid"},
			},
		}
	})
	suite.markImageReady()

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 0), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal([]containers.ResolvedMountSpec{
			{
				Kind:        containers.MountKindHostPath,
				Source:      "/dev",
				Destination: "/host/dev",
				Options:     []string{"ro"},
			},
			{
				Kind:        containers.MountKindTmpfs,
				Destination: "/tmp",
				Size:        64 << 20,
				Options:     []string{"nosuid"},
			},
		}, instance.TypedSpec().Mounts)
	})
}

func (suite *InstanceSuite) TestSpecChangeReplacesInstance() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	suite.updateSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Args = []string{"--verbose"}
	})

	// The existing instance must be destroyed rather than mutated, and the next generation created
	// from the new spec.
	suite.assertNoInstance(0)

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 1), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal(uint64(1), instance.TypedSpec().Generation)
		asrt.Equal([]string{"--verbose"}, instance.TypedSpec().Args)
	})
}

// TestEveryComparedFieldReplacesInstance walks every field the spec-to-instance comparison covers.
//
// A field left out of that comparison is not a visible failure: the container keeps running on the
// old settings and the config change silently never takes effect. Each mutation must therefore
// advance the generation.
func (suite *InstanceSuite) TestEveryComparedFieldReplacesInstance() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	mutations := []func(*containers.ContainerSpecSpec){
		func(spec *containers.ContainerSpecSpec) { spec.Entrypoint = []string{"/entrypoint.sh"} },
		func(spec *containers.ContainerSpecSpec) { spec.Args = []string{"nginx", "-g", "daemon off;"} },
		func(spec *containers.ContainerSpecSpec) { spec.WorkingDir = "/srv" },
		func(spec *containers.ContainerSpecSpec) { spec.Environment = []string{"NGINX_PORT=8080"} },
		// Each RunAs half separately: nil is meaningful there, so a comparison that only looks at
		// one half would miss the other.
		func(spec *containers.ContainerSpecSpec) { spec.RunAs.UID = new(int32(65534)) },
		func(spec *containers.ContainerSpecSpec) { spec.RunAs.GID = new(int32(65534)) },
		func(spec *containers.ContainerSpecSpec) { spec.Security.Privileged = true },
		func(spec *containers.ContainerSpecSpec) { spec.Security.CapabilitiesAdd = []string{"NET_ADMIN"} },
		func(spec *containers.ContainerSpecSpec) { spec.Security.CapabilitiesDrop = []string{"ALL"} },
		func(spec *containers.ContainerSpecSpec) { spec.Network.HostNetwork = true },
		func(spec *containers.ContainerSpecSpec) { spec.Resources.MemoryLimit = 1 << 29 },
		func(spec *containers.ContainerSpecSpec) { spec.Resources.CPULimit = 1500 },
		func(spec *containers.ContainerSpecSpec) {
			spec.Mounts = []containers.ContainerMountSpec{{Kind: containers.MountKindTmpfs, Destination: "/tmp"}}
		},
	}

	for i, mutate := range mutations {
		suite.updateSpec(mutate)

		suite.assertInstance(uint64(i + 1))
	}
}

// TestMovedDigestReplacesInstance covers the drift that is invisible in the container spec: the
// reference is untouched, but the tag now points at different bytes.
//
// The instance runs a digest, so comparing references would report no change and the container would
// keep running the old image for the life of the node.
func (suite *InstanceSuite) TestMovedDigestReplacesInstance() {
	suite.createSpec()
	suite.markImageReady()

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 0), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal(testDigest, instance.TypedSpec().Image)
	})

	// Same reference, re-pulled to different bytes.
	suite.updateImageStatus(func(status *containers.ContainerImageStatusSpec) {
		status.Digest = movedDigest
	})

	suite.assertNoInstance(0)

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 1), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal(uint64(1), instance.TypedSpec().Generation)
		asrt.Equal(movedDigest, instance.TypedSpec().Image)
	})
}

// TestImageRefChangeWaitsForRepull covers an edited image reference, which reaches the instance only
// once something has resolved it.
//
// Replacing the instance the moment the reference changes would stop the container before there is
// anything to replace it with, so the old instance has to survive until the pull lands.
func (suite *InstanceSuite) TestImageRefChangeWaitsForRepull() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	// The reference changes, but the image status still describes the old pull.
	suite.updateSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Image = containers.ContainerImageSpec{Ref: otherImageRef}
	})

	suite.tick()
	suite.assertNoInstance(1)
	suite.assertInstance(0)

	// The pull lands, and only now is there a new image to run.
	suite.updateImageStatus(func(status *containers.ContainerImageStatusSpec) {
		status.Image = otherImageRef
		status.Digest = movedDigest
	})

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 1), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal(movedDigest, instance.TypedSpec().Image)
	})
}

// TestUnresolvedImageKeepsInstance covers the image status disappearing under a running instance.
//
// There is nothing to replace it with, so churning the container would only take away a working one.
func (suite *InstanceSuite) TestUnresolvedImageKeepsInstance() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(),
		containers.NewContainerImageStatus(containers.NamespaceName, testContainer).Metadata()))

	suite.tick()
	suite.assertNoInstance(1)
	suite.assertInstance(0)
}

// TestClearedDigestKeepsInstance covers a re-pull that reports no digest under a running instance.
//
// The instance is left alone rather than replaced: there is no digest to run instead, and stopping a
// working container over a status that says nothing would be the worse of the two outcomes.
func (suite *InstanceSuite) TestClearedDigestKeepsInstance() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	suite.updateImageStatus(func(status *containers.ContainerImageStatusSpec) {
		status.Digest = ""
	})

	suite.tick()
	suite.assertNoInstance(1)
	suite.assertInstance(0)
}

// TestStaleImageStatusIsIgnored covers an image status that still describes the previous reference.
//
// ImageController keeps one status per container, so an edited reference leaves the old reference's
// ready status in place until the re-pull lands. Those are the previous image's bytes, and starting
// them under the new configuration would run an image nobody asked for.
func (suite *InstanceSuite) TestStaleImageStatusIsIgnored() {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Image = containers.ContainerImageSpec{Ref: otherImageRef}
	})

	// Ready and carrying a digest, but for testImageRef rather than the reference the spec names.
	suite.markImageReady()

	suite.tick()
	suite.assertNoInstance(0)

	// The pull of the reference the spec does name lands.
	suite.updateImageStatus(func(status *containers.ContainerImageStatusSpec) {
		status.Image = otherImageRef
		status.Digest = movedDigest
	})

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 0), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal(movedDigest, instance.TypedSpec().Image)
	})
}

// TestGatedSpecChangeKeepsInstance covers a spec change made while the image gate is shut.
//
// Stopping the container the moment its spec changes leaves nothing to start in its place for as
// long as the gate stays shut, so the running instance has to survive until the replacement can
// actually be created.
//
// The generation the replacement lands on is what proves it: destroying first and creating in a
// later pass throws away the only record of the current number, and the replacement comes back as
// generation 0, reusing the ID of the instance that was just destroyed.
func (suite *InstanceSuite) TestGatedSpecChangeKeepsInstance() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	// Shut the gate first: with the digest still resolved, the edit below would be a legitimate
	// immediate replacement and this would test nothing.
	suite.updateImageStatus(func(status *containers.ContainerImageStatusSpec) {
		status.Digest = ""
	})

	suite.tick()
	suite.assertInstance(0)

	suite.updateSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Args = []string{"--verbose"}
	})

	suite.tick()
	suite.assertInstance(0)
	suite.assertNoInstance(1)

	// The re-pull lands, and only now is there something to replace it with.
	suite.updateImageStatus(func(status *containers.ContainerImageStatusSpec) {
		status.Digest = movedDigest
	})

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 1), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal([]string{"--verbose"}, instance.TypedSpec().Args)
		asrt.Equal(movedDigest, instance.TypedSpec().Image)
	})

	suite.assertNoInstance(0)
}

// TestGatedSpecChangeKeepsInstanceOnUnresolvableMount is the same case for a gate that never opens.
//
// Nothing resolves a userVolume yet, so replacing the instance over this edit would stop the
// container permanently rather than briefly.
func (suite *InstanceSuite) TestGatedSpecChangeKeepsInstanceOnUnresolvableMount() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	suite.updateSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Args = []string{"--verbose"}
		spec.Mounts = []containers.ContainerMountSpec{
			{
				Kind:        containers.MountKindUserVolume,
				VolumeID:    "u-web-content",
				Destination: "/usr/share/nginx/html",
			},
		}
	})

	suite.tick()
	suite.assertInstance(0)
	suite.assertNoInstance(1)
}

// TestTearingDownInstanceIsReplaced covers a spec that reverts while the instance it invalidated is
// still stopping.
//
// A stop already under way cannot be taken back: whatever holds the finalizer is bringing that
// container down. Comparing the tearing-down instance against the reverted spec would report it in
// sync, and it would sit there half-destroyed with nothing running in its place.
func (suite *InstanceSuite) TestTearingDownInstanceIsReplaced() {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Args = []string{"nginx"}
	})
	suite.markImageReady()

	suite.assertInstance(0)

	instanceMD := containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID(testContainer, 0)).Metadata()

	// Stands in for the runtime controller having taken charge of the instance: the teardown below
	// cannot complete until it releases.
	suite.AddFinalizer(instanceMD, "test")

	suite.updateSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Args = []string{"--verbose"}
	})

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 0), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, instance.Metadata().Phase())
	})

	// The spec goes back to exactly what the still-stopping instance was built from.
	suite.updateSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Args = []string{"nginx"}
	})

	suite.tick()

	suite.RemoveFinalizer(instanceMD, "test")

	suite.assertNoInstance(0)

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 1), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal([]string{"nginx"}, instance.TypedSpec().Args)
	})
}

// TestNoChangeKeepsInstance is the other half of the comparison: a spec write that changes nothing
// must not churn the container.
//
// The no-op write is followed by a real one, and the generation the real one lands on is what proves
// the no-op was ignored: an over-eager comparison would have spent generation 1 on the no-op, so the
// real change would land on 2 and this generation 1 would either be gone or carry the old args.
// Asserting only that generation 1 is absent right after the no-op would pass either way, since a
// resource that has not been created yet is indistinguishable from one that never will be.
func (suite *InstanceSuite) TestNoChangeKeepsInstance() {
	suite.createSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Args = []string{"nginx"}
	})
	suite.markImageReady()

	suite.assertInstance(0)

	// RunAs halves stay nil and mounts stay empty here, which is where an over-eager comparison
	// would report a change on every pass and restart the container forever.
	suite.updateSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Args = []string{"nginx"}
	})

	suite.updateSpec(func(spec *containers.ContainerSpecSpec) {
		spec.Args = []string{"nginx", "-g", "daemon off;"}
	})

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 1), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal([]string{"nginx", "-g", "daemon off;"}, instance.TypedSpec().Args)
	})

	suite.tick()
	suite.assertNoInstance(2)
}

func (suite *InstanceSuite) TestRemovesInstancesWhenSpecGoesAway() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(),
		containers.NewContainerSpec(containers.NamespaceName, testContainer).Metadata()))

	suite.assertNoInstance(0)
}

// TestRestartsAfterTermination covers the runtime controller's status feeding back into a restart,
// once RestartInterval has elapsed since the instance finished.
func (suite *InstanceSuite) TestRestartsAfterTermination() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	suite.setInstanceStatus(0, containers.ContainerInstancePhaseTerminated, time.Now().Add(-2*containersctrl.RestartInterval))

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 1), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal(uint64(1), instance.TypedSpec().Generation)
	})

	// The terminated instance is destroyed rather than kept around: nothing is retained.
	suite.assertNoInstance(0)
}

// TestDoesNotRestartBeforeInterval covers the other half: a broken controller that restarts
// immediately would also pass TestRestartsAfterTermination alone.
func (suite *InstanceSuite) TestDoesNotRestartBeforeInterval() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	suite.setInstanceStatus(0, containers.ContainerInstancePhaseTerminated, time.Now())

	suite.tick()
	suite.assertNoInstance(1)

	suite.setInstanceStatus(0, containers.ContainerInstancePhaseTerminated, time.Now().Add(-2*containersctrl.RestartInterval))

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 1), func(*containers.ContainerInstanceSpec, *assert.Assertions) {})
}

// TestFailedInstanceRestarts covers the failed phase restarting exactly like a terminated one.
func (suite *InstanceSuite) TestFailedInstanceRestarts() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	suite.setInstanceStatus(0, containers.ContainerInstancePhaseFailed, time.Now().Add(-2*containersctrl.RestartInterval))

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 1), func(*containers.ContainerInstanceSpec, *assert.Assertions) {})
}

// TestRestartWaitsForInstanceToStop covers a restart while something still holds the instance.
//
// Destroying the terminated instance before creating its replacement is what keeps none around, and
// that destruction is not instant: the runtime controller holds a finalizer until the task is stopped
// and its runtime state cleaned up. Creating the replacement without waiting for that would run two
// containers of the same name at once, and giving up on the wait would leave the container down for
// good.
func (suite *InstanceSuite) TestRestartWaitsForInstanceToStop() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	instanceMD := containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID(testContainer, 0)).Metadata()

	// Stands in for the runtime controller still stopping the task.
	suite.AddFinalizer(instanceMD, "test")

	suite.setInstanceStatus(0, containers.ContainerInstancePhaseTerminated, time.Now().Add(-2*containersctrl.RestartInterval))

	// The restart is due, so the instance is on its way out, but it cannot be replaced yet.
	ctest.AssertResource(suite, containers.InstanceID(testContainer, 0), func(instance *containers.ContainerInstanceSpec, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, instance.Metadata().Phase())
	})

	suite.tick()
	suite.assertNoInstance(1)

	suite.RemoveFinalizer(instanceMD, "test")

	ctest.AssertResource(suite, containers.InstanceID(testContainer, 1), func(*containers.ContainerInstanceSpec, *assert.Assertions) {})
	suite.assertNoInstance(0)
}

// TestKeepsNoOldInstances covers the invariant across a run of restarts: a container is only ever
// represented by one instance, and terminated ones are not accumulated.
//
// Checking it after several restarts rather than one is the point: a leak of one instance per restart
// is invisible in a single-restart test, and this is the shape that catches it.
func (suite *InstanceSuite) TestKeepsNoOldInstances() {
	suite.createSpec()
	suite.markImageReady()

	suite.assertInstance(0)

	const restarts = 5

	for generation := range uint64(restarts) {
		suite.setInstanceStatus(generation, containers.ContainerInstancePhaseTerminated, time.Now().Add(-2*containersctrl.RestartInterval))

		ctest.AssertResource(suite, containers.InstanceID(testContainer, generation+1), func(*containers.ContainerInstanceSpec, *assert.Assertions) {})

		// Every generation before the current one is gone, not merely the oldest.
		for older := range generation + 1 {
			suite.assertNoInstance(older)
		}
	}

	instances, err := safe.StateListAll[*containers.ContainerInstanceSpec](suite.Ctx(), suite.State())
	suite.Require().NoError(err)

	count := 0

	for instance := range instances.All() {
		if instance.TypedSpec().ContainerID == testContainer {
			count++
		}
	}

	suite.Require().Equal(1, count, "expected exactly one instance for %q", testContainer)
}
