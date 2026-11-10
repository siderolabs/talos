// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kubernetes implements safe Talos API PKI rotation for the cluster.
package kubernetes

import (
	"bufio"
	"bytes"
	"context"
	stdx509 "crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-retry/retry"
	"go.yaml.in/yaml/v4"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/siderolabs/talos/pkg/cluster"
	taloskubernetes "github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/rotatepatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	secretsres "github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/rotate/pki/internal/helpers"
)

// Options is the input to the Kubernetes API rotation process.
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

	// NewKubernetesCA is the new CA for Kubernetes API.
	NewKubernetesCA *x509.PEMEncodedCertificateAndKey

	// EncoderOption is the option for encoding machine configuration (while patching).
	EncoderOption encoder.Option

	// Printf is the function used to print messages.
	Printf func(format string, args ...any)
}

type rotator struct {
	opts Options

	currentCA []byte

	talosClientProvider *cluster.ConfigClientProvider
	currentKubernetes   *cluster.KubernetesClient
	newKubernetes       *cluster.KubernetesClient
}

// Rotate rotates the Kubernetes API PKI.
//
// The process overview:
//   - fetch current information
//   - verify connectivity with the existing PKI
//   - add new Kubernetes CA as accepted
//   - verify connectivity
//   - make new CA issuing, old CA is still accepted
//   - verify connectivity with the new PKI
//   - wait for the control plane static pods to be updated
//   - remove old CA
//   - verify connectivity with the new PKI
//   - wait for the control plane static pods to be updated.
func Rotate(ctx context.Context, opts Options) error {
	r := rotator{
		opts: opts,
	}

	defer func() {
		if r.currentKubernetes != nil {
			r.currentKubernetes.K8sClose() //nolint:errcheck
		}

		if r.newKubernetes != nil {
			r.newKubernetes.K8sClose() //nolint:errcheck
		}
	}()

	r.talosClientProvider = &cluster.ConfigClientProvider{
		DefaultClient: opts.TalosClient,
	}

	return r.rotate(ctx)
}

//nolint:gocyclo
func (r *rotator) rotate(ctx context.Context) error {
	r.printIntro()

	if err := r.fetchClient(ctx, &r.currentKubernetes, "current"); err != nil {
		return err
	}

	if err := r.fetchCurrentCA(ctx); err != nil {
		return err
	}

	if err := r.printNewCA(); err != nil {
		return err
	}

	if err := r.verifyConnectivity(ctx, r.currentKubernetes, "existing PKI"); err != nil {
		return err
	}

	if err := r.addNewCAAccepted(ctx); err != nil {
		return err
	}

	if err := r.swapCAs(ctx); err != nil {
		return err
	}

	if err := r.fetchClient(ctx, &r.newKubernetes, "new"); err != nil {
		return err
	}

	if err := r.verifyConnectivity(ctx, r.newKubernetes, "new PKI"); err != nil {
		return err
	}

	// The old CA can only be dropped once the control plane components run with the credentials issued by the new CA:
	// kubelets lose their client certificates when the old CA is dropped, and they can only get new ones if
	// kube-controller-manager is able to sign them, i.e. if it doesn't use the client certificate issued by the old CA.
	if err := r.waitForControlPlane(ctx, false); err != nil {
		return err
	}

	if err := r.dropOldCA(ctx); err != nil {
		return err
	}

	if err := r.verifyConnectivity(ctx, r.newKubernetes, "new PKI"); err != nil {
		return err
	}

	return r.waitForControlPlane(ctx, true)
}

func (r *rotator) printIntro() {
	r.opts.Printf("> Starting Kubernetes API PKI rotation, dry-run mode %v...\n", r.opts.DryRun)

	r.opts.Printf("> Cluster topology:\n")

	r.opts.Printf(
		"  - control plane nodes: %q\n",
		append(
			helpers.MapToInternalIP(r.opts.ClusterInfo.NodesByType(machine.TypeInit)),
			helpers.MapToInternalIP(r.opts.ClusterInfo.NodesByType(machine.TypeControlPlane))...,
		),
	)
	r.opts.Printf(
		"  - worker nodes: %q\n",
		helpers.MapToInternalIP(r.opts.ClusterInfo.NodesByType(machine.TypeWorker)),
	)
}

