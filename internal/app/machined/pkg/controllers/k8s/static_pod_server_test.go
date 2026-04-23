// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type StaticPodListSuite struct {
	ctest.DefaultSuite
}

func newTestPod(name string) *k8s.StaticPod {
	testPod := k8s.NewStaticPod(k8s.NamespaceName, name)

	testPod.TypedSpec().Pod = map[string]any{
		"metadata": name,
		"spec":     "testSpec",
	}

	return testPod
}

func (suite *StaticPodListSuite) TestCreatesStaticPodServerStatus() {
	suite.Create(newTestPod("testPod"))

	ctest.AssertResource(suite, k8s.StaticPodServerStatusResourceID, func(r *k8s.StaticPodServerStatus, asrt *assert.Assertions) {
		asrt.True(strings.HasPrefix(r.TypedSpec().URL, "http://127.0.0.1:"))
	})
}

func (suite *StaticPodListSuite) TestServesStaticPodList() {
	suite.Create(newTestPod("testPod1"))
	suite.Create(newTestPod("testPod2"))

	var podListURL string

	ctest.AssertResource(suite, k8s.StaticPodServerStatusResourceID, func(r *k8s.StaticPodServerStatus, asrt *assert.Assertions) {
		podListURL = r.TypedSpec().URL

		asrt.NotEmpty(podListURL)
	})

	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		resp, err := http.Get(podListURL) //nolint:noctx
		if err != nil {
			return retry.ExpectedError(err)
		}

		defer resp.Body.Close() //nolint:errcheck

		content, err := io.ReadAll(resp.Body)
		suite.Require().NoError(err)

		expected := "kind: PodList\nitems:\n    - metadata: testPod1\n      spec: testSpec\n    - metadata: testPod2\n      spec: testSpec\napiversion: v1\n"
		if string(content) != expected {
			return retry.ExpectedErrorf("pod list content mismatch: got %q", string(content))
		}

		return nil
	})
}

func TestStaticPodListSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &StaticPodListSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&k8sctrl.StaticPodServerController{}))
			},
		},
	})
}
