// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeconfig

import (
	"bytes"
	stdlibx509 "crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"text/template"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

const kubeConfigTemplate = `apiVersion: v1
kind: Config
clusters:
- name: {{ .ClusterName }}
  cluster:
    server: {{ .Endpoint }}
    certificate-authority-data: {{ .CACert | base64Encode }}
users:
- name: {{ .Username }}@{{ .ClusterName }}
  user:
    client-certificate-data: {{ .ClientCert | base64Encode }}
    client-key-data: {{ .ClientKey | base64Encode }}
contexts:
- context:
    cluster: {{ .ClusterName }}
    namespace: default
    user: {{ .Username }}@{{ .ClusterName }}
  name: {{ .ContextName }}@{{ .ClusterName }}
current-context: {{ .ContextName }}@{{ .ClusterName }}
`

// GenerateAdminInput is the interface for the GenerateAdmin function.
//
// This interface is implemented by config.Cluster().
type GenerateAdminInput interface {
	Name() string
	Endpoint() *url.URL
	IssuingCA() *x509.PEMEncodedCertificateAndKey
	AcceptedCAs() []*x509.PEMEncodedCertificate
	AdminKubeconfig() config.AdminKubeconfig
}

// GenerateAdmin generates admin kubeconfig for the cluster.
func GenerateAdmin(config GenerateAdminInput, out io.Writer) error {
	acceptedCAs := config.AcceptedCAs()

	if config.IssuingCA() != nil {
		acceptedCAs = append(acceptedCAs, &x509.PEMEncodedCertificate{Crt: config.IssuingCA().Crt})
	}

	return Generate(
		&GenerateInput{
			ClusterName:         config.Name(),
			IssuingCA:           config.IssuingCA(),
			AcceptedCAs:         acceptedCAs,
			CertificateLifetime: config.AdminKubeconfig().CertLifetime(),

			CommonName:   config.AdminKubeconfig().CommonName(),
			Organization: config.AdminKubeconfig().CertOrganization(),

			Endpoint:    config.Endpoint().String(),
			Username:    "admin",
			ContextName: "admin",
		},
		out,
	)
}

// GenerateInput are input parameters for Generate.
type GenerateInput struct {
	ClusterName string

	IssuingCA           *x509.PEMEncodedCertificateAndKey
	AcceptedCAs         []*x509.PEMEncodedCertificate
	CertificateLifetime time.Duration

	CommonName   string
	Organization string

	Endpoint    string
	Username    string
	ContextName string
}

const allowedTimeSkew = 10 * time.Second

// Generate a kubeconfig for the cluster from the given Input.
func Generate(in *GenerateInput, out io.Writer) error {
	tpl, err := template.New("kubeconfig").Funcs(template.FuncMap{
		"base64Encode": base64Encode,
	}).Parse(kubeConfigTemplate)
	if err != nil {
		return fmt.Errorf("error parsing kubeconfig template: %w", err)
	}

	k8sCA, err := x509.NewCertificateAuthorityFromCertificateAndKey(in.IssuingCA)
	if err != nil {
		return fmt.Errorf("error getting Kubernetes CA: %w", err)
	}

	clientCert, err := x509.NewKeyPair(k8sCA,
		x509.CommonName(in.CommonName),
		x509.Organization(in.Organization),
		x509.NotBefore(time.Now().Add(-allowedTimeSkew)),
		x509.NotAfter(time.Now().Add(in.CertificateLifetime)),
		x509.KeyUsage(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment),
		x509.ExtKeyUsage([]stdlibx509.ExtKeyUsage{
			stdlibx509.ExtKeyUsageClientAuth,
		}),
	)
	if err != nil {
		return fmt.Errorf("error generating Kubernetes client certificate: %w", err)
	}

	clientCertPEM := x509.NewCertificateAndKeyFromKeyPair(clientCert)

	serverCAs := bytes.Join(xslices.Map(in.AcceptedCAs, func(ca *x509.PEMEncodedCertificate) []byte { return ca.Crt }), nil)

	return tpl.Execute(out, struct {
		GenerateInput

		CACert     string
		ClientCert string
		ClientKey  string
	}{
		GenerateInput: *in,
		CACert:        string(serverCAs),
		ClientCert:    string(clientCertPEM.Crt),
		ClientKey:     string(clientCertPEM.Key),
	})
}

func base64Encode(content interface{}) (string, error) {
	str, ok := content.(string)
	if !ok {
		return "", fmt.Errorf("argument to base64 encode is not a string: %v", content)
	}

	return base64.StdEncoding.EncodeToString([]byte(str)), nil
}
