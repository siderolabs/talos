// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s_test

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

type ManifestSuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

func (suite *ManifestSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, zaptest.NewLogger(suite.T()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&k8sctrl.ManifestController{}))

	suite.startRuntime()
}

func (suite *ManifestSuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

//nolint:dupl
func (suite *ManifestSuite) assertManifests(manifests []string) error {
	resources, err := suite.state.List(
		suite.ctx,
		resource.NewMetadata(k8s.ControlPlaneNamespaceName, k8s.ManifestType, "", resource.VersionUndefined),
	)
	if err != nil {
		return err
	}

	ids := xslices.Map(resources.Items, func(r resource.Resource) string { return r.Metadata().ID() })

	if !slices.Equal(manifests, ids) {
		return retry.ExpectedErrorf("expected %q, got %q", manifests, ids)
	}

	return nil
}

var defaultManifestSpec = k8s.BootstrapManifestsConfigSpec{
	Server:        "127.0.0.1",
	ClusterDomain: "cluster.",

	PodCIDRs: []string{constants.DefaultIPv4PodNet},

	ProxyEnabled: true,
	ProxyImage:   "foo/bar",
	ProxyArgs: []string{
		fmt.Sprintf("--cluster-cidr=%s", constants.DefaultIPv4PodNet),
		"--hostname-override=$(NODE_NAME)",
		"--kubeconfig=/etc/kubernetes/kubeconfig",
		"--proxy-mode=iptables",
		"--conntrack-max-per-core=0",
	},

	CoreDNSEnabled: true,
	CoreDNSImage:   "foo/bar",

	DNSServiceIP: "192.168.0.1",

	FlannelEnabled: true,
	FlannelImage:   "foo/bar",

	PodSecurityPolicyEnabled: true,
}

func (suite *ManifestSuite) TestReconcileDefaults() {
	rootSecrets := secrets.NewKubernetesRoot(secrets.KubernetesRootID)
	manifestConfig := k8s.NewBootstrapManifestsConfig()
	*manifestConfig.TypedSpec() = defaultManifestSpec

	suite.Require().NoError(suite.state.Create(suite.ctx, rootSecrets))
	suite.Require().NoError(suite.state.Create(suite.ctx, manifestConfig))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertManifests(
					[]string{
						"00-kubelet-bootstrapping-token",
						"01-csr-approver-role-binding",
						"01-csr-node-bootstrap",
						"01-csr-renewal-role-binding",
						"03-default-pod-security-policy",
						"05-flannel",
						"10-kube-proxy",
						"11-core-dns",
						"11-core-dns-svc",
						"11-kube-config-in-cluster",
						"11-talos-node-rbac-template",
					},
				)
			},
		),
	)
}

func (suite *ManifestSuite) TestReconcileDisableKubeProxy() {
	rootSecrets := secrets.NewKubernetesRoot(secrets.KubernetesRootID)
	manifestConfig := k8s.NewBootstrapManifestsConfig()
	spec := defaultManifestSpec
	spec.ProxyEnabled = false
	*manifestConfig.TypedSpec() = spec

	suite.Require().NoError(suite.state.Create(suite.ctx, rootSecrets))
	suite.Require().NoError(suite.state.Create(suite.ctx, manifestConfig))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertManifests(
					[]string{
						"00-kubelet-bootstrapping-token",
						"01-csr-approver-role-binding",
						"01-csr-node-bootstrap",
						"01-csr-renewal-role-binding",
						"03-default-pod-security-policy",
						"05-flannel",
						"11-core-dns",
						"11-core-dns-svc",
						"11-kube-config-in-cluster",
						"11-talos-node-rbac-template",
					},
				)
			},
		),
	)
}

func (suite *ManifestSuite) TestReconcileKubeProxyExtraArgs() {
	rootSecrets := secrets.NewKubernetesRoot(secrets.KubernetesRootID)
	manifestConfig := k8s.NewBootstrapManifestsConfig()
	spec := defaultManifestSpec
	spec.ProxyArgs = append(spec.ProxyArgs, "--bind-address=\"::\"")
	*manifestConfig.TypedSpec() = spec

	suite.Require().NoError(suite.state.Create(suite.ctx, rootSecrets))
	suite.Require().NoError(suite.state.Create(suite.ctx, manifestConfig))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertManifests(
					[]string{
						"00-kubelet-bootstrapping-token",
						"01-csr-approver-role-binding",
						"01-csr-node-bootstrap",
						"01-csr-renewal-role-binding",
						"03-default-pod-security-policy",
						"05-flannel",
						"10-kube-proxy",
						"11-core-dns",
						"11-core-dns-svc",
						"11-kube-config-in-cluster",
						"11-talos-node-rbac-template",
					},
				)
			},
		),
	)

	r, err := suite.state.Get(
		suite.ctx,
		resource.NewMetadata(
			k8s.ControlPlaneNamespaceName,
			k8s.ManifestType,
			"10-kube-proxy",
			resource.VersionUndefined,
		),
	)
	suite.Require().NoError(err)

	manifest := r.(*k8s.Manifest) //nolint:forcetypeassert
	suite.Assert().Len(k8sadapter.Manifest(manifest).Objects(), 3)

	suite.Assert().Equal("DaemonSet", k8sadapter.Manifest(manifest).Objects()[0].GetKind())

	ds := k8sadapter.Manifest(manifest).Objects()[0].Object
	containerSpec := ds["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)[0]
	args := containerSpec.(map[string]any)["command"].([]any) //nolint:forcetypeassert

	suite.Assert().Equal("--bind-address=\"::\"", args[len(args)-1])
}

