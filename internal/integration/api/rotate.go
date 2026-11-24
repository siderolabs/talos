// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/crypto/x509"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	etcdcfg "github.com/siderolabs/talos/pkg/machinery/config/types/etcd"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/pemutils"
	etcdres "github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	secretsres "github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/provision/access"
	"github.com/siderolabs/talos/pkg/rotate/pki/etcd"
	"github.com/siderolabs/talos/pkg/rotate/pki/kubernetes"
	"github.com/siderolabs/talos/pkg/rotate/pki/talos"
)

// RotateCASuite verifies rotation of Talos, Kubernetes and etcd CAs.
type RotateCASuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *RotateCASuite) SuiteName() string {
	return "api.RotateCASuite"
}

// SetupTest ...
func (suite *RotateCASuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 10*time.Minute)
}

// TearDownTest ...
func (suite *RotateCASuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

func (suite *RotateCASuite) newRotationContext() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)

	suite.T().Cleanup(cancel)

	return ctx
}

// TestTalos updates Talos CA in the cluster.
func (suite *RotateCASuite) TestTalos() {
	if suite.Cluster == nil {
		suite.T().Skip("cluster information is not available")
	}

	suite.T().Logf("capturing current Talos CA")

	nodeInternalIP := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	// save osRoot
	osRoot, err := safe.StateGetByID[*secretsres.OSRoot](client.WithNode(suite.ctx, nodeInternalIP), suite.Client.State(), secretsres.OSRootID)
	suite.Require().NoError(err)

	suite.T().Logf("rotating current CA -> new CA")

	newBundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	suite.Require().NoError(err)

	options := talos.Options{
		CurrentClient: suite.Client,
		ClusterInfo:   access.NewAdapter(suite.Cluster),

		ContextName: suite.Talosconfig.Context,
		Endpoints:   suite.Client.GetEndpoints(),

		NewTalosCA: newBundle.Certs.OS,

		EncoderOption: encoder.WithComments(encoder.CommentsAll),

		Printf: suite.T().Logf,
	}

	rotationCtx := suite.newRotationContext()

	newTalosconfig, err := talos.Rotate(rotationCtx, options)
	suite.Require().NoError(err)

	newClient, err := client.New(rotationCtx, client.WithConfig(newTalosconfig))
	suite.Require().NoError(err)

	if !testing.Short() {
		suite.restartAPIServices(rotationCtx, newClient)
	}

	suite.T().Logf("rotating back new CA -> old CA")

	options = talos.Options{
		CurrentClient: newClient,
		ClusterInfo:   access.NewAdapter(suite.Cluster),

		ContextName: suite.Talosconfig.Context,
		Endpoints:   suite.Client.GetEndpoints(),

		NewTalosCA: osRoot.TypedSpec().IssuingCA,

		EncoderOption: encoder.WithComments(encoder.CommentsAll),

		Printf: suite.T().Logf,
	}

	rotationCtx = suite.newRotationContext()

	_, err = talos.Rotate(rotationCtx, options)
	suite.Require().NoError(err)

	suite.AssertClusterHealthy(rotationCtx)

	suite.ClearConnectionRefused(rotationCtx, suite.DiscoverNodeInternalIPsByType(rotationCtx, machine.TypeWorker)...)
}

// TestKubernetes updates Kubernetes CA in the cluster.
func (suite *RotateCASuite) TestKubernetes() {
	if suite.Cluster == nil {
		suite.T().Skip("cluster information is not available")
	}

	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	suite.T().Logf("capturing current Kubernetes CA")

	nodeInternalIP := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	// save k8sRoot
	k8sRoot, err := safe.StateGetByID[*secretsres.KubernetesRoot](client.WithNode(suite.ctx, nodeInternalIP), suite.Client.State(), secretsres.KubernetesRootID)
	suite.Require().NoError(err)

	suite.T().Logf("rotating current CA -> new CA")

	newBundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	suite.Require().NoError(err)

	options := kubernetes.Options{
		TalosClient: suite.Client,
		ClusterInfo: access.NewAdapter(suite.Cluster),

		NewKubernetesCA: newBundle.Certs.K8s,

		EncoderOption: encoder.WithComments(encoder.CommentsAll),

		Printf: suite.T().Logf,
	}

	rotationCtx := suite.newRotationContext()

	suite.Require().NoError(kubernetes.Rotate(rotationCtx, options))

	suite.AssertClusterHealthy(rotationCtx)

	suite.T().Logf("rotating back new CA -> old CA")

	options = kubernetes.Options{
		TalosClient: suite.Client,
		ClusterInfo: access.NewAdapter(suite.Cluster),

		NewKubernetesCA: k8sRoot.TypedSpec().IssuingCA,

		EncoderOption: encoder.WithComments(encoder.CommentsAll),

		Printf: suite.T().Logf,
	}

	rotationCtx = suite.newRotationContext()

	suite.Require().NoError(kubernetes.Rotate(rotationCtx, options))

	suite.AssertClusterHealthy(rotationCtx)

	suite.rollKubeProxy(suite.newRotationContext())
}

