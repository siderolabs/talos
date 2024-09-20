// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package k8s

import (
	"bytes"
	"context"
	_ "embed"
	"strings"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
)

// ApparmorSuite verifies that a pod with apparmor security context with `RuntimeDefault` works.
type ApparmorSuite struct {
	base.K8sSuite
}

//go:embed testdata/apparmor.yaml
var apparmorPodSpec []byte

// SuiteName returns the name of the suite.
func (suite *ApparmorSuite) SuiteName() string {
	return "k8s.ApparmorSuite"
}

// TestApparmor verifies that a pod with apparmor security context with `RuntimeDefault` works.
func (suite *ApparmorSuite) TestApparmor() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reaching out to the node IP is not reliable")
	}

	if suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skipping apparmor test since provisioner is not qemu")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	suite.T().Cleanup(cancel)

	reader, err := suite.Client.Read(ctx, "/sys/kernel/security/lsm")
	suite.Require().NoError(err)

	// read from reader into a buffer
	var lsm bytes.Buffer

	_, err = lsm.ReadFrom(reader)
	suite.Require().NoError(err)

	if !strings.Contains(lsm.String(), "apparmor") {
		suite.T().Skip("skipping apparmor test since apparmor is not enabled")
	}

	apparmorPodManifest := suite.ParseManifests(apparmorPodSpec)

	suite.T().Cleanup(func() {
		cleanUpCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		suite.DeleteManifests(cleanUpCtx, apparmorPodManifest)
	})

	suite.ApplyManifests(ctx, apparmorPodManifest)

	suite.Require().NoError(suite.WaitForPodToBeRunning(ctx, time.Minute, "default", "nginx-apparmor"))
}

func init() {
	allSuites = append(allSuites, new(ApparmorSuite))
}
