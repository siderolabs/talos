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
	attempts map[string]int
	closed   bool
}

type fakePullResult struct {
	digest string
	err    error
}

func newFakePuller() *fakePuller {
	return &fakePuller{
		results:  map[string]fakePullResult{},
		blocked:  map[string]chan struct{}{},
		attempts: map[string]int{},
	}
}

func (p *fakePuller) setResult(ref, digest string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.results[ref] = fakePullResult{digest: digest, err: err}
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

func (p *fakePuller) Pull(ctx context.Context, logger *zap.Logger, ref string) (string, error) {
	p.mu.Lock()
	p.attempts[ref]++
	blocked := p.blocked[ref]
	result := p.results[ref]
	p.mu.Unlock()

	if blocked != nil {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-blocked:
		}
	}

	return result.digest, result.err
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
		puller: puller,
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&containersctrl.ImageController{
					PullerProvider: func() (containersctrl.Puller, error) { return puller, nil },
				}))
			},
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

	suite.puller.setResult(ref, "", errors.New("signature verification denied"))
	suite.criUp()
	suite.createSpecWithImage("broken", ref)

	ctest.AssertResource(suite, "broken", func(status *containers.ContainerImageStatus, asrt *assert.Assertions) {
		asrt.Equal(containers.ContainerImagePhaseFailed, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Error, "signature verification denied")
		// A failed pull leaves no digest, so the instance gate stays shut.
		asrt.Empty(status.TypedSpec().Digest)
	})
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
