// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	crictrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cri"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

func TestImageGC(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockImageService := &mockImageService{}

		// Create the controller inside synctest time function so it uses the controlled time
		controller := crictrl.NewImageGCController("cri", constants.SystemContainerdNamespace, crictrl.KubernetesRefsToRetain)
		controller.ImageServiceProvider = func() (crictrl.ImageServiceProvider, error) {
			return mockImageService, nil
		}

		suite := &ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(controller))
			},
			// We need a long timeout here because we advance time manually in the test and we want the controller
			// to have enough time to run its cleanup cycles.
			Timeout: 2 * time.Hour,
		}

		suite.SetT(t) // we need to explicitly set to the t from the synctest.Test

		suite.SetupTest()
		defer suite.TearDownTest()

		// Use synctest controlled time as the base time
		now := time.Now()

		storedImages := []images.Image{
			{
				Name:      "registry.io/org/image1:v1.3.5@sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a",
				CreatedAt: now.Add(-2 * crictrl.DefaultImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a")),
				},
			}, // ok to be gc'd
			{
				Name: "sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a",
				// the image age is more than the grace period, but the controller won't remove due to the check on the last seen unreferenced timestamp
				CreatedAt: now.Add(-4 * crictrl.DefaultImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a")),
				},
			}, // ok to be gc'd, same as above, another ref
			{
				Name:      "registry.io/org/image1:v1.3.7",
				CreatedAt: now.Add(-2 * crictrl.DefaultImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
				},
			}, // current image
			{
				Name:      "registry.io/org/image1@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
				CreatedAt: now.Add(-2 * crictrl.DefaultImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
				},
			}, // current image, canonical ref
			{
				Name:      "sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
				CreatedAt: now.Add(-2 * crictrl.DefaultImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
				},
			}, // current image, digest ref
			{
				Name:      "registry.io/org/image1:v1.3.8",
				CreatedAt: now.Add(crictrl.DefaultImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:fd03335dd2e7163e5e36e933a0c735d7fec6f42b33ddafad0bc54f333e4a23c0")),
				},
			}, // not ok to clean up, too new
			{
				Name:      "registry.io/org/image2@sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8",
				CreatedAt: now.Add(-2 * crictrl.DefaultImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8")),
				},
			}, // current image
		}

		mockImageService.images = storedImages

		criService := v1alpha1.NewService("cri")
		criService.TypedSpec().Healthy = true
		criService.TypedSpec().Running = true

		require.NoError(t, suite.State().Create(suite.Ctx(), criService))

		kubelet := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
		kubelet.TypedSpec().Image = "registry.io/org/image1:v1.3.7"
		require.NoError(t, suite.State().Create(suite.Ctx(), kubelet))

		etcd := etcd.NewSpec(etcd.NamespaceName, etcd.SpecID)
		etcd.TypedSpec().Image = "registry.io/org/image2:v3.5.9@sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8"
		require.NoError(t, suite.State().Create(suite.Ctx(), etcd))

		// // Wait for the controller to process all events and set up state
		// synctest.Wait()

		// Advance time past the grace period to make old images eligible for cleanup
		// Grace period is 60 minutes, so advance by 65 minutes to ensure cleanup
		time.Sleep(crictrl.DefaultImageGCGracePeriod + 5*time.Minute)
		synctest.Wait()

		// Advance time to trigger the cleanup cycle (15 minutes)
		time.Sleep(crictrl.DefaultImageCleanupInterval)
		synctest.Wait() // Wait for cleanup to complete

		// Images that should remain after cleanup:
		// - All referenced images (from kubelet and etcd specs)
		// - The "new" image that hasn't aged enough yet
		expectedImages := []string{
			"registry.io/org/image1:v1.3.7", // kubelet image
			"registry.io/org/image1@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135", // kubelet image canonical ref
			"sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",                        // kubelet image digest ref
			"registry.io/org/image1:v1.3.8", // new image, not old enough to clean
			"registry.io/org/image2@sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8", // etcd image
		}

		suite.Assert().Equal(expectedImages, mockImageService.imageNames(), "images after first GC run do not match expected")

		suite.Assert().Equal([]string{constants.SystemContainerdNamespace}, mockImageService.seenNamespaces(),
			"controller must only touch the containerd namespace it was constructed with")
	})
}

