// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/pkg/cluster/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

type UpgradeCheckSuite struct {
	suite.Suite
}

func TestUpgradeChecksSuite(t *testing.T) {
	suite.Run(t, &UpgradeCheckSuite{})
}

func (suite *UpgradeCheckSuite) TestK8sComponentRemovedItemsNoError() {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer ctxCancel()

	resourceState := state.WrapCore(namespaced.NewState(inmem.Build))

	for _, id := range []string{k8s.APIServerID, k8s.ControllerManagerID, k8s.SchedulerID} {
		cfg := k8s.NewStaticPod(k8s.NamespaceName, id)
		cfg.TypedSpec().Pod = map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"command": []string{
							fmt.Sprintf("/usr/local/bin/%s", id),
						},
					},
				},
			},
		}

		suite.Require().NoError(resourceState.Create(ctx, cfg))
	}

	upgradeOptions := kubernetes.UpgradeOptions{
		FromVersion: "1.24.3",
		ToVersion:   "1.25.0",
	}

	checks, err := kubernetes.NewK8sUpgradeChecks(resourceState, upgradeOptions, []string{"10.5.0.2"})
	suite.Require().NoError(err)

	checkErrors := checks.Run(ctx)
	suite.Assert().NoError(checkErrors)
}

func (suite *UpgradeCheckSuite) TestK8sComponentRemovedItemsWithError() {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer ctxCancel()

	resourceState := state.WrapCore(namespaced.NewState(inmem.Build))

	checkData := map[string]struct {
		cliFlags []string
	}{
		k8s.APIServerID: {
			cliFlags: []string{
				"/usr/local/bin/kube-apiserver",
				"--bind-address=0.0.0.0",
				"--insecure-port=0",
				"--feature-gates=RotateKubeletServerCertificate=true,CSIVolumeFSGroupPolicy",
				"--enable-admission-plugins=NodeRestriction,PodSecurityPolicy",
				"--service-account-api-audiences=api",
			},
		},
		k8s.ControllerManagerID: {
			cliFlags: []string{
				"/usr/local/bin/kube-controller-manager",
				"--bind-address=0.0.0.0",
				"--insecure-port=0",
				"--feature-gates=RotateKubeletServerCertificate=true,CSIVolumeFSGroupPolicy",
				"--register-retry-count=100",
			},
		},
		k8s.SchedulerID: {
			cliFlags: []string{
				"/usr/local/bin/kube-scheduler",
				"--bind-address=0.0.0.0",
				"--insecure-port=0",
				"--feature-gates=RotateKubeletServerCertificate=true,CSIVolumeFSGroupPolicy",
			},
		},
	}

	expected := kubernetes.K8sComponentRemovedItemsError{
		AdmissionFlags: []kubernetes.K8sComponentItem{
			{
				Node:      "10.5.0.2",
				Component: "kube-apiserver",
				Value:     "PodSecurityPolicy",
			},
		},
		CLIFlags: []kubernetes.K8sComponentItem{
			{
				Node:      "10.5.0.2",
				Component: "kube-apiserver",
				Value:     "service-account-api-audiences",
			},
			{
				Node:      "10.5.0.2",
				Component: "kube-controller-manager",
				Value:     "register-retry-count",
			},
		},
		FeatureGates: []kubernetes.K8sComponentItem{
			{
				Node:      "10.5.0.2",
				Component: "kube-apiserver",
				Value:     "CSIVolumeFSGroupPolicy",
			},
			{
				Node:      "10.5.0.2",
				Component: "kube-controller-manager",
				Value:     "CSIVolumeFSGroupPolicy",
			},
			{
				Node:      "10.5.0.2",
				Component: "kube-scheduler",
				Value:     "CSIVolumeFSGroupPolicy",
			},
		},
	}

	for _, id := range []string{k8s.APIServerID, k8s.ControllerManagerID, k8s.SchedulerID} {
		cfg := k8s.NewStaticPod(k8s.NamespaceName, id)
		cfg.TypedSpec().Pod = map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{
						"command": checkData[id].cliFlags,
					},
				},
			},
		}

		suite.Require().NoError(resourceState.Create(ctx, cfg))
	}

	upgradeOptions := kubernetes.UpgradeOptions{
		FromVersion: "1.24.3",
		ToVersion:   "1.25.0",
	}

	checks, err := kubernetes.NewK8sUpgradeChecks(resourceState, upgradeOptions, []string{"10.5.0.2"})
	suite.Require().NoError(err)

	checkErrors := checks.Run(ctx)

	removedItemsError, ok := checkErrors.(kubernetes.K8sComponentRemovedItemsError)
	if !ok {
		suite.T().Error("expected K8sComponentRemovedItemsError")
	}

	suite.Assert().Equal(expected, removedItemsError)
}
