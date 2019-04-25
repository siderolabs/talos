/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

import (
	"bytes"
	"errors"
	"text/template"
)

// CertStrings holds the string representation of a certificate and key.
type CertStrings struct {
	Crt string
	Key string
}

// Input holds info about certs, ips, and node type.
type Input struct {
	Type          string // Valid values are init, controlplane, or worker.
	Certs         *Certs
	MasterIPs     []string
	Index         int
	ClusterName   string
	ServiceDomain string
	PodNet        []string
	ServiceNet    []string
	Endpoints     string
	KubeadmTokens *KubeadmTokens
	TrustdInfo    *TrustdInfo
}

// Certs holds the base64 encoded keys and certificates.
type Certs struct {
	AdminCert string
	AdminKey  string
	OsCert    string
	OsKey     string
	K8sCert   string
	K8sKey    string
}

// KubeadmTokens holds the senesitve kubeadm data.
type KubeadmTokens struct {
	BootstrapToken string
	CertKey        string
}

// TrustdInfo holds the trustd credentials.
type TrustdInfo struct {
	Username string
	Password string
}

// Userdata will return the talos userdata for a given node type.
func Userdata(in *Input) (string, error) {
	templateData := ""
	switch udtype := in.Type; udtype {
	case "init":
		templateData = initTempl
	case "controlplane":
		templateData = controlPlaneTempl
	case "worker":
		templateData = workerTempl
	default:
		return "", errors.New("unable to determine userdata type to generate")
	}

	ud, err := renderTemplate(in, templateData)
	if err != nil {
		return "", err
	}
	return ud, nil
}

// renderTemplate will output a templated string.
func renderTemplate(in *Input, udTemplate string) (string, error) {
	templ := template.Must(template.New("udTemplate").Parse(udTemplate))
	var buf bytes.Buffer
	if err := templ.Execute(&buf, in); err != nil {
		return "", err
	}
	return buf.String(), nil
}
