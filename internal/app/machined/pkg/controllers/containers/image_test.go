// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	"go.uber.org/zap"

	containersctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/containers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// fakePuller stands in for containerd. Each ref maps to a result, and a blocked ref hangs until
// released, which is how the pulling phase gets observed.
type fakePuller struct {
	mu       sync.Mutex
	results  map[string]fakePullResult
	blocked  map[string]chan struct{}
	failures map[string][]error
	attempts map[string]int
	closed   bool
}

type fakePullResult struct {
	digest string
	user   string
	err    error
}

func newFakePuller() *fakePuller {
	return &fakePuller{
		results:  map[string]fakePullResult{},
		blocked:  map[string]chan struct{}{},
		failures: map[string][]error{},
		attempts: map[string]int{},
	}
}

// failAttempts makes the next attempts on ref fail with errs, one each, before blocking or the result apply.
func (p *fakePuller) failAttempts(ref string, errs ...error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.failures[ref] = append(p.failures[ref], errs...)
}

func (p *fakePuller) setResult(ref, digest string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.results[ref] = fakePullResult{digest: digest, err: err}
}

// setUser sets the image USER reported for ref, on top of an earlier setResult.
func (p *fakePuller) setUser(ref, user string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := p.results[ref]
	result.user = user
	p.results[ref] = result
}

// block makes pulls of ref hang until release is called.
func (p *fakePuller) block(ref string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.blocked[ref] = make(chan struct{})
}

func (p *fakePuller) release(ref string) {
	p.mu.Lock()
	ch := p.blocked[ref]
	delete(p.blocked, ref)
	p.mu.Unlock()

	if ch != nil {
		close(ch)
	}
}

func (p *fakePuller) attemptCount(ref string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.attempts[ref]
}

func (p *fakePuller) Pull(ctx context.Context, logger *zap.Logger, ref string) (containersctrl.PullResult, error) {
	p.mu.Lock()
	p.attempts[ref]++
	blocked := p.blocked[ref]
	result := p.results[ref]

	var failure error

	if failures := p.failures[ref]; len(failures) > 0 {
		failure, p.failures[ref] = failures[0], failures[1:]
	}
	p.mu.Unlock()

	if failure != nil {
		return containersctrl.PullResult{}, failure
	}

	if blocked != nil {
		select {
		case <-ctx.Done():
			return containersctrl.PullResult{}, ctx.Err()
		case <-blocked:
		}
	}

	if result.err != nil {
		return containersctrl.PullResult{}, result.err
	}

	return containersctrl.PullResult{Digest: result.digest, User: result.user}, nil
}

func (p *fakePuller) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closed = true

	return nil
}

type ImageSuite struct {
	ctest.DefaultSuite

	puller *fakePuller
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestImageSuite(t *testing.T) {
	t.Parallel()

	puller := newFakePuller()

	suite.Run(t, &ImageSuite{
		puller:  puller,
		Timeout: 15 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			ctrl := &containersctrl.ImageController{
				PullerProvider: func() (containersctrl.Puller, error) { return puller, nil },
			}

			// Retry every few milliseconds, giving up after about a second.
			ctrl.SetPullRetryPolicy(2*time.Second, 10*time.Millisecond)

			suite.Require().NoError(suite.Runtime().RegisterController(ctrl))
		},
	})
}

func (suite *ImageSuite) criUp() {
	service := v1alpha1.NewService("cri")
	service.TypedSpec().Running = true
	service.TypedSpec().Healthy = true

	suite.Require().NoError(suite.State().Create(suite.Ctx(), service))
}

func (suite *ImageSuite) createSpecWithImage(name, image string) {
	spec := containers.NewContainerSpec(containers.NamespaceName, name)
	spec.TypedSpec().Image = containers.ContainerImageSpec{Ref: image}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), spec))
}

func (suite *ImageSuite) TestPendingUntilCRIIsUp() {
	suite.createSpecWithImage("nginx", "docker.io/library/nginx:latest")

	// Without the CRI service there is nowhere to pull to, and the status says so rather than
	// silently doing nothing.
	ctest.AssertResource(suite, "nginx", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhasePending, status.TypedSpec().Phase)
	})

	suite.Assert().Zero(suite.puller.attemptCount("docker.io/library/nginx:latest"))
}

