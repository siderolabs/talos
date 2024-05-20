// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"context"
	"reflect"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

func TestCRIImageGC(t *testing.T) {
	mockImageService := &mockImageService{}
	fakeClock := clock.NewMock()

	suite.Run(t, &CRIImageGCSuite{
		mockImageService: mockImageService,
		fakeClock:        fakeClock,
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrl.CRIImageGCController{
					ImageServiceProvider: func() (runtimectrl.ImageServiceProvider, error) {
						return mockImageService, nil
					},
					Clock: fakeClock,
				}))
			},
		},
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

type CRIImageGCSuite struct {
	ctest.DefaultSuite

	mockImageService *mockImageService
	fakeClock        *clock.Mock
}

func (suite *CRIImageGCSuite) TestReconcile() {
	storedImages := []images.Image{
		{
			Name:      "registry.io/org/image1:v1.3.5@sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a",
			CreatedAt: suite.fakeClock.Now().Add(-2 * runtimectrl.ImageGCGracePeriod),
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a")),
			},
		}, // ok to be gc'd
		{
			Name: "sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a",
			// the image age is more than the grace period, but the controller won't remove due to the check on the last seen unreferenced timestamp
			CreatedAt: suite.fakeClock.Now().Add(-4 * runtimectrl.ImageGCGracePeriod),
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:6b094bd0b063a1172eec7da249eccbb48cc48333800569363d67c747960cfa0a")),
			},
		}, // ok to be gc'd, same as above, another ref
		{
			Name:      "registry.io/org/image1:v1.3.7",
			CreatedAt: suite.fakeClock.Now().Add(-2 * runtimectrl.ImageGCGracePeriod),
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
			},
		}, // current image``
		{
			Name:      "registry.io/org/image1@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			CreatedAt: suite.fakeClock.Now().Add(-2 * runtimectrl.ImageGCGracePeriod),
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
			},
		}, // current image, canonical ref
		{
			Name:      "sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			CreatedAt: suite.fakeClock.Now().Add(-2 * runtimectrl.ImageGCGracePeriod),
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135")),
			},
		}, // current image, digest ref
		{
			Name:      "registry.io/org/image1:v1.3.8",
			CreatedAt: suite.fakeClock.Now().Add(runtimectrl.ImageGCGracePeriod),
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:fd03335dd2e7163e5e36e933a0c735d7fec6f42b33ddafad0bc54f333e4a23c0")),
			},
		}, // not ok to clean up, too new
		{
			Name:      "registry.io/org/image2@sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8",
			CreatedAt: suite.fakeClock.Now().Add(-2 * runtimectrl.ImageGCGracePeriod),
			Target: v1.Descriptor{
				Digest: must(digest.Parse("sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8")),
			},
		}, // current image
	}

	suite.mockImageService.images = storedImages

	criService := v1alpha1.NewService("cri")
	criService.TypedSpec().Healthy = true
	criService.TypedSpec().Running = true

	suite.Require().NoError(suite.State().Create(suite.Ctx(), criService))

	kubelet := k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID)
	kubelet.TypedSpec().Image = "registry.io/org/image1:v1.3.7"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), kubelet))

	etcd := etcd.NewSpec(etcd.NamespaceName, etcd.SpecID)
	etcd.TypedSpec().Image = "registry.io/org/image2:v3.5.9@sha256:2f794176e9bd8a28501fa185693dc1073013a048c51585022ebce4f84b469db8"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), etcd))

	expectedImages := xslices.Map(storedImages[2:7], func(i images.Image) string { return i.Name })

	suite.Assert().NoError(retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		suite.fakeClock.Add(runtimectrl.ImageCleanupInterval)

		imageList, _ := suite.mockImageService.List(suite.Ctx()) //nolint:errcheck
		actualImages := xslices.Map(imageList, func(i images.Image) string { return i.Name })

		if reflect.DeepEqual(expectedImages, actualImages) {
			return nil
		}

		return retry.ExpectedErrorf("images don't match: expected %v actual %v", expectedImages, actualImages)
	}))
}

func TestBuildExpectedImageNames(t *testing.T) {
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

		expectedImageNames []string
	}{
		{
			name: "empty",
		},
		{
			name: "by tag",
			expectedImages: []string{
				"registry.io/org/image1:v1.3.7",
			},
			expectedImageNames: []string{
				"registry.io/org/image1:v1.3.7",
				"registry.io/org/image1@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
				"sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			},
		},
		{
			name: "by digest",
			expectedImages: []string{
				"registry.io/org/image1@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			},
			expectedImageNames: []string{
				"registry.io/org/image1@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
				"sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			},
		},
		{
			name: "by digest and tag",
			expectedImages: []string{
				"registry.io/org/image1:v1.3.7@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
			},
			expectedImageNames: []string{
				"registry.io/org/image1:v1.3.7",
				"registry.io/org/image1@sha256:7051a34bcd2522e58a2291d1aa065667f225fd07e4445590b091e86c6799b135",
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
			expectedImages, err := runtimectrl.BuildExpectedImageNames(logger, actualImages, test.expectedImages)
			require.NoError(t, err)

			expectedImageNames := maps.Keys(expectedImages)

			sort.Strings(test.expectedImageNames)
			sort.Strings(expectedImageNames)

			assert.Equal(t, test.expectedImageNames, expectedImageNames)
		})
	}
}
