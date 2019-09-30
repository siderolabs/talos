/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kubeadm

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	securityapi "github.com/talos-systems/talos/api/security"
	"github.com/talos-systems/talos/pkg/config"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/grpc/middleware/auth/basic"
)

const dirPerm os.FileMode = 0700
const certPerm os.FileMode = 0600
const keyPerm os.FileMode = 0400

// WritePKIFiles handles writing any user specified certs to disk
func WritePKIFiles(config config.Configurator) (err error) {
	certs := []struct {
		Cert     *x509.PEMEncodedCertificateAndKey
		CertPath string
		KeyPath  string
	}{
		{
			Cert:     config.Cluster().CA(),
			CertPath: constants.KubeadmCACert,
			KeyPath:  constants.KubeadmCAKey,
		},
	}

	for _, cert := range certs {
		if cert.Cert == nil {
			continue
		}

		if cert.Cert.Crt != nil {
			if err = os.MkdirAll(path.Dir(cert.CertPath), dirPerm); err != nil {
				return err
			}
			if err = ioutil.WriteFile(cert.CertPath, cert.Cert.Crt, certPerm); err != nil {
				return fmt.Errorf("write %s: %v", cert.CertPath, err)
			}
		}
		if cert.Cert.Key != nil {
			if err = os.MkdirAll(path.Dir(cert.KeyPath), dirPerm); err != nil {
				return err
			}
			if err = ioutil.WriteFile(cert.KeyPath, cert.Cert.Key, keyPerm); err != nil {
				return fmt.Errorf("write %s: %v", cert.KeyPath, err)
			}
		}
	}
	return err
}

// CreateSecurityClients handles instantiating a security API client connection
// to each trustd endpoint defined in the config.
func CreateSecurityClients(config config.Configurator) (clients []securityapi.SecurityClient, err error) {
	clients = []securityapi.SecurityClient{}

	creds := basic.NewTokenCredentials(config.Machine().Security().Token())

	// Create a trustd client for each endpoint to set up
	// a fan out approach to gathering the files
	var conn *grpc.ClientConn
	for _, endpoint := range config.Cluster().IPs() {
		conn, err = basic.NewConnection(endpoint, constants.TrustdPort, creds)
		if err != nil {
			return clients, err
		}
		clients = append(clients, securityapi.NewSecurityClient(conn))
	}
	return clients, nil
}

// Download handles the retrieval of files from a security API endpoint.
func Download(ctx context.Context, client securityapi.SecurityClient, file *securityapi.ReadFileRequest, content chan<- []byte) {
	select {
	case <-ctx.Done():
	case content <- download(ctx, client, file):
	}
}

func download(ctx context.Context, client securityapi.SecurityClient, file *securityapi.ReadFileRequest) []byte {
	var (
		resp            *securityapi.ReadFileResponse
		err             error
		attempt, snooze float64
	)
	maxWait := float64(64)

	for {
		ctxTimeout, ctxTimeoutCancel := context.WithTimeout(ctx, 2*time.Second)
		defer ctxTimeoutCancel()

		resp, err = client.ReadFile(ctxTimeout, file)
		if err == nil {
			// TODO add in checksum verification for resp.Data
			// when trustd supports providing a checksum
			break
		}

		// Context canceled, no need to do anything
		if status.Code(err) == codes.Canceled {
			break
		}

		// Error case
		log.Printf("failed to read file %s: %+v", file, err)

		// backoff
		snooze = math.Pow(2, attempt)
		if snooze > maxWait {
			snooze = maxWait
		}

		select {
		case <-ctx.Done():
			return []byte{}
		case <-time.After(time.Duration(snooze) * time.Second):
		}
	}

	// Handle case where context was canceled or request otherwise failed
	// and we dont have an actual response
	if resp == nil {
		return []byte{}
	}

	return resp.Data
}