func (r *rotator) fetchClient(ctx context.Context, clientPtr **cluster.KubernetesClient, label string) error {
	r.opts.Printf("> Building %s Kubernetes client...\n", label)

	firstNode := r.controlPlaneNodes()[0]

	*clientPtr = &cluster.KubernetesClient{
		ClientProvider: r.talosClientProvider,
		ForceEndpoint:  r.opts.KubernetesEndpoint,
	}

	_, err := (*clientPtr).K8sClient(client.WithNode(ctx, firstNode.InternalIP.String()))
	if err != nil {
		return fmt.Errorf("error fetching kubeconfig: %w", err)
	}

	return nil
}

func (r *rotator) fetchCurrentCA(ctx context.Context) error {
	r.opts.Printf("> Current Kubernetes CA:\n")

	firstNode := r.controlPlaneNodes()[0]

	k8sRoot, err := safe.StateGetByID[*secretsres.KubernetesRoot](client.WithNode(ctx, firstNode.InternalIP.String()), r.opts.TalosClient.State(), secretsres.KubernetesRootID)
	if err != nil {
		return fmt.Errorf("error fetching current Kubernetes CA: %w", err)
	}

	r.currentCA = k8sRoot.TypedSpec().IssuingCA.Crt

	var b bytes.Buffer

	if err = yaml.NewEncoder(&b).Encode(k8sRoot.TypedSpec().IssuingCA); err != nil {
		return fmt.Errorf("error encoding current Kubernetes CA: %w", err)
	}

	for scanner := bufio.NewScanner(&b); scanner.Scan(); {
		r.opts.Printf("  %s\n", scanner.Text())
	}

	return nil
}

func (r *rotator) printNewCA() error {
	r.opts.Printf("> New Kubernetes CA:\n")

	var b bytes.Buffer

	if err := yaml.NewEncoder(&b).Encode(r.opts.NewKubernetesCA); err != nil {
		return fmt.Errorf("error encoding new Talos CA: %w", err)
	}

	for scanner := bufio.NewScanner(&b); scanner.Scan(); {
		r.opts.Printf("  %s\n", scanner.Text())
	}

	return nil
}

func (r *rotator) verifyConnectivity(ctx context.Context, client *cluster.KubernetesClient, label string) error {
	r.opts.Printf("> Verifying connectivity with %s...\n", label)

	if r.opts.DryRun {
		r.opts.Printf(" - OK (dry-run mode)\n")

		return nil
	}

	clientset, err := client.K8sClient(ctx)
	if err != nil {
		return fmt.Errorf("error building Kubernetes client: %w", err)
	}

	return retry.Constant(3*time.Minute, retry.WithUnits(time.Second), retry.WithErrorLogging(true)).RetryWithContext(ctx,
		func(ctx context.Context) error {
			nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				if taloskubernetes.IsRetryableError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			var notReadyNodes []string

			for _, node := range nodes.Items {
				for _, cond := range node.Status.Conditions {
					if cond.Type == corev1.NodeReady {
						if cond.Status != corev1.ConditionTrue {
							notReadyNodes = append(notReadyNodes, node.Name)

							break
						}
					}
				}
			}

			if len(notReadyNodes) > 0 {
				return retry.ExpectedErrorf("nodes not ready: %q", notReadyNodes)
			}

			r.opts.Printf(" - OK (%d nodes ready)\n", len(nodes.Items))

			return nil
		})
}

func (r *rotator) controlPlaneNodes() []cluster.NodeInfo {
	return append(
		r.opts.ClusterInfo.NodesByType(machine.TypeInit),
		r.opts.ClusterInfo.NodesByType(machine.TypeControlPlane)...,
	)
}

