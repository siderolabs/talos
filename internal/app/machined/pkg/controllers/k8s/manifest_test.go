// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

type ManifestSuite struct {
	ctest.DefaultSuite
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

	PodSecurityPolicyEnabled: false,
}

func (suite *ManifestSuite) assertManifestIDs(ids []resource.ID) {
	ctest.AssertResources(suite, ids, func(*k8s.Manifest, *assert.Assertions) {})
	rtestutils.AssertLength[*k8s.Manifest](suite.Ctx(), suite.T(), suite.State(), len(ids))
}

func (suite *ManifestSuite) TestReconcileDefaults() {
	suite.Create(secrets.NewKubernetesRoot(secrets.KubernetesRootID))

	manifestConfig := k8s.NewBootstrapManifestsConfig()
	*manifestConfig.TypedSpec() = defaultManifestSpec
	suite.Create(manifestConfig)

	suite.assertManifestIDs([]resource.ID{
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
	})
}

func (suite *ManifestSuite) TestReconcileDisableKubeProxy() {
	suite.Create(secrets.NewKubernetesRoot(secrets.KubernetesRootID))

	manifestConfig := k8s.NewBootstrapManifestsConfig()
	spec := defaultManifestSpec
	spec.ProxyEnabled = false
	*manifestConfig.TypedSpec() = spec
	suite.Create(manifestConfig)

	suite.assertManifestIDs([]resource.ID{
		"00-kubelet-bootstrapping-token",
		"01-csr-approver-role-binding",
		"01-csr-node-bootstrap",
		"01-csr-renewal-role-binding",
		"05-flannel",
		"11-core-dns",
		"11-core-dns-svc",
		"11-kube-config-in-cluster",
		"11-talos-node-rbac-template",
	})
}

func (suite *ManifestSuite) TestReconcileKubeProxyExtraArgs() {
	suite.Create(secrets.NewKubernetesRoot(secrets.KubernetesRootID))

	manifestConfig := k8s.NewBootstrapManifestsConfig()
	spec := defaultManifestSpec
	spec.ProxyArgs = append(spec.ProxyArgs, "--bind-address=\"::\"")
	*manifestConfig.TypedSpec() = spec
	suite.Create(manifestConfig)

	suite.assertManifestIDs([]resource.ID{
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
	})

	ctest.AssertResource(suite, "10-kube-proxy", func(manifest *k8s.Manifest, asrt *assert.Assertions) {
		objects := k8sadapter.Manifest(manifest).Objects()
		asrt.Len(objects, 3)
		asrt.Equal("DaemonSet", objects[0].GetKind())

		ds := objects[0].Object
		containerSpec := ds["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)["containers"].([]any)[0]
		args := containerSpec.(map[string]any)["command"].([]any) //nolint:forcetypeassert

		asrt.Equal("--bind-address=\"::\"", args[len(args)-1])
	})
}

func (suite *ManifestSuite) TestReconcileIPv6() {
	suite.Create(secrets.NewKubernetesRoot(secrets.KubernetesRootID))

	manifestConfig := k8s.NewBootstrapManifestsConfig()
	spec := defaultManifestSpec
	spec.PodCIDRs = []string{constants.DefaultIPv6PodNet}
	spec.DNSServiceIP = ""
	spec.DNSServiceIPv6 = "fc00:db8:10::10"
	*manifestConfig.TypedSpec() = spec
	suite.Create(manifestConfig)

	suite.assertManifestIDs([]resource.ID{
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
	})

	ctest.AssertResource(suite, "11-core-dns-svc", func(manifest *k8s.Manifest, asrt *assert.Assertions) {
		objects := k8sadapter.Manifest(manifest).Objects()
		asrt.Len(objects, 1)

		service := objects[0]
		asrt.Equal("Service", service.GetKind())

		v, _, _ := unstructured.NestedString(service.Object, "spec", "clusterIP") //nolint:errcheck
		asrt.Equal(spec.DNSServiceIPv6, v)

		vv, _, _ := unstructured.NestedStringSlice(service.Object, "spec", "clusterIPs") //nolint:errcheck
		asrt.Equal([]string{spec.DNSServiceIPv6}, vv)

		vv, _, _ = unstructured.NestedStringSlice(service.Object, "spec", "ipFamilies") //nolint:errcheck
		asrt.Equal([]string{"IPv6"}, vv)

		v, _, _ = unstructured.NestedString(service.Object, "spec", "ipFamilyPolicy") //nolint:errcheck
		asrt.Equal("SingleStack", v)
	})

	ctest.AssertResource(suite, "05-flannel", func(manifest *k8s.Manifest, asrt *assert.Assertions) {
		objects := k8sadapter.Manifest(manifest).Objects()
		asrt.Len(objects, 5)

		configmap := objects[3]
		asrt.Equal("ConfigMap", configmap.GetKind())

		v, _, _ := unstructured.NestedString(configmap.Object, "data", "net-conf.json") //nolint:errcheck
		asrt.Contains(v, `"EnableIPv4": false`)
		asrt.Contains(v, `"EnableIPv6": true`)
		asrt.Contains(v, fmt.Sprintf(`"IPv6Network": "%s"`, constants.DefaultIPv6PodNet))
	})
}

func TestManifestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ManifestSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&k8sctrl.ManifestController{}))
			},
		},
	})
}
