/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kubernetes

import (
	stdlibx509 "crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
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

	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/retry"
)

// Helper represents a set of helper methods for interacting with the
// Kubernetes API.
type Helper struct {
	client *kubernetes.Clientset
}

// NewHelper initializes and returns a Helper.
func NewHelper() (helper *Helper, err error) {
	kubeconfig := "/etc/kubernetes/kubelet.conf"

	var config *restclient.Config

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	var clientset *kubernetes.Clientset

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Helper{clientset}, nil
}

// NewClientFromPKI initializes and returns a Helper.
func NewClientFromPKI(ca, crt, key []byte, host, port string) (helper *Helper, err error) {
	tlsClientConfig := restclient.TLSClientConfig{
		CAData:   ca,
		CertData: crt,
		KeyData:  key,
	}

	config := &restclient.Config{
		Host:            "https://" + net.JoinHostPort(host, port),
		TLSClientConfig: tlsClientConfig,
	}

	var clientset *kubernetes.Clientset

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Helper{clientset}, nil
}

// NewTemporaryClientFromPKI initializes a Kubernetes client using a certificate
// with a TTL of 10 minutes.
func NewTemporaryClientFromPKI(caCrt, caKey []byte, endpoint, port string) (helper *Helper, err error) {
	opts := []x509.Option{
		x509.RSA(true),
		x509.CommonName("admin"),
		x509.Organization("system:masters"),
		x509.NotAfter(time.Now().Add(10 * time.Minute)),
	}

	key, err := x509.NewRSAKey()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create RSA key")
	}

	keyBlock, _ := pem.Decode(key.KeyPEM)
	if keyBlock == nil {
		return nil, errors.New("failed to decode key")
	}

	keyRSA, err := stdlibx509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, "failled to parse private key")
	}

	csr, err := x509.NewCertificateSigningRequest(keyRSA, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create CSR")
	}

	crt, err := x509.NewCertificateFromCSRBytes(caCrt, caKey, csr.X509CertificateRequestPEM, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create certificate from CSR")
	}

	h, err := NewClientFromPKI(caCrt, crt.X509CertificatePEM, key.KeyPEM, endpoint, "6443")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client")
	}

	return h, nil
}

// MasterIPs cordons and drains a node in one call.
func (h *Helper) MasterIPs() (addrs []string, err error) {
	endpoints, err := h.client.CoreV1().Endpoints("default").Get("kubernetes", metav1.GetOptions{})
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

// LabelNodeAsMaster labels a node with the required master label.
func (h *Helper) LabelNodeAsMaster(name string) (err error) {
	n, err := h.client.CoreV1().Nodes().Get(name, metav1.GetOptions{})
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
		return errors.Wrapf(err, "failed to marshal unmodified node %q into JSON", n.Name)
	}

	n.Labels[constants.LabelNodeRoleMaster] = ""

	newData, err := json.Marshal(n)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal modified node %q into JSON", n.Name)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
	if err != nil {
		return errors.Wrap(err, "failed to create two way merge patch")
	}

	if _, err := h.client.CoreV1().Nodes().Patch(n.Name, types.StrategicMergePatchType, patchBytes); err != nil {
		if apierrors.IsConflict(err) {
			return errors.Wrap(err, "unable to update node metadata due to conflict")
		}

		return errors.Wrapf(err, "error patching node %q", n.Name)
	}

	return nil
}

// CordonAndDrain cordons and drains a node in one call.
func (h *Helper) CordonAndDrain(node string) (err error) {
	if err = h.Cordon(node); err != nil {
		return err
	}

	return h.Drain(node)
}

// Cordon marks a node as unschedulable.
func (h *Helper) Cordon(name string) error {
	node, err := h.client.CoreV1().Nodes().Get(name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get node %s", name)
	}

	if node.Spec.Unschedulable {
		return nil
	}

	node.Spec.Unschedulable = true

	if _, err := h.client.CoreV1().Nodes().Update(node); err != nil {
		return errors.Wrapf(err, "failed to cordon node %s", node.GetName())
	}

	return nil
}

// Uncordon marks a node as schedulable.
func (h *Helper) Uncordon(name string) error {
	node, err := h.client.CoreV1().Nodes().Get(name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to get node %s", name)
	}

	if node.Spec.Unschedulable {
		node.Spec.Unschedulable = false
		if _, err := h.client.CoreV1().Nodes().Update(node); err != nil {
			return errors.Wrapf(err, "failed to uncordon node %s", node.GetName())
		}
	}

	return nil
}

// Drain evicts all pods on a given node.
func (h *Helper) Drain(node string) error {
	opts := metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": node}).String(),
	}

	pods, err := h.client.CoreV1().Pods(metav1.NamespaceAll).List(opts)
	if err != nil {
		return errors.Wrapf(err, "cannot get pods for node %s", node)
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

func (h *Helper) evict(p corev1.Pod, gracePeriod int64) error {
	for {
		pol := &policy.Eviction{
			ObjectMeta:    metav1.ObjectMeta{Namespace: p.GetNamespace(), Name: p.GetName()},
			DeleteOptions: &metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod},
		}
		err := h.client.CoreV1().Pods(p.GetNamespace()).Evict(pol)

		switch {
		case apierrors.IsTooManyRequests(err):
			time.Sleep(5 * time.Second)
		case apierrors.IsNotFound(err):
			return nil
		case err != nil:
			return errors.Wrapf(err, "failed to evict pod %s/%s", p.GetNamespace(), p.GetName())
		default:
			if err = h.waitForPodDeleted(&p); err != nil {
				return errors.Wrapf(err, "failed waiting on pod %s/%s to be deleted", p.GetNamespace(), p.GetName())
			}
		}
	}
}

func (h *Helper) waitForPodDeleted(p *corev1.Pod) error {
	return retry.Constant(time.Minute, retry.WithUnits(3*time.Second)).Retry(func() error {
		pod, err := h.client.CoreV1().Pods(p.GetNamespace()).Get(p.GetName(), metav1.GetOptions{})
		switch {
		case apierrors.IsNotFound(err):
			return nil
		case err != nil:
			return retry.UnexpectedError(errors.Wrapf(err, "failed to get pod %s/%s", p.GetNamespace(), p.GetName()))
		}

		if pod.GetUID() != p.GetUID() {
			return nil
		}

		return retry.ExpectedError(errors.New("pod is still running on the node"))
	})
}
