/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/blang/semver"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"golang.org/x/xerrors"
	yaml "gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/pkg/crypto/x509"
)

// UserData represents the user data.
type UserData struct {
	Version           Version     `yaml:"version"`
	Security          *Security   `yaml:"security"`
	Networking        *Networking `yaml:"networking"`
	Services          *Services   `yaml:"services"`
	Files             []*File     `yaml:"files"`
	Debug             bool        `yaml:"debug"`
	Env               Env         `yaml:"env,omitempty"`
	Install           *Install    `yaml:"install,omitempty"`
	KubernetesVersion string      `yaml:"kubernetesVersion,omitempty"`
}

// Validate ensures the required fields are present in the userdata
// nolint: gocyclo
func (data *UserData) Validate() error {
	var result *multierror.Error

	if _, err := semver.Parse(data.KubernetesVersion); err != nil {
		result = multierror.Append(result, errors.Wrap(err, "version must be semantic"))
	}

	// All nodeType checks
	if data.Services != nil {
		result = multierror.Append(result, data.Services.Validate(CheckServices()))
		if data.Services.Trustd != nil {
			result = multierror.Append(result, data.Services.Trustd.Validate(CheckTrustdAuth(), CheckTrustdEndpointsAreValidIPsOrHostnames()))
		}
		if data.Services.Init != nil {
			result = multierror.Append(result, data.Services.Init.Validate(CheckInitCNI()))
		}
		if data.Services.Kubeadm != nil {
			switch {
			case data.Services.Kubeadm.IsBootstrap():
				result = multierror.Append(result, data.Security.OS.Validate(CheckOSCA()))
				result = multierror.Append(result, data.Security.Kubernetes.Validate(CheckKubernetesCA()))
			case data.Services.Kubeadm.IsControlPlane():
				result = multierror.Append(result, data.Services.Trustd.Validate(CheckTrustdEndpointsArePresent()))
			case data.Services.Kubeadm.IsWorker():
				result = multierror.Append(result, data.Services.Trustd.Validate(CheckTrustdEndpointsArePresent()))
			}
		}
	}

	// Surely there's a better way to do this
	if data.Networking != nil && data.Networking.OS != nil {
		for _, dev := range data.Networking.OS.Devices {
			result = multierror.Append(result, dev.Validate(CheckDeviceInterface(), CheckDeviceAddressing(), CheckDeviceRoutes()))
		}
	}

	return result.ErrorOrNil()
}

// Security represents the set of options available to configure security.
type Security struct {
	OS         *OSSecurity         `yaml:"os"`
	Kubernetes *KubernetesSecurity `yaml:"kubernetes"`
}

// Networking represents the set of options available to configure networking.
type Networking struct {
	Kubernetes struct{} `yaml:"kubernetes"`
	OS         *OSNet   `yaml:"os"`
}

// OSNet represents the network interfaces present on the host
type OSNet struct {
	Devices    []Device `yaml:"devices"`
	Hostname   string   `yaml:"hostname"`
	Domainname string   `yaml:"domainname"`
}

// File represents a file to write to disk.
type File struct {
	Contents    string      `yaml:"contents"`
	Permissions os.FileMode `yaml:"permissions"`
	Path        string      `yaml:"path"`
}

// Open is a convenience function that reads the user data from disk, and
// unmarshals it.
func Open(p string) (data *UserData, err error) {
	fileBytes, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read user data: %v", err)
	}

	data = &UserData{}
	if err = yaml.Unmarshal(fileBytes, data); err != nil {
		return nil, fmt.Errorf("unmarshal user data: %v", err)
	}

	return data, nil
}

type certTest struct {
	Cert     *x509.PEMEncodedCertificateAndKey
	Path     string
	Required bool
}

// nolint: gocyclo
func checkCertKeyPair(certs []certTest) error {
	var result *multierror.Error
	for _, cert := range certs {
		// Verify the required sections are present
		if cert.Required && cert.Cert == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", cert.Path, "", ErrRequiredSection))
		}

		// Bail early since we're already missing the required sections
		if result.ErrorOrNil() != nil {
			continue
		}

		// If it isn't required, there is a chance that it is nil.
		if cert.Cert == nil {
			continue
		}

		if cert.Cert.Crt == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", cert.Path+".crt", "", ErrRequiredSection))
		}

		if cert.Cert.Key == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", cert.Path+".key", "", ErrRequiredSection))
		}

		// test if CA fields are present ( x509 package handles the b64 decode
		// during yaml unmarshal, so we have the bytes if it was successful )
		var block *pem.Block
		block, _ = pem.Decode(cert.Cert.Crt)
		// nolint: gocritic
		if block == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", cert.Path+".crt", cert.Cert.Crt, ErrInvalidCert))
		} else {
			if block.Type != "CERTIFICATE" {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", cert.Path+".crt", cert.Cert.Crt, ErrInvalidCertType))
			}
		}

		block, _ = pem.Decode(cert.Cert.Key)
		// nolint: gocritic
		if block == nil {
			result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", cert.Path+".key", cert.Cert.Key, ErrInvalidCert))
		} else {
			if !strings.HasSuffix(block.Type, "PRIVATE KEY") {
				result = multierror.Append(result, xerrors.Errorf("[%s] %q: %w", cert.Path+".key", cert.Cert.Key, ErrInvalidCertType))
			}
		}
	}

	return result.ErrorOrNil()
}
