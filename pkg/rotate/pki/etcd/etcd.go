// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package etcd implements safe etcd PKI rotation for the cluster.
package etcd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-retry/retry"
	"go.yaml.in/yaml/v4"
	"google.golang.org/grpc/codes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/siderolabs/talos/pkg/cluster"
	taloskubernetes "github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/rotatepatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	etcdcfg "github.com/siderolabs/talos/pkg/machinery/config/types/etcd"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/pemutils"
	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	etcdres "github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	v1alpha1res "github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
	"github.com/siderolabs/talos/pkg/rotate/pki/internal/helpers"
)

const (
	// etcdRestartTimeout is the time for etcd to be restarted with the new PKI and to become healthy.
	//
	// The restart might be deferred, e.g. while the etcd member is still a learner.
	etcdRestartTimeout = 5 * time.Minute
	// apiServerRestartTimeout is the time for kube-apiserver to be restarted with the new PKI and to become ready.
	apiServerRestartTimeout = 5 * time.Minute
	// connectivityTimeout is the time for all etcd members to report healthy.
	connectivityTimeout = 3 * time.Minute
)

// Options is the input to the etcd PKI rotation process.
type Options struct {
	// DryRun is the flag to enable dry-run mode.
	//
	// In dry-run mode, the rotation process will not make any changes to the cluster.
	DryRun bool

	// TalosClient is a Talos API client
	TalosClient *client.Client
	// ClusterInfo provides information about cluster topology.
	ClusterInfo cluster.Info

	// KubernetesEndpoint overrides the default Kubernetes API endpoint.
	KubernetesEndpoint string

	// NewEtcdCA is the new CA for etcd.
	NewEtcdCA *x509.PEMEncodedCertificateAndKey

	// EncoderOption is the option for encoding machine configuration (while patching).
	EncoderOption encoder.Option

	// Printf is the function used to print messages.
	Printf func(format string, args ...any)
}

type rotator struct {
	opts Options

	// oldCAs is the list of the issuing CAs found in the cluster before the rotation.
	//
	// Normally there is a single CA shared by all control plane nodes, but there might be more
	// if the previous rotation was interrupted while swapping the CAs.
	oldCAs [][]byte

	kubernetes *cluster.KubernetesClient
}

// Rotate rotates the etcd PKI.
//
// etcd CA is only present on the control plane nodes, so only the control plane nodes are affected.
//
// The process overview:
//   - fetch current information
//   - verify etcd health with the existing PKI
//   - add new etcd CA as accepted
//   - verify etcd health
//   - make new CA issuing, old CA is still accepted
//   - verify etcd health with the new PKI
//   - remove old CA
//   - verify etcd health with the new PKI.
//
// Each step is applied to the control plane nodes one by one: when the set of the trusted CAs changes (i.e. when the CA
// is added or removed), etcd and kube-apiserver are restarted on the node to pick up the new PKI, and the process moves on
// to the next node only once they are back and healthy.
// Replacing the issuing CA doesn't restart anything: the re-issued certificates are picked up on the next connection.
//
// A step is started only when the previous one is complete on all nodes, so at any moment every etcd member
// (and every etcd client) trusts the certificates presented by any other member: the process can be aborted
// at any point without breaking the cluster, and it can be started over to finish the rotation.
func Rotate(ctx context.Context, opts Options) error {
	r := rotator{
		opts: opts,
	}

	defer func() {
		if r.kubernetes != nil {
			r.kubernetes.K8sClose() //nolint:errcheck
		}
	}()

	return r.rotate(ctx)
}

//nolint:gocyclo
func (r *rotator) rotate(ctx context.Context) error {
	r.printIntro()

	if len(r.controlPlaneNodes()) == 0 {
		return errors.New("no control plane nodes found")
	}

	if err := r.fetchCurrentCAs(ctx); err != nil {
		return err
	}

	if err := r.printNewCA(); err != nil {
		return err
	}

	if err := r.fetchKubernetesClient(ctx); err != nil {
		return err
	}

	if err := r.verifyConnectivity(ctx, "existing PKI"); err != nil {
		return err
	}

	if err := r.addNewCAAccepted(ctx); err != nil {
		return err
	}

	if err := r.verifyConnectivity(ctx, "existing PKI, new CA accepted"); err != nil {
		return err
	}

	if err := r.swapCAs(ctx); err != nil {
		return err
	}

	if err := r.verifyConnectivity(ctx, "new PKI, old CA accepted"); err != nil {
		return err
	}

	if err := r.dropOldCAs(ctx); err != nil {
		return err
	}

	return r.verifyConnectivity(ctx, "new PKI")
}