// TestImageGCTalosContainers covers the instance which collects the taloscontainers namespace, where
// the expected set comes from the containers declared in the machine configuration.
func TestImageGCTalosContainers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			cleanupInterval = time.Minute
			gracePeriod     = 5 * time.Minute
		)

		mockImageService := &mockImageService{}

		controller := crictrl.NewImageGCController("cri", constants.TalosContainersContainerdNamespace, crictrl.TalosContainersRefsToRetain)
		controller.ImageServiceProvider = func() (crictrl.ImageServiceProvider, error) {
			return mockImageService, nil
		}
		controller.CleanupInterval = cleanupInterval
		controller.GCGracePeriod = gracePeriod

		suite := &ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(controller))
			},
			Timeout: 2 * time.Hour,
		}

		suite.SetT(t)

		suite.SetupTest()
		defer suite.TearDownTest()

		now := time.Now()

		mockImageService.images = []images.Image{
			{
				// Referenced by a ContainerSpec below. Note the stored name uses the `docker.io`
				// host produced by the pull path, while the spec carries the `index.docker.io`
				// canonical form: the two must still match.
				Name:      "docker.io/library/alpine:3.23",
				CreatedAt: now.Add(-2 * gracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
				},
			},
			{
				// Referenced by a ContainerSpec by digest.
				Name:      "registry.k8s.io/pause@sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8",
				CreatedAt: now.Add(-2 * gracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8")),
				},
			},
			{
				// Left over from a container which is no longer declared: this is the leak the
				// controller exists to collect.
				Name:      "docker.io/library/alpine:3.22",
				CreatedAt: now.Add(-2 * gracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a")),
				},
			},
		}

		criService := v1alpha1.NewService("cri")
		criService.TypedSpec().Healthy = true
		criService.TypedSpec().Running = true

		require.NoError(t, suite.State().Create(suite.Ctx(), criService))

		shell := containers.NewContainerSpec(containers.NamespaceName, "shell")
		shell.TypedSpec().Image = containers.ContainerImageSpec{Ref: "index.docker.io/library/alpine:3.23"}
		require.NoError(t, suite.State().Create(suite.Ctx(), shell))

		pause := containers.NewContainerSpec(containers.NamespaceName, "pause")
		pause.TypedSpec().Image = containers.ContainerImageSpec{
			Ref: "registry.k8s.io/pause@sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8",
		}
		require.NoError(t, suite.State().Create(suite.Ctx(), pause))

		time.Sleep(gracePeriod + cleanupInterval)
		synctest.Wait()

		suite.Assert().Equal(
			[]string{
				"docker.io/library/alpine:3.23",
				"registry.k8s.io/pause@sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8",
			},
			mockImageService.imageNames(),
			"images referenced by a ContainerSpec must survive, unreferenced ones must not",
		)

		// Removing the container's configuration makes its image collectable, but only after it has
		// been seen unreferenced for the grace period.
		require.NoError(t, suite.State().Destroy(suite.Ctx(), shell.Metadata()))

		time.Sleep(cleanupInterval)
		synctest.Wait()

		suite.Assert().Contains(mockImageService.imageNames(), "docker.io/library/alpine:3.23",
			"image must not be collected before the grace period elapses")

		time.Sleep(gracePeriod + cleanupInterval)
		synctest.Wait()

		suite.Assert().Equal(
			[]string{"registry.k8s.io/pause@sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8"},
			mockImageService.imageNames(),
			"image of a removed container must be collected",
		)

		suite.Assert().Equal([]string{constants.TalosContainersContainerdNamespace}, mockImageService.seenNamespaces(),
			"controller must only touch the containerd namespace it was constructed with")
	})
}