func (suite *ImageSuite) TestPullsAndReportsDigest() {
	const ref = "docker.io/library/nginx:1.27"

	suite.puller.setResult(ref, "sha256:abc123", nil)
	suite.criUp()
	suite.createSpecWithImage("nginx", ref)

	ctest.AssertResource(suite, "nginx", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhaseReady, status.TypedSpec().Phase)
		asrt.Equal("sha256:abc123", status.TypedSpec().Digest)
		asrt.Empty(status.TypedSpec().Error)
	})
}

func (suite *ImageSuite) TestReportsPullingWhileBlocked() {
	const ref = "docker.io/library/slow:1.0"

	suite.puller.block(ref)
	suite.puller.setResult(ref, "sha256:slow", nil)
	suite.criUp()
	suite.createSpecWithImage("slow", ref)

	ctest.AssertResource(suite, "slow", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhasePulling, status.TypedSpec().Phase)
	})

	suite.puller.release(ref)

	ctest.AssertResource(suite, "slow", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhaseReady, status.TypedSpec().Phase)
		asrt.Equal("sha256:slow", status.TypedSpec().Digest)
	})
}

func (suite *ImageSuite) TestReportsFailure() {
	const ref = "docker.io/library/broken:1.0"

	// Not a terminal error, so this fails only once the pull times out.
	suite.puller.setResult(ref, "", errors.New("registry unavailable"))
	suite.criUp()
	suite.createSpecWithImage("broken", ref)

	ctest.AssertResource(suite, "broken", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhaseFailed, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Error, "registry unavailable")
		// A failed pull leaves no digest, so the instance gate stays shut.
		asrt.Empty(status.TypedSpec().Digest)
	})

	suite.Assert().Greater(suite.puller.attemptCount(ref), 1)
}

func (suite *ImageSuite) TestReportsAttemptErrorWhileRetrying() {
	const ref = "docker.io/library/flaky:1.0"

	// Two failed attempts, then a third which blocks until released.
	suite.puller.failAttempts(ref, errors.New("registry unavailable"), errors.New("connection reset"))
	suite.puller.block(ref)
	suite.puller.setResult(ref, "sha256:flaky", nil)
	suite.criUp()
	suite.createSpecWithImage("flaky", ref)

	ctest.AssertResource(suite, "flaky", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhasePulling, status.TypedSpec().Phase)
		asrt.Equal("connection reset", status.TypedSpec().Error)
	})

	suite.puller.release(ref)

	ctest.AssertResource(suite, "flaky", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhaseReady, status.TypedSpec().Phase)
		asrt.Equal("sha256:flaky", status.TypedSpec().Digest)
		asrt.Empty(status.TypedSpec().Error)
	})

	suite.Assert().Equal(3, suite.puller.attemptCount(ref))
}

func (suite *ImageSuite) TestDoesNotRepullUnchangedReference() {
	const ref = "docker.io/library/stable:1.0"

	suite.puller.setResult(ref, "sha256:stable", nil)
	suite.criUp()
	suite.createSpecWithImage("stable", ref)

	ctest.AssertResource(suite, "stable", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhaseReady, status.TypedSpec().Phase)
	})

	// Touch the spec without changing the reference: several reconciles follow, but the pull must
	// not be repeated. Otherwise every unrelated event would re-fetch the image.
	var updatedSpec *containers.ContainerSpec

	for range 3 {
		updatedSpec = ctest.UpdateWithConflicts(suite, containers.NewContainerSpec(containers.NamespaceName, "stable"),
			func(spec *containers.ContainerSpec) error {
				spec.TypedSpec().WorkingDir = "/srv"

				return nil
			})
	}

	// Only the first update actually changes content.
	suite.Assert().Equal("2", updatedSpec.Metadata().Version().String())

	ctest.AssertResource(suite, "stable", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhaseReady, status.TypedSpec().Phase)
	})

	suite.Assert().Equal(1, suite.puller.attemptCount(ref))
}

