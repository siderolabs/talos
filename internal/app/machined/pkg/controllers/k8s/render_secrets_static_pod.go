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

	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/os-runtime/pkg/controller"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/state"

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

// ManagedResources implements controller.Controller interface.
func (ctrl *RenderSecretsStaticPodController) ManagedResources() (resource.Namespace, resource.Type) {
	return k8s.ControlPlaneNamespaceName, k8s.SecretsStatusType
}

// Run implements controller.Controller interface.
//
//nolint: gocyclo
func (ctrl *RenderSecretsStaticPodController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	if err := r.UpdateDependencies([]controller.Dependency{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.KubernetesType,
			Kind:      controller.DependencyWeak,
		},
	}); err != nil {
		return fmt.Errorf("error setting up dependencies: %w", err)
	}

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

		secrets := secretsRes.(*secrets.Kubernetes).Secrets()

		serviceAccountKey, err := secrets.ServiceAccount.GetKey()
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
						getter:       func() *x509.PEMEncodedCertificateAndKey { return secrets.EtcdCA },
						certFilename: "etcd-client-ca.crt",
					},
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return secrets.EtcdPeer },
						certFilename: "etcd-client.crt",
						keyFilename:  "etcd-client.key",
					},
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return secrets.CA },
						certFilename: "ca.crt",
					},
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return secrets.APIServer },
						certFilename: "apiserver.crt",
						keyFilename:  "apiserver.key",
					},
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return secrets.APIServerKubeletClient },
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
						getter:       func() *x509.PEMEncodedCertificateAndKey { return secrets.AggregatorCA },
						certFilename: "aggregator-ca.crt",
					},
					{
						getter:       func() *x509.PEMEncodedCertificateAndKey { return secrets.FrontProxy },
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
						getter:       func() *x509.PEMEncodedCertificateAndKey { return secrets.CA },
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
						template: []byte("{{ .AdminKubeconfig }}"),
					},
				},
			},
			{
				name:      "kube-scheduler",
				directory: constants.KubernetesSchedulerSecretsDir,
				templates: []template{
					{
						filename: "kubeconfig",
						template: []byte("{{ .AdminKubeconfig }}"),
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

			for _, templ := range pod.templates {
				var t *stdlibtemplate.Template

				t, err = stdlibtemplate.New(templ.filename).Parse(string(templ.template))
				if err != nil {
					return fmt.Errorf("error parsing template %q: %w", templ.filename, err)
				}

				var buf bytes.Buffer

				if err = t.Execute(&buf, secrets); err != nil {
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

		if err = r.Update(ctx, k8s.NewSecretsStatus(k8s.ControlPlaneNamespaceName, k8s.StaticPodSecretsStaticPodID), func(r resource.Resource) error {
			r.(*k8s.SecretsStatus).Status().Ready = true
			r.(*k8s.SecretsStatus).Status().Version = secretsRes.Metadata().Version().String()

			return nil
		}); err != nil {
			return err
		}
	}
}
