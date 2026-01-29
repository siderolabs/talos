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
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	crictrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/cri"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

func TestImageGC(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		mockImageService := &mockImageService{}

		// Create the controller inside synctest time function so it uses the controlled time
		controller := crictrl.NewImageGCController("cri", true)
		controller.ImageServiceProvider = func() (crictrl.ImageServiceProvider, error) {
			return mockImageService, nil
		}

		// Set up the test environment manually
		suite := &ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				// Register the controller
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
				CreatedAt: now.Add(-2 * crictrl.ImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a")),
				},
			}, // ok to be gc'd
			{
				Name: "sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a",
				// the image age is more than the grace period, but the controller won't remove due to the check on the last seen unreferenced timestamp
				CreatedAt: now.Add(-4 * crictrl.ImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a")),
				},
			}, // ok to be gc'd, same as above, another ref
			{
				Name:      "registry.io/org/image1:v1.3.7",
				CreatedAt: now.Add(-2 * crictrl.ImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
				},
			}, // current image
			{
				Name:      "registry.io/org/image1@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
				CreatedAt: now.Add(-2 * crictrl.ImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
				},
			}, // current image, canonical ref
			{
				Name:      "sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
				CreatedAt: now.Add(-2 * crictrl.ImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
				},
			}, // current image, digest ref
			{
				Name:      "registry.io/org/image1:v1.3.8",
				CreatedAt: now.Add(crictrl.ImageGCGracePeriod),
				Target: v1.Descriptor{
					Digest: must(digest.Parse("sha256:fd03335dd2e7163e5e36e933a0c735d7fec6f42b33ddafad0bc54f333e4a23c0")),
				},
			}, // not ok to clean up, too new
			{
				Name:      "registry.io/org/image2@sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8",
				CreatedAt: now.Add(-2 * crictrl.ImageGCGracePeriod),
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
		time.Sleep(crictrl.ImageGCGracePeriod + 5*time.Minute)
		synctest.Wait()

		// Advance time to trigger the cleanup cycle (15 minutes)
		time.Sleep(crictrl.ImageCleanupInterval)
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

		imageList, err := mockImageService.List(suite.Ctx())
		require.NoError(t, err)

		actualImages := xslices.Map(imageList, func(i images.Image) string { return i.Name })

		suite.Assert().Equal(expectedImages, actualImages, "images after first GC run do not match expected")
	})
}

type mockImageService struct {
	mu sync.Mutex

	images []images.Image
}

func (m *mockImageService) ImageService() images.Store {
	return m
}

func (m *mockImageService) Close() error {
	return nil
}

func (m *mockImageService) Get(ctx context.Context, name string) (images.Image, error) {
	panic("not implemented")
}

func (m *mockImageService) List(ctx context.Context, filters ...string) ([]images.Image, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

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

	m.images = xslices.FilterInPlace(m.images, func(i images.Image) bool { return i.Name != name })

	return nil
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
