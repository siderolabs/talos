// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	stdlibx509 "crypto/x509"

	corev1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// Client represents a set of helper methods for interacting with the
// Kubernetes API.
type Client struct {
	*kubernetes.Clientset
}

// NewClientFromKubeletKubeconfig initializes and returns a Client.
func NewClientFromKubeletKubeconfig() (client *Client, err error) {
	var config *restclient.Config

	config, err = clientcmd.BuildConfigFromFlags("", constants.KubeletKubeconfig)
	if err != nil {
		return nil, err
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	var clientset *kubernetes.Clientset

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{clientset}, nil
}

// NewForConfig initializes and returns a client using the provided config.
func NewForConfig(config *restclient.Config) (client *Client, err error) {
	var clientset *kubernetes.Clientset

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{clientset}, nil
}

// NewClientFromPKI initializes and returns a Client.
//
// nolint: interfacer
func NewClientFromPKI(ca, crt, key []byte, endpoint *url.URL) (client *Client, err error) {
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

	var clientset *kubernetes.Clientset

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{clientset}, nil
}

// NewTemporaryClientFromPKI initializes a Kubernetes client using a certificate
// with a TTL of 10 minutes.
func NewTemporaryClientFromPKI(ca *x509.PEMEncodedCertificateAndKey, endpoint *url.URL) (client *Client, err error) {
	opts := []x509.Option{
		x509.RSA(true),
		x509.CommonName("admin"),
		x509.Organization("system:masters"),
		x509.NotAfter(time.Now().Add(10 * time.Minute)),
	}

	key, err := x509.NewRSAKey()
	if err != nil {
		return nil, fmt.Errorf("failed to create RSA key: %w", err)
	}

	keyBlock, _ := pem.Decode(key.KeyPEM)
	if keyBlock == nil {
		return nil, errors.New("failed to decode key")
	}

	keyRSA, err := stdlibx509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	csr, err := x509.NewCertificateSigningRequest(keyRSA, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSR: %w", err)
	}

	crt, err := x509.NewCertificateFromCSRBytes(ca.Crt, ca.Key, csr.X509CertificateRequestPEM, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate from CSR: %w", err)
	}

	h, err := NewClientFromPKI(ca.Crt, crt.X509CertificatePEM, key.KeyPEM, endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return h, nil
}

// MasterIPs returns a list of control plane endpoints (IP addresses).
func (h *Client) MasterIPs(ctx context.Context) (addrs []string, err error) {
	endpoints, err := h.CoreV1().Endpoints("default").Get(ctx, "kubernetes", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	addrs = []string{}

	for _, endpoint := range endpoints.Subsets {
		for _, addr := range endpoint.Addresses {
			addrs = append(addrs, addr.IP)
		}
	}

	return addrs, nil
}

// WorkerIPs returns list of worker nodes IP addresses.
func (h *Client) WorkerIPs(ctx context.Context) (addrs []string, err error) {
	resp, err := h.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	addrs = []string{}

	for _, node := range resp.Items {
		if _, ok := node.Labels[constants.LabelNodeRoleMaster]; ok {
			continue
		}

		for _, nodeAddress := range node.Status.Addresses {
			if nodeAddress.Type == corev1.NodeInternalIP {
				addrs = append(addrs, nodeAddress.Address)
			}
		}
	}

	return addrs, nil
}

// LabelNodeAsMaster labels a node with the required master label and NoSchedule taint.
//
//nolint: gocyclo
func (h *Client) LabelNodeAsMaster(name string) (err error) {
	n, err := h.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// The node may appear to have no labels at first, so we check for the
	// existence of a well known label to ensure the patch will be successful.
	if _, found := n.ObjectMeta.Labels[corev1.LabelHostname]; !found {
		return errors.New("could not find hostname label")
	}

	oldData, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("failed to marshal unmodified node %q into JSON: %w", n.Name, err)
	}

	n.Labels[constants.LabelNodeRoleMaster] = ""

	taintFound := false

	for _, taint := range n.Spec.Taints {
		if taint.Key == constants.LabelNodeRoleMaster && taint.Value == "true" {
			taintFound = true
			break
		}
	}

	if !taintFound {
		n.Spec.Taints = append(n.Spec.Taints, corev1.Taint{
			Key:    constants.LabelNodeRoleMaster,
			Value:  "true",
			Effect: corev1.TaintEffectNoSchedule,
		})
	}

	newData, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("failed to marshal modified node %q into JSON: %w", n.Name, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch: %w", err)
	}

	if _, err := h.CoreV1().Nodes().Patch(context.TODO(), n.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		if apierrors.IsConflict(err) {
			return fmt.Errorf("unable to update node metadata due to conflict: %w", err)
		}

		return fmt.Errorf("error patching node %q: %w", n.Name, err)
	}

	return nil
}

// WaitUntilReady waits for a node to be ready.
func (h *Client) WaitUntilReady(name string) error {
	return retry.Exponential(3*time.Minute, retry.WithUnits(250*time.Millisecond), retry.WithJitter(50*time.Millisecond)).Retry(func() error {
		attemptCtx, attemptCtxCancel := context.WithTimeout(context.TODO(), 30*time.Second)
		defer attemptCtxCancel()

		node, err := h.CoreV1().Nodes().Get(attemptCtx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return retry.ExpectedError(err)
			}

			if apierrors.ReasonForError(err) == metav1.StatusReasonUnknown {
				// non-API error, e.g. networking error
				return retry.ExpectedError(err)
			}

			return retry.UnexpectedError(err)
		}

		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady {
				if cond.Status != corev1.ConditionTrue {
					return retry.ExpectedError(fmt.Errorf("node not ready"))
				}
			}
		}

		return nil
	})
}