func (r *rotator) printIntro() {
	r.opts.Printf("> Starting etcd PKI rotation, dry-run mode %v...\n", r.opts.DryRun)

	r.opts.Printf("> Cluster topology:\n")

	r.opts.Printf(
		"  - control plane nodes: %q\n",
		helpers.MapToInternalIP(r.controlPlaneNodes()),
	)
}

func (r *rotator) controlPlaneNodes() []cluster.NodeInfo {
	return append(
		r.opts.ClusterInfo.NodesByType(machine.TypeInit),
		r.opts.ClusterInfo.NodesByType(machine.TypeControlPlane)...,
	)
}

func (r *rotator) printCA(ca *x509.PEMEncodedCertificateAndKey) error {
	var b bytes.Buffer

	if err := yaml.NewEncoder(&b).Encode(ca); err != nil {
		return fmt.Errorf("error encoding etcd CA: %w", err)
	}

	for scanner := bufio.NewScanner(&b); scanner.Scan(); {
		r.opts.Printf("  %s\n", scanner.Text())
	}

	return nil
}

// fetchCurrentCAs gathers the issuing etcd CAs from all control plane nodes.
//
// It also verifies that all control plane nodes are using the machine configuration which supports the rotation.
func (r *rotator) fetchCurrentCAs(ctx context.Context) error {
	r.opts.Printf("> Current etcd CA:\n")

	for _, node := range r.controlPlaneNodes() {
		nodeIP := node.InternalIP.String()

		issuingCA, err := r.fetchIssuingCA(ctx, nodeIP)
		if err != nil {
			return fmt.Errorf("node %s: %w", nodeIP, err)
		}

		if bytes.Equal(bytes.TrimSpace(issuingCA.Crt), bytes.TrimSpace(r.opts.NewEtcdCA.Crt)) {
			// the node is already using the new CA
			continue
		}

		if slices.ContainsFunc(r.oldCAs, func(ca []byte) bool { return bytes.Equal(ca, issuingCA.Crt) }) {
			continue
		}

		r.oldCAs = append(r.oldCAs, issuingCA.Crt)

		if len(r.oldCAs) > 1 {
			r.opts.Printf("> Another etcd CA is in use on %s (a previous rotation was interrupted?):\n", nodeIP)
		}

		if err = r.printCA(issuingCA); err != nil {
			return err
		}
	}

	if len(r.oldCAs) == 0 {
		r.opts.Printf("  (all control plane nodes are already using the new etcd CA)\n")
	}

	return nil
}

// fetchIssuingCA returns the issuing etcd CA from the machine configuration of the node.
func (r *rotator) fetchIssuingCA(ctx context.Context, nodeIP string) (*x509.PEMEncodedCertificateAndKey, error) {
	mc, err := safe.StateGetByID[*configres.MachineConfig](client.WithNode(ctx, nodeIP), r.opts.TalosClient.State(), configres.ActiveID)
	if err != nil {
		return nil, fmt.Errorf("error fetching machine config: %w", err)
	}

	if !mc.Provider().Has(etcdcfg.EtcdCAConfig) {
		return nil, rotatepatcher.ErrEtcdLegacyConfig
	}

	caConfig := mc.Provider().EtcdCAConfig()
	if caConfig == nil || caConfig.IssuingCA() == nil {
		return nil, errors.New("etcd issuing CA is not set")
	}

	return caConfig.IssuingCA(), nil
}

func (r *rotator) printNewCA() error {
	r.opts.Printf("> New etcd CA:\n")

	return r.printCA(r.opts.NewEtcdCA)
}

