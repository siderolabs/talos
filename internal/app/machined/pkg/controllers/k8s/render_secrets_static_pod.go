// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	stdlibtemplate "text/template"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/k8s"
	"github.com/talos-systems/talos/pkg/resources/secrets"
)

// RenderSecretsStaticPodController manages k8s.SecretsReady and renders secrets from secrets.Kubernetes.
type RenderSecretsStaticPodController struct{}

// Name implements controller.Controller interface.
func (ctrl *RenderSecretsStaticPodController) Name() string {
	return "k8s.RenderSecretsStaticPodController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RenderSecretsStaticPodController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.RootType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesType,
			ID:        pointer.ToString(secrets.KubernetesID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.EtcdType,
			ID:        pointer.ToString(secrets.EtcdID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RenderSecretsStaticPodController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.SecretsStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *RenderSecretsStaticPodController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		secretsRes, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.KubernetesType, secrets.KubernetesID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting secrets resource: %w", err)
		}

		etcdRes, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.EtcdType, secrets.EtcdID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting secrets resource: %w", err)
		}

		rootEtcdRes, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.RootType, secrets.RootEtcdID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting secrets resource: %w", err)
		}

		rootK8sRes, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.RootType, secrets.RootKubernetesID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting secrets resource: %w", err)
		}

		rootEtcdSecrets := rootEtcdRes.(*secrets.Root).EtcdSpec()
		rootK8sSecrets := rootK8sRes.(*secrets.Root).KubernetesSpec()
		etcdSecrets := etcdRes.(*secrets.Etcd).Certs()
		k8sSecrets := secretsRes.(*secrets.Kubernetes).Certs()

		serviceAccountKey, err := rootK8sSecrets.ServiceAccount.GetKey()
		if err != nil {
			return fmt.Errorf("error parsing service account key: %w", err)
		}

		type secret struct {
			getter       func() *x509.PEMEncodedCertificateAndKey
			certFilename string
			keyFilename  string
		}

		type template struct {
			filename string
			template []byte
		}

		for _, pod := range []struct {
			name      string
			directory string
			secrets   []secret
			templates []template
		}{
			{
				name:      "kube-apiserver",
				directory: constants.KubernetesAPIServerSecretsDir,
				secrets: []secret{
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return rootEtcdSecrets.EtcdCA },
						certFilename: "etcd-client-ca.crt",
					},
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return etcdSecrets.EtcdPeer },
						certFilename: "etcd-client.crt",
						keyFilename:  "etcd-client.key",
					},
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return rootK8sSecrets.CA },
						certFilename: "ca.crt",
					},
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return k8sSecrets.APIServer },
						certFilename: "apiserver.crt",
						keyFilename:  "apiserver.key",
					},
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return k8sSecrets.APIServerKubeletClient },
						certFilename: "apiserver-kubelet-client.crt",
						keyFilename:  "apiserver-kubelet-client.key",
					},
					{
						getter: func() *x509.PEMEncodedCertificateAndKey {
							return &x509.PEMEncodedCertificateAndKey{
								Crt: serviceAccountKey.GetPublicKeyPEM(),
								Key: serviceAccountKey.GetPrivateKeyPEM(),
							}
						},
						certFilename: "service-account.pub",
						keyFilename:  "service-account.key",
					},
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return rootK8sSecrets.AggregatorCA },
						certFilename: "aggregator-ca.crt",
					},
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return k8sSecrets.FrontProxy },
						certFilename: "front-proxy-client.crt",
						keyFilename:  "front-proxy-client.key",
					},
				},
				templates: []template{
					{
						filename: "encryptionconfig.yaml",
						template: kubeSystemEncryptionConfigTemplate,
					},
					{
						filename: "auditpolicy.yaml",
						template: kubeSystemAuditPolicyTemplate,
					},
				},
			},
			{
				name:      "kube-controller-manager",
				directory: constants.KubernetesControllerManagerSecretsDir,
				secrets: []secret{
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return rootK8sSecrets.CA },
						certFilename: "ca.crt",
						keyFilename:  "ca.key",
					},
					{
						getter: func() *x509.PEMEncodedCertificateAndKey {
							return &x509.PEMEncodedCertificateAndKey{
								Crt: serviceAccountKey.GetPublicKeyPEM(),
								Key: serviceAccountKey.GetPrivateKeyPEM(),
							}
						},
						keyFilename: "service-account.key",
					},
				},
				templates: []template{
					{
						filename: "kubeconfig",
						template: []byte("{{ .Secrets.AdminKubeconfig }}"),
					},
				},
			},
			{
				name:      "kube-scheduler",
				directory: constants.KubernetesSchedulerSecretsDir,
				templates: []template{
					{
						filename: "kubeconfig",
						template: []byte("{{ .Secrets.AdminKubeconfig }}"),
					},
				},
			},
		} {
			if err = os.MkdirAll(pod.directory, 0o755); err != nil {
				return fmt.Errorf("error creating secrets directory for %q: %w", pod.name, err)
			}

			for _, secret := range pod.secrets {
				certAndKey := secret.getter()

				if secret.certFilename != "" {
					if err = ioutil.WriteFile(filepath.Join(pod.directory, secret.certFilename), certAndKey.Crt, 0o400); err != nil {
						return fmt.Errorf("error writing certificate %q for %q: %w", secret.certFilename, pod.name, err)
					}

					if err = os.Chown(filepath.Join(pod.directory, secret.certFilename), constants.KubernetesRunUser, -1); err != nil {
						return fmt.Errorf("error chowning %q for %q: %w", secret.certFilename, pod.name, err)
					}
				}

				if secret.keyFilename != "" {
					if err = ioutil.WriteFile(filepath.Join(pod.directory, secret.keyFilename), certAndKey.Key, 0o400); err != nil {
						return fmt.Errorf("error writing key %q for %q: %w", secret.keyFilename, pod.name, err)
					}

					if err = os.Chown(filepath.Join(pod.directory, secret.keyFilename), constants.KubernetesRunUser, -1); err != nil {
						return fmt.Errorf("error chowning %q for %q: %w", secret.keyFilename, pod.name, err)
					}
				}
			}

			type templateParams struct {
				Root    *secrets.RootKubernetesSpec
				Secrets *secrets.KubernetesCertsSpec
			}

			params := templateParams{
				Root:    rootK8sSecrets,
				Secrets: k8sSecrets,
			}

			for _, templ := range pod.templates {
				var t *stdlibtemplate.Template

				t, err = stdlibtemplate.New(templ.filename).Parse(string(templ.template))
				if err != nil {
					return fmt.Errorf("error parsing template %q: %w", templ.filename, err)
				}

				var buf bytes.Buffer

				if err = t.Execute(&buf, &params); err != nil {
					return fmt.Errorf("error executing template %q: %w", templ.filename, err)
				}

				if err = ioutil.WriteFile(filepath.Join(pod.directory, templ.filename), buf.Bytes(), 0o400); err != nil {
					return fmt.Errorf("error writing template %q for %q: %w", templ.filename, pod.name, err)
				}

				if err = os.Chown(filepath.Join(pod.directory, templ.filename), constants.KubernetesRunUser, -1); err != nil {
					return fmt.Errorf("error chowning %q for %q: %w", templ.filename, pod.name, err)
				}
			}
		}

		if err = r.Modify(ctx, k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID), func(r resource.Resource) error {
			r.(*k8s.SecretsStatus).Status().Ready = true
			r.(*k8s.SecretsStatus).Status().Version = secretsRes.Metadata().Version().String()

			return nil
		}); err != nil {
			return err
		}
	}
}
