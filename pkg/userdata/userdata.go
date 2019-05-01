/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	stdlibx509 "crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	stdlibnet "net"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/talos-systems/talos/internal/pkg/net"
	"github.com/talos-systems/talos/pkg/crypto/x509"

	yaml "gopkg.in/yaml.v2"
)

// UserData represents the user data.
type UserData struct {
	Version    Version     `yaml:"version"`
	Security   *Security   `yaml:"security"`
	Networking *Networking `yaml:"networking"`
	Services   *Services   `yaml:"services"`
	Files      []*File     `yaml:"files"`
	Debug      bool        `yaml:"debug"`
	Env        Env         `yaml:"env,omitempty"`
	Install    *Install    `yaml:"install,omitempty"`
}

/*
type WorkerData UserData

// Validate ensures the necessary configuration for a
// worker node is present
func (w *WorkerData) Validate() error {
	var result *multierror.Error
	result = multierror.Append(result, w.Version.Validate())
	result = multierror.Append(result, w.Services.Init.Validate())
	//result = multierror.Append(result, w.Services.Kubeadm.Validate())
	result = multierror.Append(result, w.Services.Trustd.Validate())
	return result
}

type InitData UserData

func (i *InitData) Validate() error {
	var result *multierror.Error
	result = multierror.Append(result, i.Version.Validate())
	result = multierror.Append(result, i.Security.OS.Validate())
	//result = multierror.Append(result, i.Security.Kubernetes.Validate())
	result = multierror.Append(result, i.Services.Init.Validate())
	//result = multierror.Append(result, i.Services.Kubeadm.Validate())
	result = multierror.Append(result, i.Services.Trustd.Validate())
	return result

}

type MasterData UserData

func (m *MasterData) Validate() error {
	var result *multierror.Error
	result = multierror.Append(result, m.Version.Validate())
	result = multierror.Append(result, m.Services.Init.Validate())
	//result = multierror.Append(result, m.Services.Kubeadm.Validate())
	result = multierror.Append(result, m.Services.Trustd.Validate())
	return result
}
*/

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
	Devices []Device `yaml:"devices"`
}

// File represents a file to write to disk.
type File struct {
	Contents    string      `yaml:"contents"`
	Permissions os.FileMode `yaml:"permissions"`
	Path        string      `yaml:"path"`
}

// WriteFiles writes the requested files to disk.
func (data *UserData) WriteFiles() (err error) {
	for _, f := range data.Files {
		p := path.Join("/var", f.Path)
		if err = os.MkdirAll(path.Dir(p), os.ModeDir); err != nil {
			return
		}
		if err = ioutil.WriteFile(p, []byte(f.Contents), f.Permissions); err != nil {
			return
		}
	}

	return nil
}

// IsBootstrap indicates if the current kubeadm configuration is a master init
// configuration.
func (data *UserData) IsBootstrap() bool {
	return data.Services.Kubeadm.bootstrap
}

// IsControlPlane indicates if the current kubeadm configuration is a worker
// acting as a master.
func (data *UserData) IsControlPlane() bool {
	return data.Services.Kubeadm.controlPlane
}

// IsMaster indicates if the current kubeadm configuration is a master
// configuration.
func (data *UserData) IsMaster() bool {
	return data.Services.Kubeadm.bootstrap || data.Services.Kubeadm.controlPlane
}

// IsWorker indicates if the current kubeadm configuration is a worker
// configuration.
func (data *UserData) IsWorker() bool {
	return !data.IsMaster()
}

// NewIdentityCSR creates a new CSR for the node's identity certificate.
func (data *UserData) NewIdentityCSR() (csr *x509.CertificateSigningRequest, err error) {
	var key *x509.Key
	key, err = x509.NewKey()
	if err != nil {
		return nil, err
	}

	data.Security.OS.Identity = &x509.PEMEncodedCertificateAndKey{}
	data.Security.OS.Identity.Key = key.KeyPEM

	pemBlock, _ := pem.Decode(key.KeyPEM)
	if pemBlock == nil {
		return nil, fmt.Errorf("failed to decode key")
	}
	keyEC, err := stdlibx509.ParseECPrivateKey(pemBlock.Bytes)
	if err != nil {
		return nil, err
	}
	ips, err := net.IPAddrs()
	if err != nil {
		return nil, err
	}
	for _, san := range data.Services.Trustd.CertSANs {
		if ip := stdlibnet.ParseIP(san); ip != nil {
			ips = append(ips, ip)
		}
	}
	hostname, err := os.Hostname()
	if err != nil {
		return
	}
	opts := []x509.Option{}
	names := []string{hostname}
	opts = append(opts, x509.DNSNames(names))
	opts = append(opts, x509.IPAddresses(ips))
	opts = append(opts, x509.NotAfter(time.Now().Add(time.Duration(8760)*time.Hour)))
	csr, err = x509.NewCertificateSigningRequest(keyEC, opts...)
	if err != nil {
		return nil, err
	}

	return csr, nil
}

// Download initializes a UserData struct from a remote URL.
// nolint: gocyclo
func Download(url string, headers *map[string]string) (data *UserData, err error) {
	// TODO(andrewrynhard): Implement functional options.
	maxRetries := 10
	maxWait := float64(64)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return data, err
	}

	if headers != nil {
		for k, v := range *headers {
			req.Header.Set(k, v)
		}
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := client.Do(req)
		if err != nil {
			return data, err
		}
		// nolint: errcheck
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Received %d\n", resp.StatusCode)
			snooze := math.Pow(2, float64(attempt))
			if snooze > maxWait {
				snooze = maxWait
			}
			time.Sleep(time.Duration(snooze) * time.Second)
			continue
		}

		dataBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return data, fmt.Errorf("read user data: %s", err.Error())
		}

		data = &UserData{}
		if err := yaml.Unmarshal(dataBytes, data); err != nil {
			return data, fmt.Errorf("unmarshal user data: %s", err.Error())
		}

		return data, nil
	}
	return data, fmt.Errorf("failed to download userdata from: %s", url)
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
