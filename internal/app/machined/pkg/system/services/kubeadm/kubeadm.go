/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kubeadm

import (
	"context"
	"errors"
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
	kubeadmv1beta2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"

	securityapi "github.com/talos-systems/talos/api/security"
	"github.com/talos-systems/talos/internal/pkg/cis"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/grpc/middleware/auth/basic"
	"github.com/talos-systems/talos/pkg/userdata"
)

const dirPerm os.FileMode = 0700
const certPerm os.FileMode = 0600
const keyPerm os.FileMode = 0400

func editFullInitConfig(data *userdata.UserData) (err error) {
	if data.Services.Kubeadm.InitConfiguration == nil {
		return errors.New("expected InitConfiguration")
	}
	err = editInitConfig(data)
	if err != nil {
		return err
	}

	if data.Services.Kubeadm.ClusterConfiguration == nil {
		return errors.New("expected ClusterConfiguration")
	}
	err = editClusterConfig(data)
	if err != nil {
		return err
	}

	return err
}

func editInitConfig(data *userdata.UserData) (err error) {
	if data.Services.Kubeadm.InitConfiguration == nil {
		return errors.New("expected InitConfiguration")
	}

	initConfiguration, ok := data.Services.Kubeadm.InitConfiguration.(*kubeadmv1beta2.InitConfiguration)
	if !ok {
		return errors.New("failed InitConfiguration assertion")
	}

	// Hardcodes specific kubeadm config parameters
	initConfiguration.NodeRegistration.CRISocket = constants.ContainerdAddress

	return nil
}

func editJoinConfig(data *userdata.UserData) (err error) {
	if data.Services.Kubeadm.JoinConfiguration == nil {
		return errors.New("expected JoinConfiguration")
	}
	joinConfiguration, ok := data.Services.Kubeadm.JoinConfiguration.(*kubeadmv1beta2.JoinConfiguration)
	if !ok {
		return errors.New("failed JoinConfiguration assertion")
	}

	joinConfiguration.NodeRegistration.CRISocket = constants.ContainerdAddress
	if err = cis.EnforceWorkerRequirements(joinConfiguration); err != nil {
		return err
	}
	return nil
}

func editClusterConfig(data *userdata.UserData) (err error) {
	if data.Services.Kubeadm.ClusterConfiguration == nil {
		return errors.New("expected ClusterConfiguration")
	}

	clusterConfiguration, ok := data.Services.Kubeadm.ClusterConfiguration.(*kubeadmv1beta2.ClusterConfiguration)
	if !ok {
		return errors.New("failed ClusterConfiguration assertion")
	}

	// Hardcodes specific kubeadm config parameters
	clusterConfiguration.KubernetesVersion = data.KubernetesVersion
	clusterConfiguration.UseHyperKubeImage = true

	// Apply CIS hardening recommendations; only generate encryption token only if we're the bootstrap node
	if err = cis.EnforceBootstrapMasterRequirements(clusterConfiguration); err != nil {
		return err
	}

	return nil
}

// WriteConfig writes out the kubeadm config
func WriteConfig(data *userdata.UserData) (err error) {
	var b []byte

	// Enforce configuration edits
	switch {
	case data.Services.Kubeadm.JoinConfiguration != nil:
		err = editJoinConfig(data)
	case data.Services.Kubeadm.InitConfiguration != nil:
		err = editFullInitConfig(data)
	default:
		return errors.New("unsupported kubeadm configuration")
	}

	if err != nil {
		return err
	}

	if data.Services.Kubeadm.IsControlPlane() {
		if err = cis.EnforceCommonMasterRequirements(data.Security.Kubernetes.AESCBCEncryptionSecret); err != nil {
			return err
		}
	}

	// Marshal up config string
	if _, err = data.Services.Kubeadm.MarshalYAML(); err != nil {
		return err
	}
	b = []byte(data.Services.Kubeadm.ConfigurationStr)

	// Write out marshaled config
	p := path.Dir(constants.KubeadmConfig)
	if err = os.MkdirAll(p, os.ModeDir); err != nil {
		return fmt.Errorf("create %s: %v", p, err)
	}

	if err = ioutil.WriteFile(constants.KubeadmConfig, b, 0400); err != nil {
		return fmt.Errorf("write %s: %v", constants.KubeadmConfig, err)
	}

	return nil
}

// WritePKIFiles handles writing any user specified certs to disk
func WritePKIFiles(data *userdata.UserData) (err error) {
	if data.Security.Kubernetes == nil {
		return fmt.Errorf("[%s] is required", "security.kubernetes")
	}

	certs := []struct {
		Cert     *x509.PEMEncodedCertificateAndKey
		CertPath string
		KeyPath  string
	}{
		{
			Cert:     data.Security.Kubernetes.CA,
			CertPath: constants.KubeadmCACert,
			KeyPath:  constants.KubeadmCAKey,
		},
		{
			Cert:     data.Security.Kubernetes.SA,
			CertPath: constants.KubeadmSACert,
			KeyPath:  constants.KubeadmSAKey,
		},
		{
			Cert:     data.Security.Kubernetes.FrontProxy,
			CertPath: constants.KubeadmFrontProxyCACert,
			KeyPath:  constants.KubeadmFrontProxyCAKey,
		},
		{
			Cert:     data.Security.Kubernetes.Etcd,
			CertPath: constants.KubeadmEtcdCACert,
			KeyPath:  constants.KubeadmEtcdCAKey,
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

// FileSet compares the list of required files to the ones
// already present on the node and returns the delta
func FileSet(files []string) []*securityapi.ReadFileRequest {
	fileRequests := []*securityapi.ReadFileRequest{}
	// Check to see if we already have the file locally
	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			fileRequests = append(fileRequests, &securityapi.ReadFileRequest{Path: file})
		}
	}

	return fileRequests
}

// CreateSecurityClients handles instantiating a security API client connection
// to each trustd endpoint defined in userdata
func CreateSecurityClients(data *userdata.UserData) (clients []securityapi.SecurityClient, err error) {
	clients = []securityapi.SecurityClient{}

	creds := basic.NewTokenCredentials(data.Services.Trustd.Token)

	// Create a trustd client for each endpoint to set up
	// a fan out approach to gathering the files
	var conn *grpc.ClientConn
	for _, endpoint := range data.Services.Trustd.Endpoints {
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
