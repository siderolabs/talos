// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kubernetes implements safe Talos API PKI rotation for the cluster.
package kubernetes

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-retry/retry"
	"go.yaml.in/yaml/v4"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/pkg/cluster"
	taloskubernetes "github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
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
//   - remove old CA
//   - verify connectivity with the new PKI.
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

	if err := r.dropOldCA(ctx); err != nil {
		return err
	}

	if err := r.verifyConnectivity(ctx, r.newKubernetes, "new PKI"); err != nil {
		return err
	}

	return nil
}

func (r *rotator) printIntro() {
	r.opts.Printf("> Starting Kubernetes API PKI rotation, dry-run mode %v...\n", r.opts.DryRun)

	r.opts.Printf("> Cluster topology:\n")

	r.opts.Printf("  - control plane nodes: %q\n",
		append(
			helpers.MapToInternalIP(r.opts.ClusterInfo.NodesByType(machine.TypeInit)),
			helpers.MapToInternalIP(r.opts.ClusterInfo.NodesByType(machine.TypeControlPlane))...,
		),
	)
	r.opts.Printf("  - worker nodes: %q\n",
		helpers.MapToInternalIP(r.opts.ClusterInfo.NodesByType(machine.TypeWorker)),
	)
}

func (r *rotator) fetchClient(ctx context.Context, clientPtr **cluster.KubernetesClient, label string) error {
	r.opts.Printf("> Building %s Kubernetes client...\n", label)

	firstNode := append(
		r.opts.ClusterInfo.NodesByType(machine.TypeInit),
		r.opts.ClusterInfo.NodesByType(machine.TypeControlPlane)...,
	)[0]

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

	firstNode := append(
		r.opts.ClusterInfo.NodesByType(machine.TypeInit),
		r.opts.ClusterInfo.NodesByType(machine.TypeControlPlane)...,
	)[0]

	k8sRoot, err := safe.StateGetByID[*secretsres.KubernetesRoot](client.WithNode(ctx, firstNode.InternalIP.String()), r.opts.TalosClient.COSI, secretsres.KubernetesRootID)
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
					if cond.Type == v1.NodeReady {
						if cond.Status != v1.ConditionTrue {
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

func (r *rotator) addNewCAAccepted(ctx context.Context) error {
	r.opts.Printf("> Adding new Kubernetes CA as accepted...\n")

	if err := r.patchAllNodes(ctx,
		func(_ machine.Type, config *v1alpha1.Config) error {
			config.ClusterConfig.ClusterAcceptedCAs = append(
				config.ClusterConfig.ClusterAcceptedCAs,
				&x509.PEMEncodedCertificate{
					Crt: r.opts.NewKubernetesCA.Crt,
				},
			)

			return nil
		}); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	return nil
}

func (r *rotator) swapCAs(ctx context.Context) error {
	r.opts.Printf("> Making new Kubernetes CA the issuing CA, old Kubernetes CA the accepted CA...\n")

	if err := r.patchAllNodes(ctx,
		func(machineType machine.Type, config *v1alpha1.Config) error {
			config.ClusterConfig.ClusterAcceptedCAs = append(
				config.ClusterConfig.ClusterAcceptedCAs,
				&x509.PEMEncodedCertificate{
					Crt: r.currentCA,
				},
			)
			config.ClusterConfig.ClusterAcceptedCAs = slices.DeleteFunc(config.Cluster().AcceptedCAs(), func(ca *x509.PEMEncodedCertificate) bool {
				return bytes.Equal(ca.Crt, r.opts.NewKubernetesCA.Crt)
			})

			if machineType.IsControlPlane() {
				config.ClusterConfig.ClusterCA = r.opts.NewKubernetesCA
			} else {
				config.ClusterConfig.ClusterCA = &x509.PEMEncodedCertificateAndKey{
					Crt: r.opts.NewKubernetesCA.Crt,
				}
			}

			return nil
		}); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	return nil
}

func (r *rotator) dropOldCA(ctx context.Context) error {
	r.opts.Printf("> Removing old Kubernetes CA from the accepted CAs...\n")

	if err := r.patchAllNodes(ctx,
		func(_ machine.Type, config *v1alpha1.Config) error {
			config.ClusterConfig.ClusterAcceptedCAs = slices.DeleteFunc(config.Cluster().AcceptedCAs(), func(ca *x509.PEMEncodedCertificate) bool {
				return bytes.Equal(ca.Crt, r.currentCA)
			})

			return nil
		}); err != nil {
		return fmt.Errorf("error patching all machine configs: %w", err)
	}

	return nil
}

func (r *rotator) patchAllNodes(ctx context.Context, patchFunc func(machineType machine.Type, config *v1alpha1.Config) error) error {
	for _, machineType := range []machine.Type{machine.TypeInit, machine.TypeControlPlane, machine.TypeWorker} {
		for _, node := range r.opts.ClusterInfo.NodesByType(machineType) {
			if r.opts.DryRun {
				r.opts.Printf("  - %s: skipped (dry-run)\n", node.InternalIP)

				continue
			}

			if err := helpers.PatchNodeConfigWithKubeletRestart(ctx, r.opts.TalosClient, node.InternalIP.String(), r.opts.EncoderOption, func(config *v1alpha1.Config) error {
				return patchFunc(machineType, config)
			}); err != nil {
				return fmt.Errorf("error patching node %s: %w", node.InternalIP, err)
			}

			r.opts.Printf("  - %s: OK\n", node.InternalIP)
		}
	}

	return nil
}