func (r *rotator) fetchKubernetesClient(ctx context.Context) error {
	r.opts.Printf("> Building Kubernetes client...\n")

	r.kubernetes = &cluster.KubernetesClient{
		ClientProvider: &cluster.ConfigClientProvider{
			DefaultClient: r.opts.TalosClient,
		},
		ForceEndpoint: r.opts.KubernetesEndpoint,
	}

	if _, err := r.kubernetes.K8sClient(client.WithNode(ctx, r.controlPlaneNodes()[0].InternalIP.String())); err != nil {
		return fmt.Errorf("error fetching kubeconfig: %w", err)
	}

	return nil
}

// verifyConnectivity checks that every etcd member is up, and it sees the leader of the cluster.
func (r *rotator) verifyConnectivity(ctx context.Context, label string) error {
	r.opts.Printf("> Verifying etcd health with %s...\n", label)

	if r.opts.DryRun {
		r.opts.Printf(" - OK (dry-run mode)\n")

		return nil
	}

	nodes := r.controlPlaneNodes()

	if err := retry.Constant(connectivityTimeout, retry.WithUnits(time.Second), retry.WithErrorLogging(true)).RetryWithContext(ctx,
		func(ctx context.Context) error {
			for _, node := range nodes {
				if err := r.checkEtcdMember(ctx, node.InternalIP.String()); err != nil {
					return err
				}
			}

			return nil
		},
	); err != nil {
		return err
	}

	r.opts.Printf(" - OK (%d members healthy)\n", len(nodes))

	return nil
}

// checkEtcdMember queries the status of the etcd member running on the node.
//
// The request is served by the node using its own etcd client credentials, so it verifies the etcd client PKI as well.
func (r *rotator) checkEtcdMember(ctx context.Context, nodeIP string) error {
	resp, err := r.opts.TalosClient.EtcdStatus(client.WithNode(ctx, nodeIP))
	if err != nil {
		if code := client.StatusCode(err); code == codes.Canceled || code == codes.PermissionDenied || code == codes.Unimplemented {
			return err
		}

		return retry.ExpectedErrorf("%s: error getting etcd status: %w", nodeIP, err)
	}

	for _, message := range resp.GetMessages() {
		status := message.GetMemberStatus()

		if len(status.GetErrors()) > 0 {
			return retry.ExpectedErrorf("%s: etcd member reports errors: %q", nodeIP, status.GetErrors())
		}

		if status.GetLeader() == 0 {
			return retry.ExpectedErrorf("%s: etcd member has no leader", nodeIP)
		}
	}

	return nil
}

func (r *rotator) addNewCAAccepted(ctx context.Context) error {
	r.opts.Printf("> Adding new etcd CA as accepted...\n")

	if err := r.patchAllNodes(ctx,
		rotatepatcher.EtcdAddAcceptedCA(r.opts.NewEtcdCA.Crt),
	); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	return nil
}

func (r *rotator) swapCAs(ctx context.Context) error {
	r.opts.Printf("> Making new etcd CA the issuing CA, old etcd CA the accepted CA...\n")

	if err := r.patchAllNodes(ctx,
		func(provider config.Provider) (config.Provider, error) {
			var err error

			// normally the old CA is the issuing CA of the node, and it is kept as accepted when the issuing CA is replaced,
			// but if the previous rotation was interrupted, the nodes might have different issuing CAs
			for _, oldCA := range r.oldCAs {
				if provider, err = rotatepatcher.EtcdAddAcceptedCA(oldCA)(provider); err != nil {
					return nil, err
				}
			}

			return rotatepatcher.EtcdSetCA(r.opts.NewEtcdCA)(provider)
		},
	); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	return nil
}

func (r *rotator) dropOldCAs(ctx context.Context) error {
	r.opts.Printf("> Removing old etcd CA from the accepted CAs...\n")

	extraAcceptedCAs := map[string]int{}

	if err := r.patchAllNodesWithNode(ctx,
		func(nodeIP string, provider config.Provider) (config.Provider, error) {
			var err error

			for _, oldCA := range r.oldCAs {
				if provider, err = rotatepatcher.EtcdDeleteAcceptedCA(oldCA)(provider); err != nil {
					return nil, err
				}
			}

			// the first accepted CA is the issuing CA
			extraAcceptedCAs[nodeIP] = len(provider.EtcdCAConfig().AcceptedCAs()) - 1

			return provider, nil
		},
	); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	for _, node := range r.controlPlaneNodes() {
		if count := extraAcceptedCAs[node.InternalIP.String()]; count > 0 {
			r.opts.Printf("  - %s: %d other accepted etcd CA(s) are left untouched, review the %s machine configuration document\n", node.InternalIP, count, etcdcfg.EtcdCAConfig)
		}
	}

	return nil
}

