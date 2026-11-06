// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type ExtraManifestSuite struct {
	ctest.DefaultSuite
}

func (suite *ExtraManifestSuite) TestReconcileInlineManifests() {
	configExtraManifests := k8s.NewExtraManifestsConfig()
	*configExtraManifests.TypedSpec() = k8s.ExtraManifestsConfigSpec{
		ExtraManifests: []k8s.ExtraManifest{
			{
				Name:     "namespaces",
				Priority: "99",
				InlineManifest: strings.TrimSpace(
					`
apiVersion: v1
kind: Namespace
metadata:
    name: ci
---
apiVersion: v1
kind: Namespace
metadata:
    name: build
`,
				),
			},
		},
	}

	statusNetwork := network.NewStatus(network.NamespaceName, network.StatusID)
	statusNetwork.TypedSpec().AddressReady = true
	statusNetwork.TypedSpec().ConnectivityReady = true

	suite.Create(configExtraManifests)
	suite.Create(statusNetwork)

	ctest.AssertResources(suite, []resource.ID{"99-namespaces"}, func(manifest *k8s.Manifest, asrt *assert.Assertions) {
		objects := k8sadapter.Manifest(manifest).Objects()

		if asrt.Len(objects, 2) {
			asrt.Equal("ci", objects[0].GetName())
			asrt.Equal("build", objects[1].GetName())
		}
	})
	rtestutils.AssertLength[*k8s.Manifest](suite.Ctx(), suite.T(), suite.State(), 1)
}

func (suite *ExtraManifestSuite) TestReconcileURLManifests() {
	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)

	_, err := zw.Write([]byte(`
apiVersion: v1
kind: Namespace
metadata:
    name: downloaded
`))
	suite.Require().NoError(err)
	suite.Require().NoError(zw.Close())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/manifest.yaml.gz" || r.Header.Get("X-Token") != "secret" {
			w.WriteHeader(http.StatusForbidden)

			return
		}

		w.Write(buf.Bytes()) //nolint:errcheck
	}))
	suite.T().Cleanup(srv.Close)

	configExtraManifests := k8s.NewExtraManifestsConfig()
	*configExtraManifests.TypedSpec() = k8s.ExtraManifestsConfigSpec{
		ExtraManifests: []k8s.ExtraManifest{
			{
				Name:     "downloaded",
				Priority: "99",
				URL:      srv.URL + "/manifest.yaml.gz",
				ExtraHeaders: map[string]string{
					"X-Token": "secret",
				},
			},
		},
	}

	statusNetwork := network.NewStatus(network.NamespaceName, network.StatusID)
	statusNetwork.TypedSpec().AddressReady = true
	statusNetwork.TypedSpec().ConnectivityReady = true

	suite.Create(configExtraManifests)
	suite.Create(statusNetwork)

	ctest.AssertResources(suite, []resource.ID{"99-downloaded"}, func(manifest *k8s.Manifest, asrt *assert.Assertions) {
		objects := k8sadapter.Manifest(manifest).Objects()

		if asrt.Len(objects, 1) {
			asrt.Equal("downloaded", objects[0].GetName())
		}
	})
}

func TestExtraManifestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ExtraManifestSuite{
		Timeout: 10 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&k8sctrl.ExtraManifestController{}))
		},
	})
}