// TestEtcd updates etcd CA in the cluster.
func (suite *RotateCASuite) TestEtcd() {
	if suite.Cluster == nil {
		suite.T().Skip("cluster information is not available")
	}

	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	nodeInternalIP := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)
	nodeCtx := client.WithNode(suite.ctx, nodeInternalIP)

	cfg, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	if !cfg.Has(etcdcfg.EtcdCAConfig) {
		suite.T().Skipf("etcd CA rotation requires %s machine configuration document", etcdcfg.EtcdCAConfig)
	}

	suite.T().Logf("capturing current etcd CA")

	// save etcdRoot
	etcdRoot, err := safe.StateGetByID[*secretsres.EtcdRoot](nodeCtx, suite.Client.State(), secretsres.EtcdRootID)
	suite.Require().NoError(err)

	suite.T().Logf("rotating current CA -> new CA")

	newBundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	suite.Require().NoError(err)

	suite.rotateEtcd(newBundle.Certs.Etcd)

	suite.T().Logf("rotating back new CA -> old CA")

	suite.rotateEtcd(etcdRoot.TypedSpec().EtcdCA)
}

// rotateEtcd rotates the etcd CA, and verifies that the cluster is using only the new CA afterwards.
func (suite *RotateCASuite) rotateEtcd(newCA *x509.PEMEncodedCertificateAndKey) {
	// etcd and kube-apiserver are restarted twice on each control plane node, one node at a time
	rotationCtx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	suite.Require().NoError(etcd.Rotate(rotationCtx, etcd.Options{
		TalosClient: suite.Client,
		ClusterInfo: access.NewAdapter(suite.Cluster),

		NewEtcdCA: newCA,

		EncoderOption: encoder.WithComments(encoder.CommentsAll),

		Printf: suite.T().Logf,
	}))

	expectedFingerprint := pemutils.Fingerprint([]*x509.PEMEncodedCertificate{{Crt: newCA.Crt}})

	for _, node := range suite.DiscoverNodeInternalIPsByType(rotationCtx, machine.TypeControlPlane) {
		nodeCtx := client.WithNode(rotationCtx, node)

		etcdRoot, err := safe.StateGetByID[*secretsres.EtcdRoot](nodeCtx, suite.Client.State(), secretsres.EtcdRootID)
		suite.Require().NoError(err)

		suite.Assert().Equal(newCA.Crt, etcdRoot.TypedSpec().EtcdCA.Crt, "node %s", node)
		suite.Assert().Equal(expectedFingerprint, pemutils.Fingerprint(etcdRoot.TypedSpec().AcceptedCAs), "node %s", node)

		serviceSpec, err := safe.StateGetByID[*etcdres.ServiceSpec](nodeCtx, suite.Client.State(), etcdres.ServiceSpecID)
		suite.Require().NoError(err)

		suite.Assert().Equal(expectedFingerprint, serviceSpec.TypedSpec().CAFingerprint, "node %s", node)

		pkiStatus, err := safe.StateGetByID[*etcdres.PKIStatus](nodeCtx, suite.Client.State(), etcdres.PKIID)
		suite.Require().NoError(err)

		suite.Assert().Equal(expectedFingerprint, pkiStatus.TypedSpec().IssuingCAFingerprint, "node %s", node)

		// the etcd API goes via the etcd client of the node
		_, err = suite.Client.EtcdMemberList(nodeCtx, &machineapi.EtcdMemberListRequest{})
		suite.Require().NoError(err)
	}

	suite.AssertClusterHealthy(rotationCtx)
}

// rollKubeProxy restarts kube-proxy once the Kubernetes CA rotation is over.
//
// kube-proxy talks to the API server with the in-cluster kubeconfig which trusts the service account
// CA bundle, and client-go only re-reads that CA from disk every 5 minutes, so a kube-proxy which
// happened to pick up the intermediate CA keeps failing to watch Services and EndpointSlices long
// after the rotation is done: it silently stops programming rules for Services created in the
// meantime.
func (suite *RotateCASuite) rollKubeProxy(ctx context.Context) {
	const (
		namespace     = "kube-system"
		daemonSetName = "kube-proxy"
		labelSelector = "k8s-app=kube-proxy"
	)

	daemonSets := suite.Clientset.AppsV1().DaemonSets(namespace)

	_, err := daemonSets.Get(ctx, daemonSetName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		suite.T().Logf("skipping kube-proxy restart: daemonset %s/%s is not found", namespace, daemonSetName)

		return
	}

	suite.Require().NoError(err)

	suite.T().Logf("restarting kube-proxy")

	patch := fmt.Appendf(nil, `{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":%q}}}}}`, time.Now().Format(time.RFC3339Nano))

	_, err = daemonSets.Patch(ctx, daemonSetName, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	suite.Require().NoError(err)

	suite.Require().NoError(suite.WaitForDaemonSetReady(ctx, 3*time.Minute, namespace, labelSelector))
}

func (suite *RotateCASuite) restartAPIServices(ctx context.Context, c *client.Client) {
	suite.T().Logf("restarting API services")

	var oldClient *client.Client

	oldClient, suite.Client = suite.Client, c

	defer func() {
		suite.Client = oldClient
	}()

	for _, node := range suite.DiscoverNodeInternalIPsByType(ctx, machine.TypeControlPlane) {
		suite.T().Logf("restarting API services on %s", node)

		err := c.Restart(client.WithNode(ctx, node), constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD, "trustd")
		suite.Require().NoError(err)

		suite.ClearConnectionRefused(ctx, node)

		err = c.Restart(client.WithNode(ctx, node), constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD, "apid")
		suite.Require().NoError(err)

		suite.ClearConnectionRefused(ctx, node)
	}

	for _, node := range suite.DiscoverNodeInternalIPsByType(ctx, machine.TypeWorker) {
		suite.T().Logf("restarting API services on %s", node)

		err := c.Restart(client.WithNode(ctx, node), constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD, "apid")
		suite.Require().NoError(err)

		suite.ClearConnectionRefused(ctx, node)
	}

	suite.AssertClusterHealthy(ctx)
}

func init() {
	allSuites = append(allSuites, new(RotateCASuite))
}
