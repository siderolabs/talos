// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type KubeletKubeconfigSuite struct {
	ctest.DefaultSuite

	kubeconfigPath string
}

func TestKubeletKubeconfigSuite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig-kubelet")

	s := &KubeletKubeconfigSuite{
		kubeconfigPath: path,
	}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 10 * time.Second,
		AfterSetup: func(ds *ctest.DefaultSuite) {
			// Reset filesystem state so tests don't leak into each other.
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				ds.Require().NoError(err)
			}

			ds.Require().NoError(ds.Runtime().RegisterController(&k8sctrl.KubeletKubeconfigController{
				Path: path,
			}))
		},
	}

	suite.Run(t, s)
}

func hashOf(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

func (suite *KubeletKubeconfigSuite) writeKubeconfig(data []byte) {
	suite.T().Helper()

	suite.Require().NoError(os.WriteFile(suite.kubeconfigPath, data, 0o600))
}

func (suite *KubeletKubeconfigSuite) TestMissingFileNoResource() {
	ctest.AssertNoResource[*k8s.KubeletKubeconfig](suite, k8s.KubeletKubeconfigID)
}

func (suite *KubeletKubeconfigSuite) TestCreateUpdateDelete() {
	initial := []byte("apiVersion: v1\nkind: Config\nclusters: []\n")

	suite.writeKubeconfig(initial)

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.Equal(hashOf(initial), res.TypedSpec().Hash)
		},
	)

	updated := slices.Concat(initial, []byte("users: []\n"))

	suite.writeKubeconfig(updated)

	ctest.AssertResource(
		suite,
		k8s.KubeletKubeconfigID,
		func(res *k8s.KubeletKubeconfig, assert *assert.Assertions) {
			assert.Equal(hashOf(updated), res.TypedSpec().Hash)
		},
	)

	suite.Require().NoError(os.Remove(suite.kubeconfigPath))

	ctest.AssertNoResource[*k8s.KubeletKubeconfig](suite, k8s.KubeletKubeconfigID)
}
