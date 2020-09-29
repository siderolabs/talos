// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeconfig

import (
	"encoding/base64"
	"fmt"
	"io"
	"text/template"
	"time"

	"github.com/talos-systems/crypto/x509"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const adminKubeConfigTemplate = `apiVersion: v1
kind: Config
clusters:
- name: {{ .Cluster }}
  cluster:
    server: {{ .Server }}
    certificate-authority-data: {{ .CACert }}
users:
- name: admin@{{ .Cluster }}
  user:
    client-certificate-data: {{ .AdminCert }}
    client-key-data: {{ .AdminKey }}
contexts:
- context:
    cluster: {{ .Cluster }}
    namespace: default
    user: admin@{{ .Cluster }}
  name: admin@{{ .Cluster }}
current-context: admin@{{ .Cluster }}
`

// GenerateAdmin generates admin kubeconfig for the cluster.
func GenerateAdmin(config config.ClusterConfig, out io.Writer) error {
	tpl, err := template.New("kubeconfig").Parse(adminKubeConfigTemplate)
	if err != nil {
		return fmt.Errorf("error parsing kubeconfig template: %w", err)
	}

	k8sCA, err := config.CA().GetCert()
	if err != nil {
		return fmt.Errorf("error getting Kubernetes CA certificate: %w", err)
	}

	k8sKey, err := config.CA().GetRSAKey()
	if err != nil {
		return fmt.Errorf("error parseing Kubernetes key: %w", err)
	}

	adminCert, err := x509.NewCertficateAndKey(k8sCA, k8sKey,
		x509.RSA(true),
		x509.CommonName(constants.KubernetesAdminCertCommonName),
		x509.Organization(constants.KubernetesAdminCertOrganization),
		x509.NotAfter(time.Now().Add(config.AdminKubeconfig().CertLifetime())))
	if err != nil {
		return fmt.Errorf("error generating admin certificate: %w", err)
	}

	input := struct {
		Cluster   string
		CACert    string
		AdminCert string
		AdminKey  string
		Server    string
	}{
		Cluster:   config.Name(),
		CACert:    base64.StdEncoding.EncodeToString(config.CA().Crt),
		AdminCert: base64.StdEncoding.EncodeToString(adminCert.Crt),
		AdminKey:  base64.StdEncoding.EncodeToString(adminCert.Key),
		Server:    config.Endpoint().String(),
	}

	return tpl.Execute(out, input)
}
