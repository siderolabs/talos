// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	taloskubernetes "github.com/siderolabs/go-kubernetes/kubernetes"
	"github.com/siderolabs/go-retry/retry"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

const (
	// DrainTimeout is maximum time to wait for the node to be drained.
	DrainTimeout = 5 * time.Minute
)

// Client represents a set of helper methods for interacting with the
// Kubernetes API.
type Client struct {
	*taloskubernetes.Client
}

// NewClientFromKubeletKubeconfig initializes and returns a Client.
func NewClientFromKubeletKubeconfig() (*Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", constants.KubeletKubeconfig)
	if err != nil {
		return nil, err
	}

	return NewForConfig(config)
}

// NewForConfig initializes and returns a client using the provided config.
func NewForConfig(config *restclient.Config) (*Client, error) {
	client, err := taloskubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		Client: client,
	}, nil
}

// NewClientFromPKI initializes and returns a Client.
//
//nolint:interfacer
func NewClientFromPKI(ca, crt, key []byte, endpoint *url.URL) (*Client, error) {
	tlsClientConfig := restclient.TLSClientConfig{
		CAData:   ca,
		CertData: crt,
		KeyData:  key,
	}

	config := &restclient.Config{
		Host:            endpoint.String(),
		TLSClientConfig: tlsClientConfig,
		Timeout:         30 * time.Second,
	}

	return NewForConfig(config)
}

// NewTemporaryClientControlPlane initializes a Kubernetes client for a controlplane node
// using PKI information.
//
// The client uses "localhost" endpoint, so it doesn't depend on the loadbalancer to be ready.
func NewTemporaryClientControlPlane(ctx context.Context, r controller.Reader) (client *Client, err error) {
	k8sRoot, err := safe.ReaderGet[*secrets.KubernetesRoot](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesRootType, secrets.KubernetesRootID, resource.VersionUndefined))
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	k8sRootSpec := k8sRoot.TypedSpec()

	return NewTemporaryClientFromPKI(k8sRootSpec.IssuingCA, k8sRootSpec.LocalEndpoint)
}