// CordonAndDrain cordons and drains a node in one call.
func (h *Client) CordonAndDrain(node string) (err error) {
	if err = h.Cordon(node); err != nil {
		return err
	}

	return h.Drain(node)
}

const (
	talosCordonedAnnotationName  = "talos.dev/cordoned"
	talosCordonedAnnotationValue = "true"
)

// Cordon marks a node as unschedulable.
func (h *Client) Cordon(name string) error {
	err := retry.Exponential(30*time.Second, retry.WithUnits(250*time.Millisecond), retry.WithJitter(50*time.Millisecond)).Retry(func() error {
		node, err := h.CoreV1().Nodes().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return retry.UnexpectedError(err)
		}

		if node.Spec.Unschedulable {
			return nil
		}

		node.Annotations[talosCordonedAnnotationName] = talosCordonedAnnotationValue
		node.Spec.Unschedulable = true

		if _, err := h.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{}); err != nil {
			return retry.ExpectedError(err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to cordon node %s: %w", name, err)
	}

	return nil
}

// Uncordon marks a node as schedulable.
//
// If force is set, node will be uncordoned even if cordoned not by Talos.
func (h *Client) Uncordon(name string, force bool) error {
	err := retry.Exponential(30*time.Second, retry.WithUnits(250*time.Millisecond), retry.WithJitter(50*time.Millisecond)).Retry(func() error {
		attemptCtx, attemptCtxCancel := context.WithTimeout(context.TODO(), 10*time.Second)
		defer attemptCtxCancel()

		node, err := h.CoreV1().Nodes().Get(attemptCtx, name, metav1.GetOptions{})
		if err != nil {
			return retry.UnexpectedError(err)
		}

		if !force && node.Annotations[talosCordonedAnnotationName] != talosCordonedAnnotationValue {
			// not cordoned by Talos, skip it
			return nil
		}

		if node.Spec.Unschedulable {
			node.Spec.Unschedulable = false
			delete(node.Annotations, talosCordonedAnnotationName)

			if _, err := h.CoreV1().Nodes().Update(attemptCtx, node, metav1.UpdateOptions{}); err != nil {
				return retry.ExpectedError(err)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to uncordon node %s: %w", name, err)
	}

	return nil
}

// Drain evicts all pods on a given node.
func (h *Client) Drain(node string) error {
	opts := metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node}).String(),
	}

	pods, err := h.CoreV1().Pods(metav1.NamespaceAll).List(context.TODO(), opts)
	if err != nil {
		return fmt.Errorf("cannot get pods for node %s: %w", node, err)
	}

	var wg sync.WaitGroup

	wg.Add(len(pods.Items))

	// Evict each pod.

	for _, pod := range pods.Items {
		go func(p corev1.Pod) {
			defer wg.Done()

			for _, ref := range p.ObjectMeta.OwnerReferences {
				if ref.Kind == "DaemonSet" {
					log.Printf("skipping DaemonSet pod %s\n", p.GetName())
					return
				}
			}

			if err := h.evict(p, int64(60)); err != nil {
				log.Printf("WARNING: failed to evict pod: %v", err)
			}
		}(pod)
	}

	wg.Wait()

	return nil
}

func (h *Client) evict(p corev1.Pod, gracePeriod int64) error {
	for {
		pol := &policy.Eviction{
			ObjectMeta:    metav1.ObjectMeta{Namespace: p.GetNamespace(), Name: p.GetName()},
			DeleteOptions: &metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod},
		}
		err := h.CoreV1().Pods(p.GetNamespace()).Evict(context.TODO(), pol)

		switch {
		case apierrors.IsTooManyRequests(err):
			time.Sleep(5 * time.Second)
		case apierrors.IsNotFound(err):
			return nil
		case err != nil:
			return fmt.Errorf("failed to evict pod %s/%s: %w", p.GetNamespace(), p.GetName(), err)
		default:
			if err = h.waitForPodDeleted(&p); err != nil {
				return fmt.Errorf("failed waiting on pod %s/%s to be deleted: %w", p.GetNamespace(), p.GetName(), err)
			}
		}
	}
}

func (h *Client) waitForPodDeleted(p *corev1.Pod) error {
	return retry.Constant(time.Minute, retry.WithUnits(3*time.Second)).Retry(func() error {
		pod, err := h.CoreV1().Pods(p.GetNamespace()).Get(context.TODO(), p.GetName(), metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			return nil
		case err != nil:
			return retry.UnexpectedError(fmt.Errorf("failed to get pod %s/%s: %w", p.GetNamespace(), p.GetName(), err))
		}

		if pod.GetUID() != p.GetUID() {
			return nil
		}

		return retry.ExpectedError(errors.New("pod is still running on the node"))
	})
}