func (suite *ManifestSuite) TestReconcileIPv6() {
	rootSecrets := secrets.NewKubernetesRoot(secrets.KubernetesRootID)
	manifestConfig := k8s.NewBootstrapManifestsConfig()
	spec := defaultManifestSpec
	spec.PodCIDRs = []string{constants.DefaultIPv6PodNet}
	spec.DNSServiceIP = ""
	spec.DNSServiceIPv6 = "fc00:db8:10::10"
	*manifestConfig.TypedSpec() = spec

	suite.Require().NoError(suite.state.Create(suite.ctx, rootSecrets))
	suite.Require().NoError(suite.state.Create(suite.ctx, manifestConfig))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertManifests(
					[]string{
						"00-kubelet-bootstrapping-token",
						"01-csr-approver-role-binding",
						"01-csr-node-bootstrap",
						"01-csr-renewal-role-binding",
						"03-default-pod-security-policy",
						"05-flannel",
						"10-kube-proxy",
						"11-core-dns",
						"11-core-dns-svc",
						"11-kube-config-in-cluster",
						"11-talos-node-rbac-template",
					},
				)
			},
		),
	)

	r, err := suite.state.Get(
		suite.ctx,
		resource.NewMetadata(
			k8s.ControlPlaneNamespaceName,
			k8s.ManifestType,
			"11-core-dns-svc",
			resource.VersionUndefined,
		),
	)
	suite.Require().NoError(err)

	manifest := r.(*k8s.Manifest) //nolint:forcetypeassert
	suite.Assert().Len(k8sadapter.Manifest(manifest).Objects(), 1)

	service := k8sadapter.Manifest(manifest).Objects()[0]
	suite.Assert().Equal("Service", service.GetKind())

	v, _, _ := unstructured.NestedString(service.Object, "spec", "clusterIP") //nolint:errcheck
	suite.Assert().Equal(spec.DNSServiceIPv6, v)

	vv, _, _ := unstructured.NestedStringSlice(service.Object, "spec", "clusterIPs") //nolint:errcheck
	suite.Assert().Equal([]string{spec.DNSServiceIPv6}, vv)

	vv, _, _ = unstructured.NestedStringSlice(service.Object, "spec", "ipFamilies") //nolint:errcheck
	suite.Assert().Equal([]string{"IPv6"}, vv)

	v, _, _ = unstructured.NestedString(service.Object, "spec", "ipFamilyPolicy") //nolint:errcheck
	suite.Assert().Equal("SingleStack", v)

	r, err = suite.state.Get(
		suite.ctx,
		resource.NewMetadata(
			k8s.ControlPlaneNamespaceName,
			k8s.ManifestType,
			"05-flannel",
			resource.VersionUndefined,
		),
	)
	suite.Require().NoError(err)

	manifest = r.(*k8s.Manifest) //nolint:forcetypeassert
	suite.Assert().Len(k8sadapter.Manifest(manifest).Objects(), 5)

	configmap := k8sadapter.Manifest(manifest).Objects()[3]
	suite.Assert().Equal("ConfigMap", configmap.GetKind())

	v, _, _ = unstructured.NestedString(configmap.Object, "data", "net-conf.json") //nolint:errcheck
	suite.Assert().Contains(v, `"EnableIPv4": false`)
	suite.Assert().Contains(v, `"EnableIPv6": true`)
	suite.Assert().Contains(v, fmt.Sprintf(`"IPv6Network": "%s"`, constants.DefaultIPv6PodNet))
}

func (suite *ManifestSuite) TestReconcileDisablePSP() {
	rootSecrets := secrets.NewKubernetesRoot(secrets.KubernetesRootID)
	manifestConfig := k8s.NewBootstrapManifestsConfig()
	spec := defaultManifestSpec
	spec.PodSecurityPolicyEnabled = false
	*manifestConfig.TypedSpec() = spec

	suite.Require().NoError(suite.state.Create(suite.ctx, rootSecrets))
	suite.Require().NoError(suite.state.Create(suite.ctx, manifestConfig))

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				return suite.assertManifests(
					[]string{
						"00-kubelet-bootstrapping-token",
						"01-csr-approver-role-binding",
						"01-csr-node-bootstrap",
						"01-csr-renewal-role-binding",
						"05-flannel",
						"10-kube-proxy",
						"11-core-dns",
						"11-core-dns-svc",
						"11-kube-config-in-cluster",
						"11-talos-node-rbac-template",
					},
				)
			},
		),
	)
}

func (suite *ManifestSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestManifestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(ManifestSuite))
}