// NewTemporaryClientFromPKI initializes a Kubernetes client using a certificate
// with a TTL of 10 minutes.
func NewTemporaryClientFromPKI(ca *x509.PEMEncodedCertificateAndKey, endpoint *url.URL) (client *Client, err error) {
	opts := []x509.Option{
		x509.CommonName(constants.KubernetesAdminCertCommonName),
		x509.Organization(constants.KubernetesAdminCertOrganization),
		x509.NotBefore(time.Now().Add(-time.Minute)), // allow for a minute for the time to be not in sync across nodes
		x509.NotAfter(time.Now().Add(10 * time.Minute)),
	}

	k8sCA, err := x509.NewCertificateAuthorityFromCertificateAndKey(ca)
	if err != nil {
		return nil, fmt.Errorf("failed decoding Kubernetes CA: %w", err)
	}

	keyPair, err := x509.NewKeyPair(k8sCA, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed generating temporary cert: %w", err)
	}

	h, err := NewClientFromPKI(ca.Crt, keyPair.CrtPEM, keyPair.KeyPEM, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return h, nil
}

// NodeIPs returns list of node IP addresses by machine type.
//
//nolint:gocyclo
func (h *Client) NodeIPs(ctx context.Context, machineType machine.Type) (addrs []string, err error) {
	resp, err := h.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	addrs = []string{}

	for _, node := range resp.Items {
		_, labelControlPlane := node.Labels[constants.LabelNodeRoleControlPlane]

		var skip, foundInternalIP bool

		switch machineType {
		case machine.TypeInit, machine.TypeControlPlane:
			skip = !labelControlPlane
		case machine.TypeWorker:
			skip = labelControlPlane
		case machine.TypeUnknown:
			fallthrough
		default:
			panic(fmt.Sprintf("unexpected machine type %v", machineType))
		}

		if skip {
			continue
		}

		// try to get the internal IP address
		for _, nodeAddress := range node.Status.Addresses {
			if nodeAddress.Type == corev1.NodeInternalIP {
				addrs = append(addrs, nodeAddress.Address)
				foundInternalIP = true

				break
			}
		}

		if !foundInternalIP {
			// no internal IP, fallback to external IP
			for _, nodeAddress := range node.Status.Addresses {
				if nodeAddress.Type == corev1.NodeExternalIP {
					addrs = append(addrs, nodeAddress.Address)

					break
				}
			}
		}
	}

	return addrs, nil
}

// Drain evicts all pods on a given node.
func (h *Client) Drain(ctx context.Context, node string) error {
	ctx, cancel := context.WithTimeout(ctx, DrainTimeout)
	defer cancel()

	opts := metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node}).String(),
	}

	pods, err := h.CoreV1().Pods(metav1.NamespaceAll).List(ctx, opts)
	if err != nil {
		return fmt.Errorf("cannot get pods for node %s: %w", node, err)
	}

	var eg errgroup.Group

	// Evict each pod.

	for _, pod := range pods.Items {
		eg.Go(func() error {
			if _, ok := pod.ObjectMeta.Annotations[corev1.MirrorPodAnnotationKey]; ok {
				log.Printf("skipping mirror pod %s/%s\n", pod.GetNamespace(), pod.GetName())

				return nil
			}

			controllerRef := metav1.GetControllerOf(&pod)

			if controllerRef == nil {
				log.Printf("skipping unmanaged pod %s/%s\n", pod.GetNamespace(), pod.GetName())

				return nil
			}

			if controllerRef.Kind == appsv1.SchemeGroupVersion.WithKind("DaemonSet").Kind {
				log.Printf("skipping DaemonSet pod %s/%s\n", pod.GetNamespace(), pod.GetName())

				return nil
			}

			if !pod.DeletionTimestamp.IsZero() {
				log.Printf("skipping deleted pod %s/%s\n", pod.GetNamespace(), pod.GetName())
			}

			if err := h.evict(ctx, pod, int64(60)); err != nil {
				log.Printf("WARNING: failed to evict pod: %v", err)
			}

			return nil
		})
	}

	return eg.Wait()
}

func (h *Client) evict(ctx context.Context, p corev1.Pod, gracePeriod int64) error {
	for {
		pol := &policy.Eviction{
			ObjectMeta:    metav1.ObjectMeta{Namespace: p.GetNamespace(), Name: p.GetName()},
			DeleteOptions: &metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod},
		}
		err := h.CoreV1().Pods(p.GetNamespace()).Evict(ctx, pol)

		switch {
		case apierrors.IsTooManyRequests(err):
			time.Sleep(5 * time.Second)
		case apierrors.IsNotFound(err):
			return nil
		case err != nil:
			return fmt.Errorf("failed to evict pod %s/%s: %w", p.GetNamespace(), p.GetName(), err)
		default:
			if err = h.waitForPodDeleted(ctx, &p); err != nil {
				return fmt.Errorf("failed waiting on pod %s/%s to be deleted: %w", p.GetNamespace(), p.GetName(), err)
			}

			return nil
		}
	}
}

func (h *Client) waitForPodDeleted(ctx context.Context, p *corev1.Pod) error {
	return retry.Constant(time.Minute, retry.WithUnits(3*time.Second)).RetryWithContext(ctx, func(ctx context.Context) error {
		pod, err := h.CoreV1().Pods(p.GetNamespace()).Get(ctx, p.GetName(), metav1.GetOptions{})

		switch {
		case apierrors.IsNotFound(err):
			return nil
		case apierrors.IsForbidden(err):
			// in Kubernetes 1.32+, NodeRestriction plugin won't let us list a pod which is not on our node, including deleted ones
			return nil
		case err != nil:
			if IsRetryableError(err) {
				return retry.ExpectedError(err)
			}

			return fmt.Errorf("failed to get pod %s/%s: %w", p.GetNamespace(), p.GetName(), err)
		}

		if pod.GetUID() != p.GetUID() {
			return nil
		}

		return retry.ExpectedErrorf("pod is still running on the node")
	})
}