func (r *rotator) patchAllNodes(ctx context.Context, patchFunc func(config.Provider) (config.Provider, error)) error {
	return r.patchAllNodesWithNode(ctx, func(_ string, provider config.Provider) (config.Provider, error) {
		return patchFunc(provider)
	})
}

func (r *rotator) patchAllNodesWithNode(ctx context.Context, patchFunc func(nodeIP string, provider config.Provider) (config.Provider, error)) error {
	for _, node := range r.controlPlaneNodes() {
		nodeIP := node.InternalIP.String()

		if r.opts.DryRun {
			r.opts.Printf("  - %s: skipped (dry-run)\n", nodeIP)

			continue
		}

		if err := r.patchNode(ctx, nodeIP, patchFunc); err != nil {
			return fmt.Errorf("error patching node %s: %w", nodeIP, err)
		}

		r.opts.Printf("  - %s: OK\n", nodeIP)
	}

	return nil
}

// expectedPKI describes the etcd PKI the node should be using after the machine configuration is patched.
type expectedPKI struct {
	// cas is the fingerprint of the set of the trusted etcd CAs.
	cas string
	// issuingCA is the fingerprint of the issuing etcd CA.
	issuingCA string
}

// patchNode patches the machine configuration of the node, and waits for the new PKI to be in use.
//
// As the trusted CAs are only loaded on start, etcd and kube-apiserver are restarted by Talos when the set of the etcd CAs
// changes. The fingerprint of the CAs they were started with is reported back, so it is compared with the fingerprint
// of the CAs in the patched machine configuration.
//
// When only the issuing CA changes (while the set of the trusted CAs stays the same), nothing is restarted: both etcd
// and kube-apiserver load the certificates on each connection, so it is enough to wait for the certificates to be re-issued.
func (r *rotator) patchNode(ctx context.Context, nodeIP string, patchFunc func(nodeIP string, provider config.Provider) (config.Provider, error)) error {
	// don't take down an etcd member if the cluster is not healthy: there might be no quorum left
	for _, node := range r.controlPlaneNodes() {
		if err := r.checkEtcdMember(ctx, node.InternalIP.String()); err != nil {
			return fmt.Errorf("etcd is not healthy: %w", err)
		}
	}

	var expected expectedPKI

	if err := helpers.PatchNodeConfig(ctx, r.opts.TalosClient, nodeIP, r.opts.EncoderOption,
		func(provider config.Provider) (config.Provider, error) {
			patched, err := patchFunc(nodeIP, provider)
			if err != nil {
				return nil, err
			}

			caConfig := patched.EtcdCAConfig()

			expected = expectedPKI{
				cas:       pemutils.Fingerprint(caConfig.AcceptedCAs()),
				issuingCA: pemutils.Fingerprint([]*x509.PEMEncodedCertificate{{Crt: caConfig.IssuingCA().Crt}}),
			}

			return patched, nil
		},
	); err != nil {
		return fmt.Errorf("error patching node config: %w", err)
	}

	if err := retry.Constant(etcdRestartTimeout, retry.WithUnits(time.Second), retry.WithErrorLogging(true)).RetryWithContext(ctx,
		func(ctx context.Context) error {
			return r.checkEtcd(ctx, nodeIP, expected)
		},
	); err != nil {
		return fmt.Errorf("error waiting for etcd: %w", err)
	}

	clientset, err := r.kubernetes.K8sClient(ctx)
	if err != nil {
		return fmt.Errorf("error building Kubernetes client: %w", err)
	}

	if err = retry.Constant(apiServerRestartTimeout, retry.WithUnits(time.Second), retry.WithErrorLogging(true)).RetryWithContext(ctx,
		func(ctx context.Context) error {
			return r.checkAPIServer(ctx, clientset, nodeIP, expected)
		},
	); err != nil {
		return fmt.Errorf("error waiting for %s: %w", k8s.APIServerID, err)
	}

	return nil
}