// waitForControlPlane waits for the control plane static pods to be running with the secrets matching the new PKI.
//
// Every change to the Kubernetes PKI re-renders the secrets of the control plane static pods (the CA bundle,
// the kubeconfigs of kube-controller-manager and kube-scheduler issued by the issuing CA, etc.), which in turn bumps the
// secrets version annotation of the static pods.
//
// As the secrets are rendered asynchronously after the machine configuration is patched, the expected secrets version
// is found by waiting for the secrets to match the new PKI: the client certificate of kube-controller-manager should
// be issued by the new CA, and (if oldCADropped is set) the old CA should no longer be accepted.
//
// As the kubelet is restarted on every machine configuration change, the static pods which were running
// before the restart might be left behind holding the ports, so the updated static pods keep crashing until the kubelet
// cleans them up, which requires a working connection to the API server.
func (r *rotator) waitForControlPlane(ctx context.Context, oldCADropped bool) error {
	r.opts.Printf("> Waiting for control plane static pods to be updated...\n")

	if r.opts.DryRun {
		r.opts.Printf(" - OK (dry-run mode)\n")

		return nil
	}

	newCA, err := parseCertificate(r.opts.NewKubernetesCA.Crt)
	if err != nil {
		return fmt.Errorf("error parsing new Kubernetes CA: %w", err)
	}

	oldCA, err := parseCertificate(r.currentCA)
	if err != nil {
		return fmt.Errorf("error parsing current Kubernetes CA: %w", err)
	}

	clientset, err := r.newKubernetes.K8sClient(ctx)
	if err != nil {
		return fmt.Errorf("error building Kubernetes client: %w", err)
	}

	for _, node := range r.controlPlaneNodes() {
		nodeIP := node.InternalIP.String()

		var expectedVersion resource.Version

		if err = retry.Constant(time.Minute, retry.WithUnits(time.Second), retry.WithErrorLogging(true)).RetryWithContext(ctx,
			func(ctx context.Context) error {
				expectedVersion, err = r.checkSecrets(ctx, nodeIP, newCA, oldCA, oldCADropped)

				return err
			},
		); err != nil {
			return fmt.Errorf("error waiting for control plane secrets on %s: %w", nodeIP, err)
		}

		if err = r.waitForStaticPods(ctx, clientset, nodeIP, expectedVersion); err != nil {
			return err
		}
	}

	return nil
}

func (r *rotator) waitForStaticPods(ctx context.Context, clientset k8sclient.Interface, nodeIP string, expectedVersion resource.Version) error {
	for _, service := range []string{k8s.APIServerID, k8s.ControllerManagerID, k8s.SchedulerID} {
		if err := retry.Constant(5*time.Minute, retry.WithUnits(time.Second), retry.WithErrorLogging(true)).RetryWithContext(ctx,
			func(ctx context.Context) error {
				return checkStaticPod(ctx, clientset, nodeIP, service, expectedVersion)
			},
		); err != nil {
			return fmt.Errorf("error waiting for %s on %s: %w", service, nodeIP, err)
		}

		r.opts.Printf("  - %s: %s OK\n", nodeIP, service)
	}

	return nil
}

// checkSecrets verifies that the control plane secrets on the node match the new PKI, and returns their version.
func (r *rotator) checkSecrets(ctx context.Context, nodeIP string, newCA, oldCA *stdx509.Certificate, oldCADropped bool) (resource.Version, error) {
	k8sSecrets, err := safe.StateGetByID[*secretsres.Kubernetes](client.WithNode(ctx, nodeIP), r.opts.TalosClient.State(), secretsres.KubernetesID)
	if err != nil {
		return resource.Version{}, fmt.Errorf("error fetching Kubernetes secrets: %w", err)
	}

	kubeconfig, err := clientcmd.Load([]byte(k8sSecrets.TypedSpec().ControllerManagerKubeconfig))
	if err != nil {
		return resource.Version{}, fmt.Errorf("error parsing kube-controller-manager kubeconfig: %w", err)
	}

	for _, authInfo := range kubeconfig.AuthInfos {
		cert, err := parseCertificate(authInfo.ClientCertificateData)
		if err != nil {
			return resource.Version{}, fmt.Errorf("error parsing kube-controller-manager client certificate: %w", err)
		}

		if cert.CheckSignatureFrom(newCA) != nil {
			return resource.Version{}, retry.ExpectedErrorf("%s: kube-controller-manager client certificate is not issued by the new CA yet", nodeIP)
		}
	}

	if oldCADropped {
		if err = checkCADropped(kubeconfig, oldCA); err != nil {
			return resource.Version{}, fmt.Errorf("%s: %w", nodeIP, err)
		}
	}

	return k8sSecrets.Metadata().Version(), nil
}