// TestImageGCTalosContainersResolvedDigests covers the two expectation sources which name an image by
// digest rather than by reference: the pull result, and what a running instance was created against.
//
// Both matter when the declared reference no longer names the stored image: after the reference is
// edited, or when it is a moving tag which has since been re-resolved.
func TestImageGCTalosContainersResolvedDigests(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const (
			cleanupInterval = time.Minute
			gracePeriod     = 5 * time.Minute

			runningDigest = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
			pulledDigest  = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
			orphanDigest  = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
		)

		mockImageService := &mockImageService{}

		controller := crictrl.NewImageGCController("cri", constants.TalosContainersContainerdNamespace, crictrl.TalosContainersRefsToRetain)
		controller.ImageServiceProvider = func() (crictrl.ImageServiceProvider, error) {
			return mockImageService, nil
		}
		controller.CleanupInterval = cleanupInterval
		controller.GCGracePeriod = gracePeriod

		suite := &ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(controller))
			},
			Timeout: 2 * time.Hour,
		}

		suite.SetT(t)

		suite.SetupTest()
		defer suite.TearDownTest()

		now := time.Now()

		mockImageService.images = []images.Image{
			{
				// Named by no reference in the expected set: the container's declared reference has
				// been edited to a tag which has not been pulled yet, so only the still-running
				// instance names these bytes.
				Name:      "docker.io/library/nginx:1.29",
				CreatedAt: now.Add(-2 * gracePeriod),
				Target:    v1.Descriptor{Digest: must(digest.Parse(runningDigest))},
			},
			{
				// The container declares the moving tag `redis:8`, which does not match this record
				// by name and tag. Only the pull result names these bytes.
				Name:      "docker.io/library/redis:8.0",
				CreatedAt: now.Add(-2 * gracePeriod),
				Target:    v1.Descriptor{Digest: must(digest.Parse(pulledDigest))},
			},
			{
				Name:      "docker.io/library/mysql:9.0",
				CreatedAt: now.Add(-2 * gracePeriod),
				Target:    v1.Descriptor{Digest: must(digest.Parse(orphanDigest))},
			},
		}

		criService := v1alpha1.NewService("cri")
		criService.TypedSpec().Healthy = true
		criService.TypedSpec().Running = true

		require.NoError(t, suite.State().Create(suite.Ctx(), criService))

		// web: reference edited to a tag whose pull is still in flight, so its image status carries
		// no digest yet, and the running instance is the only thing naming the old image.
		web := containers.NewContainerSpec(containers.NamespaceName, "web")
		web.TypedSpec().Image = containers.ContainerImageSpec{Ref: "index.docker.io/library/nginx:1.30"}
		require.NoError(t, suite.State().Create(suite.Ctx(), web))

		webImage := containers.NewContainerImageStatus(containers.NamespaceName, "web")
		webImage.TypedSpec().Phase = containers.ContainerImagePhasePulling
		webImage.TypedSpec().Image = "index.docker.io/library/nginx:1.30"
		require.NoError(t, suite.State().Create(suite.Ctx(), webImage))

		webInstance := containers.NewContainerInstanceSpec(containers.NamespaceName, containers.InstanceID("web", 0))
		webInstance.TypedSpec().ContainerID = "web"
		webInstance.TypedSpec().Image = runningDigest
		require.NoError(t, suite.State().Create(suite.Ctx(), webInstance))

		// cache: declares a moving tag, which the stored record does not carry. The pull result is
		// what pins the bytes that tag resolved to.
		cache := containers.NewContainerSpec(containers.NamespaceName, "cache")
		cache.TypedSpec().Image = containers.ContainerImageSpec{Ref: "index.docker.io/library/redis:8"}
		require.NoError(t, suite.State().Create(suite.Ctx(), cache))

		cacheImage := containers.NewContainerImageStatus(containers.NamespaceName, "cache")
		cacheImage.TypedSpec().Phase = containers.ContainerImagePhaseReady
		cacheImage.TypedSpec().Image = "index.docker.io/library/redis:8"
		cacheImage.TypedSpec().Digest = pulledDigest
		require.NoError(t, suite.State().Create(suite.Ctx(), cacheImage))

		time.Sleep(gracePeriod + cleanupInterval)
		synctest.Wait()

		suite.Assert().Equal(
			[]string{
				"docker.io/library/nginx:1.29",
				"docker.io/library/redis:8.0",
			},
			mockImageService.imageNames(),
			"images named only by an instance spec or an image status must survive; one named by nothing must not",
		)
	})
}

type mockImageService struct {
	mu sync.Mutex

	images []images.Image

	// namespaces records every containerd namespace the controller operated in, so that a test can
	// assert it collected the namespace it was constructed with and no other.
	namespaces map[string]struct{}
}

func (m *mockImageService) ImageService() images.Store {
	return m
}

func (m *mockImageService) Close() error {
	return nil
}

// recordNamespace notes the containerd namespace carried by ctx. Called with m.mu held.
func (m *mockImageService) recordNamespace(ctx context.Context) {
	ns, ok := namespaces.Namespace(ctx)
	if !ok {
		ns = "<none>"
	}

	if m.namespaces == nil {
		m.namespaces = map[string]struct{}{}
	}

	m.namespaces[ns] = struct{}{}
}