func (suite *ImageSuite) TestRepullsWhenReferenceChanges() {
	const (
		oldRef = "docker.io/library/moving:1.0"
		newRef = "docker.io/library/moving:2.0"
	)

	suite.puller.setResult(oldRef, "sha256:v1", nil)
	suite.puller.setResult(newRef, "sha256:v2", nil)
	suite.criUp()
	suite.createSpecWithImage("moving", oldRef)

	ctest.AssertResource(suite, "moving", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal("sha256:v1", status.TypedSpec().Digest)
	})

	ctest.UpdateWithConflicts(suite, containers.NewContainerSpec(containers.NamespaceName, "moving"),
		func(spec *containers.ContainerSpec) error {
			spec.TypedSpec().Image.Ref = newRef

			return nil
		})

	ctest.AssertResource(suite, "moving", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal("sha256:v2", status.TypedSpec().Digest)
	})
}

func (suite *ImageSuite) TestRemovesStatusWhenSpecGoesAway() {
	const ref = "docker.io/library/gone:1.0"

	suite.puller.setResult(ref, "sha256:gone", nil)
	suite.criUp()
	suite.createSpecWithImage("gone", ref)

	ctest.AssertResource(suite, "gone", func(*containers.ContainerImageStatus, *assert.Assertions) {})

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(),
		containers.NewContainerSpec(containers.NamespaceName, "gone").Metadata()))

	ctest.AssertNoResource[*containers.ContainerImageStatus](suite, "gone")
}

func (suite *ImageSuite) TestRejectsNamedUser() {
	const ref = "docker.io/library/named-user:1.0"

	suite.puller.setResult(ref, "sha256:named", nil)
	suite.puller.setUser(ref, "app")
	suite.criUp()
	suite.createSpecWithImage("named-user", ref)

	ctest.AssertResource(suite, "named-user", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhaseFailed, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Error, `image USER "app" is not numeric`)
		asrt.Contains(status.TypedSpec().Error, "runAs.uid and runAs.gid")
		// The pull succeeded, but the digest is withheld so the instance gate stays shut.
		asrt.Empty(status.TypedSpec().Digest)
	})
}

func (suite *ImageSuite) TestAllowsNamedUserCoveredByRunAs() {
	const ref = "docker.io/library/named-user-covered:1.0"

	suite.puller.setResult(ref, "sha256:covered", nil)
	suite.puller.setUser(ref, "app:1000")
	suite.criUp()

	spec := containers.NewContainerSpec(containers.NamespaceName, "named-user-covered")
	spec.TypedSpec().Image = containers.ContainerImageSpec{Ref: ref}
	spec.TypedSpec().RunAs = containers.ContainerRunAsSpec{UID: new(uint32(1000))}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), spec))

	ctest.AssertResource(suite, "named-user-covered", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhaseReady, status.TypedSpec().Phase)
		asrt.Equal("sha256:covered", status.TypedSpec().Digest)
		asrt.Empty(status.TypedSpec().Error)
	})
}

func (suite *ImageSuite) TestNamedUserRecoversOnceRunAsIsSet() {
	const ref = "docker.io/library/named-user-fixed:1.0"

	suite.puller.setResult(ref, "sha256:fixed", nil)
	suite.puller.setUser(ref, "app:app")
	suite.criUp()
	suite.createSpecWithImage("named-user-fixed", ref)

	ctest.AssertResource(suite, "named-user-fixed", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhaseFailed, status.TypedSpec().Phase)
	})

	ctest.UpdateWithConflicts(suite, containers.NewContainerSpec(containers.NamespaceName, "named-user-fixed"),
		func(spec *containers.ContainerSpec) error {
			spec.TypedSpec().RunAs = containers.ContainerRunAsSpec{UID: new(uint32(1000)), GID: new(uint32(1000))}

			return nil
		})

	ctest.AssertResource(suite, "named-user-fixed", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhaseReady, status.TypedSpec().Phase)
		asrt.Equal("sha256:fixed", status.TypedSpec().Digest)
		asrt.Empty(status.TypedSpec().Error)
	})

	// The fix is a config change, not an image change: the image already on the node is reused.
	suite.Assert().Equal(1, suite.puller.attemptCount(ref))
}
