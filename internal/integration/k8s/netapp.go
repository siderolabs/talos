// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package k8s

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"os"
	"text/template"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/internal/integration/base"
)

var (
	// tridentValues holds the Helm values for the trident-operator chart.
	tridentValues []byte

	//go:embed testdata/trident-backend-san.yaml
	tridentBackendSANTemplate string

	//go:embed testdata/trident-backend-nas.yaml
	tridentBackendNASTemplate string

	//go:embed testdata/trident-storageclass-san.yaml
	tridentStorageClassSAN []byte

	//go:embed testdata/trident-storageclass-nas.yaml
	tridentStorageClassNAS []byte
)

// tridentONTAPConfig holds the ONTAP connection details templated into the
// backend manifests. Values come from environment variables populated from CI
// secrets, so nothing sensitive is committed to the repository.
type tridentONTAPConfig struct {
	ManagementLIF string
	SVM           string
	Username      string
	Password      string
}

// NetAppSuite tests deploying NetApp Trident against ONTAP SAN and NAS backends.
type NetAppSuite struct {
	base.K8sSuite
}

// SuiteName returns the name of the suite.
func (suite *NetAppSuite) SuiteName() string {
	return "k8s.NetAppSuite"
}

// TestDeploy tests deploying Trident and running fio against ONTAP SAN and NAS storage classes.
func (suite *NetAppSuite) TestDeploy() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reaching out to the node IP is not reliable")
	}

	if suite.CSITestName != "netapp" {
		suite.T().Skip("skipping netapp test as it is not enabled")
	}

	ontap := tridentONTAPConfig{
		ManagementLIF: os.Getenv("TRIDENT_ONTAP_MANAGEMENT_LIF"),
		SVM:           os.Getenv("TRIDENT_ONTAP_SVM"),
		Username:      os.Getenv("TRIDENT_ONTAP_USERNAME"),
		Password:      os.Getenv("TRIDENT_ONTAP_PASSWORD"),
	}

	if ontap.ManagementLIF == "" || ontap.SVM == "" || ontap.Username == "" || ontap.Password == "" {
		suite.T().Fatalf("skipping netapp test: ONTAP backend is not configured (set TRIDENT_ONTAP_* environment variables)")
	}

	timeout, err := time.ParseDuration(suite.CSITestTimeout)
	if err != nil {
		suite.T().Fatalf("failed to parse timeout: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	suite.T().Cleanup(cancel)

	if err := suite.HelmInstall(
		ctx,
		"trident",
		"https://netapp.github.io/trident-helm-chart",
		TridentOperatorChartVersion,
		"trident-operator",
		"trident-operator",
		tridentValues,
	); err != nil {
		suite.T().Fatalf("failed to install Trident chart: %v", err)
	}

	// The trident-operator chart creates a cluster-scoped TridentOrchestrator named "trident".
	// Helm's --wait only covers the operator Deployment; the operator then installs Trident
	// (and its CRDs, including TridentBackendConfig) asynchronously, so wait for it to finish
	// before applying backends. An empty namespace is required for the cluster-scoped resource.
	suite.Require().NoError(suite.WaitForResource(
		ctx,
		"",
		"trident.netapp.io",
		"TridentOrchestrator",
		"v1",
		"trident",
		"{.status.status}",
		"Installed",
	))

	// The operator registers the remaining Trident CRDs (TridentBackendConfig, ...) only while
	// reconciling TridentOrchestrator to Installed, which happens after the RESTMapper cached
	// discovery during the wait above. Reset the mapper so those new CRDs are visible; otherwise
	// applying the backends fails with `no matches for kind "TridentBackendConfig"`.
	suite.Mapper.Reset()

	// On failure, dump the trident-node CSI plugin logs. Attach/mount errors (iSCSI login,
	// multipath, NFS mount) are reported by the node plugin, not the fio pod. Registered here so
	// it covers the backend applies and both fio runs.
	suite.T().Cleanup(func() {
		if !suite.T().Failed() {
			return
		}

		// Fresh context: the test ctx may already be past its deadline on a timed-out run.
		logCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()

		suite.logTridentNodeLogs(logCtx)
	})

	sanBackend := suite.ParseManifests(suite.renderTridentBackend(tridentBackendSANTemplate, ontap))
	nasBackend := suite.ParseManifests(suite.renderTridentBackend(tridentBackendNASTemplate, ontap))

	sanStorageClass := suite.ParseManifests(tridentStorageClassSAN)
	nasStorageClass := suite.ParseManifests(tridentStorageClassNAS)

	suite.ApplyManifests(ctx, sanBackend)
	suite.ApplyManifests(ctx, nasBackend)
	suite.ApplyManifests(ctx, sanStorageClass)
	suite.ApplyManifests(ctx, nasStorageClass)

	suite.T().Cleanup(func() {
		suite.DeleteManifests(ctx, sanStorageClass)
		suite.DeleteManifests(ctx, nasStorageClass)
		suite.DeleteManifests(ctx, sanBackend)
		suite.DeleteManifests(ctx, nasBackend)
	})

	// A TridentBackendConfig reaches phase "Bound" once Trident has validated the ONTAP connection.
	suite.Require().NoError(suite.WaitForResource(ctx, "trident", "trident.netapp.io", "TridentBackendConfig", "v1", "backend-ontap-san", "{.status.phase}", "Bound"))
	suite.Require().NoError(suite.WaitForResource(ctx, "trident", "trident.netapp.io", "TridentBackendConfig", "v1", "backend-ontap-nas", "{.status.phase}", "Bound"))

	suite.Run("fio-ontap-san", func() {
		suite.Require().NoError(suite.RunFIOTest(ctx, "trident-ontap-san", "10G"))
	})

	suite.Run("fio-ontap-nas", func() {
		suite.Require().NoError(suite.RunFIOTest(ctx, "trident-ontap-nas", "10G"))
	})
}

// renderTridentBackend templates the ONTAP connection details (from CI secrets) into a backend manifest.
func (suite *NetAppSuite) renderTridentBackend(tmplText string, ontap tridentONTAPConfig) []byte {
	tmpl, err := template.New("backend").Parse(tmplText)
	suite.Require().NoError(err)

	var rendered bytes.Buffer

	suite.Require().NoError(tmpl.Execute(&rendered, ontap))

	return rendered.Bytes()
}

// logTridentNodeLogs dumps the trident-node CSI plugin logs (the trident-main container on every
// node) into the test output. Attach/mount failures — iSCSI login, multipath, NFS mount — are
// reported by the node plugin rather than the fio pod, so this is where a failed run is diagnosed.
func (suite *NetAppSuite) logTridentNodeLogs(ctx context.Context) {
	pods, err := suite.Clientset.CoreV1().Pods("trident").List(ctx, metav1.ListOptions{
		LabelSelector: "app=node.csi.trident.netapp.io",
	})
	if err != nil {
		suite.T().Logf("failed to list trident-node pods: %s", err)

		return
	}

	tailLines := int64(200)

	for _, pod := range pods.Items {
		req := suite.Clientset.CoreV1().Pods("trident").GetLogs(pod.Name, &corev1.PodLogOptions{
			Container: "trident-main",
			TailLines: &tailLines,
		})

		readCloser, err := req.Stream(ctx)
		if err != nil {
			suite.T().Logf("failed to get logs for trident/%s: %s", pod.Name, err)

			continue
		}

		scanner := bufio.NewScanner(readCloser)
		for scanner.Scan() {
			suite.T().Logf("trident/%s: %s", pod.Name, scanner.Text())
		}

		readCloser.Close() //nolint:errcheck
	}
}

func init() {
	allSuites = append(allSuites, new(NetAppSuite))
}