// seenNamespaces returns the containerd namespaces the controller has operated in so far.
func (m *mockImageService) seenNamespaces() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	seen := maps.Keys(m.namespaces)
	slices.Sort(seen)

	return seen
}

func (m *mockImageService) Get(ctx context.Context, name string) (images.Image, error) {
	panic("not implemented")
}

func (m *mockImageService) List(ctx context.Context, filters ...string) ([]images.Image, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.recordNamespace(ctx)

	return slices.Clone(m.images), nil
}

func (m *mockImageService) Create(ctx context.Context, image images.Image) (images.Image, error) {
	panic("not implemented")
}

func (m *mockImageService) Update(ctx context.Context, image images.Image, fieldpaths ...string) (images.Image, error) {
	panic("not implemented")
}

func (m *mockImageService) Delete(ctx context.Context, name string, opts ...images.DeleteOpt) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.recordNamespace(ctx)

	m.images = xslices.FilterInPlace(m.images, func(i images.Image) bool { return i.Name != name })

	return nil
}

// imageNames returns the names of the images left in the store.
//
// It reads the store directly rather than through List, so that inspecting it from a test does not
// count as the controller having operated in a namespace.
func (m *mockImageService) imageNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return xslices.Map(m.images, func(i images.Image) string { return i.Name })
}

func TestBuildExpectedImageDigests(t *testing.T) {
	actualImages := []images.Image{
		{
			Name: "registry.io/org/image1:v1.3.5@sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a",
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a")),
			},
		},
		{
			Name: "sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a",
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a")),
			},
		},
		{
			Name: "registry.io/org/image1:v1.3.7",
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
			},
		},
		{
			Name: "registry.io/org/image1@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
			},
		},
		{
			Name: "sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
			},
		},
		{
			Name: "registry.io/org/image1:v1.3.8",
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:fd03335dd2e7163e5e36e933a0c735d7fec6f42b33ddafad0bc54f333e4a23c0")),
			},
		},
		{
			Name: "registry.io/org/image2@sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8",
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8")),
			},
		},
		{
			// As the pull path names it: `docker.io`, not the `index.docker.io` canonical form.
			Name: "docker.io/library/alpine:3.23",
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:ba0b7fbc85d67cf5b0d0c1e2b0b0eab0a5fa9b3b7c8b6e0a1a2b3c4d5e6f7081")),
			},
		},
	}

	logger := zaptest.NewLogger(t)

	for _, test := range []struct {
		name           string
		expectedImages []string

		expectedDigests []string
	}{
		{
			name: "empty",
		},
		{
			name: "by tag",
			expectedImages: []string{
				"registry.io/org/image1:v1.3.7",
			},
			expectedDigests: []string{
				"sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			},
		},
		{
			name: "by digest",
			expectedImages: []string{
				"registry.io/org/image1@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			},
			expectedDigests: []string{
				"sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			},
		},
		{
			name: "by digest and tag",
			expectedImages: []string{
				"registry.io/org/image1:v1.3.7@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			},
			expectedDigests: []string{
				"sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			},
		},
		{
			name: "not found",
			expectedImages: []string{
				"registry.io/org/image1:v1.3.9",
			},
		},
		{
			// ContainerImageStatus and ContainerInstanceSpec name an image by bare digest, which
			// resolves to itself: there is nothing to look up in the stored images.
			name: "by bare digest",
			expectedImages: []string{
				"sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			},
			expectedDigests: []string{
				"sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			},
		},
		{
			// ContainerConfig canonicalizes Docker Hub references to the `index.docker.io` host,
			// while the pull path stores the image record under `docker.io`. Both must resolve to
			// the same digest, or every container image would look unreferenced.
			name: "docker hub host normalization",
			expectedImages: []string{
				"index.docker.io/library/alpine:3.23",
			},
			expectedDigests: []string{
				"sha256:ba0b7fbc85d67cf5b0d0c1e2b0b0eab0a5fa9b3b7c8b6e0a1a2b3c4d5e6f7081",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			expectedDigests, err := crictrl.BuildExpectedDigests(logger, actualImages, test.expectedImages)
			require.NoError(t, err)

			expectedDigestKeys := maps.Keys(expectedDigests)

			slices.Sort(test.expectedDigests)
			slices.Sort(expectedDigestKeys)

			assert.Equal(t, test.expectedDigests, expectedDigestKeys)
		})
	}
}

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}