func checkCADropped(kubeconfig *clientcmdapi.Config, oldCA *stdx509.Certificate) error {
	for _, cluster := range kubeconfig.Clusters {
		acceptedCAs, err := parseCertificates(cluster.CertificateAuthorityData)
		if err != nil {
			return fmt.Errorf("error parsing kube-controller-manager CA bundle: %w", err)
		}

		if slices.ContainsFunc(acceptedCAs, oldCA.Equal) {
			return retry.ExpectedErrorf("old CA is still accepted")
		}
	}

	return nil
}

func parseCertificate(data []byte) (*stdx509.Certificate, error) {
	certs, err := parseCertificates(data)
	if err != nil {
		return nil, err
	}

	if len(certs) == 0 {
		return nil, errors.New("no certificates found")
	}

	return certs[0], nil
}

func parseCertificates(data []byte) ([]*stdx509.Certificate, error) {
	var certs []*stdx509.Certificate

	for {
		var block *pem.Block

		block, data = pem.Decode(data)
		if block == nil {
			return certs, nil
		}

		cert, err := stdx509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}

		certs = append(certs, cert)
	}
}

func checkStaticPod(ctx context.Context, clientset k8sclient.Interface, nodeIP, service string, expectedVersion resource.Version) error {
	pods, err := clientset.CoreV1().Pods(metav1.NamespaceSystem).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("k8s-app = %s", service),
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

		// the static pod secrets version is the version of the secrets resource the pod was rendered from, so the pod is up-to-date
		// as soon as it reaches the expected version: an unrelated change might have bumped it even further
		secretsVersion, err := resource.ParseVersion(pod.Annotations[constants.AnnotationStaticPodSecretsVersion])
		if err != nil || secretsVersion.Value() < expectedVersion.Value() {
			return retry.ExpectedErrorf("%s: %s: waiting, secrets version mismatch: got %q, expected %q",
				nodeIP, service, pod.Annotations[constants.AnnotationStaticPodSecretsVersion], expectedVersion)
		}

		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				return nil
			}
		}

		return retry.ExpectedErrorf("%s: %s: pod is not ready", nodeIP, service)
	}

	return retry.ExpectedErrorf("%s: %s: pod is not found", nodeIP, service)
}

func (r *rotator) addNewCAAccepted(ctx context.Context) error {
	r.opts.Printf("> Adding new Kubernetes CA as accepted...\n")

	if err := r.patchAllNodes(ctx,
		rotatepatcher.K8sAddAcceptedCA(r.opts.NewKubernetesCA.Crt),
	); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	return nil
}

func (r *rotator) swapCAs(ctx context.Context) error {
	r.opts.Printf("> Making new Kubernetes CA the issuing CA, old Kubernetes CA the accepted CA...\n")

	if err := r.patchAllNodes(ctx,
		func(provider config.Provider) (config.Provider, error) {
			provider, err := rotatepatcher.K8sAddAcceptedCA(r.currentCA)(provider)
			if err != nil {
				return nil, err
			}

			provider, err = rotatepatcher.K8sDeleteAcceptedCA(r.opts.NewKubernetesCA.Crt)(provider)
			if err != nil {
				return nil, err
			}

			return rotatepatcher.K8sSetCA(r.opts.NewKubernetesCA)(provider)
		}); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	return nil
}

func (r *rotator) dropOldCA(ctx context.Context) error {
	r.opts.Printf("> Removing old Kubernetes CA from the accepted CAs...\n")

	if err := r.patchAllNodes(
		ctx,
		rotatepatcher.K8sDeleteAcceptedCA(r.currentCA),
	); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	return nil
}

func (r *rotator) patchAllNodes(ctx context.Context, patchFunc func(provider config.Provider) (config.Provider, error)) error {
	for _, machineType := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker} {
		for _, node := range r.opts.ClusterInfo.NodesByType(machineType) {
			if r.opts.DryRun {
				r.opts.Printf("  - %s: skipped (dry-run)\n", node.InternalIP)

				continue
			}

			if err := helpers.PatchNodeConfigWithKubeletRestart(
				ctx, r.opts.TalosClient, node.InternalIP.String(), r.opts.EncoderOption,
				patchFunc,
			); err != nil {
				return fmt.Errorf("error patching node %s: %w", node.InternalIP, err)
			}

			r.opts.Printf("  - %s: OK\n", node.InternalIP)
		}
	}

	return nil
}