// getResource fetches the resource from the node, any failure but the fatal one is retried.
func getResource[T resource.Resource](ctx context.Context, r *rotator, nodeIP string, ptr resource.Pointer) (T, error) {
	res, err := safe.StateGet[T](client.WithNode(ctx, nodeIP), r.opts.TalosClient.State(), ptr)
	if err != nil {
		if code := client.StatusCode(err); code == codes.Canceled || code == codes.PermissionDenied {
			return res, err
		}

		return res, retry.ExpectedErrorf("%s: error getting %s: %w", nodeIP, ptr.Type(), err)
	}

	return res, nil
}

// checkEtcd verifies that etcd on the node is running with the expected PKI, and it is healthy.
func (r *rotator) checkEtcd(ctx context.Context, nodeIP string, expected expectedPKI) error {
	pkiStatus, err := getResource[*etcdres.PKIStatus](ctx, r, nodeIP, etcdres.NewPKIStatus(etcdres.NamespaceName, etcdres.PKIID).Metadata())
	if err != nil {
		return err
	}

	if pkiStatus.TypedSpec().CAFingerprint != expected.cas || pkiStatus.TypedSpec().IssuingCAFingerprint != expected.issuingCA {
		return retry.ExpectedErrorf("%s: waiting for the etcd PKI to be updated", nodeIP)
	}

	serviceSpec, err := getResource[*etcdres.ServiceSpec](ctx, r, nodeIP, etcdres.NewServiceSpec().Metadata())
	if err != nil {
		return err
	}

	if serviceSpec.TypedSpec().CAFingerprint != expected.cas {
		return retry.ExpectedErrorf("%s: waiting for etcd to be restarted with the new PKI", nodeIP)
	}

	service, err := getResource[*v1alpha1res.Service](ctx, r, nodeIP, v1alpha1res.NewService("etcd").Metadata())
	if err != nil {
		return err
	}

	if !service.TypedSpec().Running || !service.TypedSpec().Healthy {
		return retry.ExpectedErrorf("%s: etcd is not healthy", nodeIP)
	}

	// the service status is updated asynchronously, so query etcd directly as well
	return r.checkEtcdMember(ctx, nodeIP)
}

// checkAPIServer verifies that kube-apiserver on the node is running with the expected etcd PKI, and it is ready.
//
//nolint:gocyclo
func (r *rotator) checkAPIServer(ctx context.Context, clientset k8sclient.Interface, nodeIP string, expected expectedPKI) error {
	secretsStatus, err := getResource[*k8s.SecretsStatus](ctx, r, nodeIP, k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID).Metadata())
	if err != nil {
		return err
	}

	if secretsStatus.TypedSpec().EtcdCAFingerprint != expected.cas || secretsStatus.TypedSpec().EtcdIssuingCAFingerprint != expected.issuingCA {
		return retry.ExpectedErrorf("%s: %s: waiting for the etcd PKI to be updated", nodeIP, k8s.APIServerID)
	}

	pods, err := clientset.CoreV1().Pods(metav1.NamespaceSystem).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("k8s-app = %s", k8s.APIServerID),
	})
	if err != nil {
		if taloskubernetes.IsRetryableError(err) {
			return retry.ExpectedError(err)
		}

		return err
	}

	for _, pod := range pods.Items {
		if pod.Status.HostIP != nodeIP {
			continue
		}

		if pod.Annotations[constants.AnnotationStaticPodEtcdCAFingerprint] != expected.cas {
			return retry.ExpectedErrorf("%s: %s: waiting for the pod to be restarted with the new etcd PKI", nodeIP, k8s.APIServerID)
		}

		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				return nil
			}
		}

		return retry.ExpectedErrorf("%s: %s: pod is not ready", nodeIP, k8s.APIServerID)
	}

	return retry.ExpectedErrorf("%s: %s: pod is not found", nodeIP, k8s.APIServerID)
}
