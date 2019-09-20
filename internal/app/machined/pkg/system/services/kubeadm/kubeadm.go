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
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	kubeadmv1beta2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"

	securityapi "github.com/talos-systems/talos/api/security"
	"github.com/talos-systems/talos/internal/pkg/cis"
	"github.com/talos-systems/talos/pkg/cmd"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/grpc/middleware/auth/basic"
	"github.com/talos-systems/talos/pkg/userdata"
)

const dirPerm os.FileMode = 0700
const certPerm os.FileMode = 0600
const keyPerm os.FileMode = 0400

// PhaseCerts shells out to kubeadm to generate the necessary PKI.
func PhaseCerts() error {
	// Run kubeadm init phase certs all. This should fill in whatever gaps
	// we have in the provided certs.
	return cmd.Run(
		"kubeadm",
		"init",
		"phase",
		"certs",
		"all",
		"--config",
		constants.KubeadmConfig,
	)
}

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
	clusterConfiguration.KubernetesVersion = constants.KubernetesVersion
	clusterConfiguration.UseHyperKubeImage = true

	// Apply CIS hardening recommendations; only generate encryption token only if we're the bootstrap node
	if err = cis.EnforceMasterRequirements(clusterConfiguration, data.Services.Kubeadm.IsBootstrap()); err != nil {
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
			if err = ioutil.WriteFile(cert.KeyPath, cert.Cert.Key, certPerm); err != nil {
				return fmt.Errorf("write %s: %v", cert.KeyPath, err)
			}
		}
	}
	return err
}

// RequiredFiles returns a slice of the required CA and
// security policies necessary for kubeadm init to function.
// This serves as a base for the list of files that need to
// be synced via trustd from other nodes
func RequiredFiles() []string {
	return []string{
		constants.AuditPolicyPathInitramfs,
		constants.EncryptionConfigInitramfsPath,
		constants.KubeadmCACert,
		constants.KubeadmCAKey,
		constants.KubeadmSACert,
		constants.KubeadmSAKey,
		constants.KubeadmFrontProxyCACert,
		constants.KubeadmFrontProxyCAKey,
		constants.KubeadmEtcdCACert,
		constants.KubeadmEtcdCAKey,
	}
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

// CreateSecurityClients handles instantiating a trustd client connection
// to each trustd endpoint defined in userdata
func CreateSecurityClients(data *userdata.UserData) ([]securityapi.SecurityClient, error) {
	var creds basic.Credentials
	var err error

	trustds := []securityapi.SecurityClient{}

	creds, err = basic.NewCredentials(data.Services.Trustd)
	if err != nil {
		return trustds, err
	}

	// Create a trustd client for each endpoint to set up
	// a fan out approach to gathering the files
	var conn *grpc.ClientConn
	for _, endpoint := range data.Services.Trustd.Endpoints {
		conn, err = basic.NewConnection(endpoint, constants.TrustdPort, creds)
		if err != nil {
			return trustds, err
		}
		trustds = append(trustds, securityapi.NewSecurityClient(conn))
	}
	return trustds, nil
}

// Download handles the retrieval of files from a trustd endpoint
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

// WriteTrustdFiles handles reading the replies from trustd and writing them
// out to a file on disk
func WriteTrustdFiles(requestedFile string, content []byte) (err error) {
	// If the file already exists, no need to write it again
	// TODO: perhaps look at some sort of checksum verification?
	if _, err = os.Stat(requestedFile); err == nil {
		return err
	}

	if err = os.MkdirAll(filepath.Dir(requestedFile), dirPerm); err != nil {
		log.Printf("failed to create directory for %s: %v", requestedFile, err)
		return err
	}

	// Write out content
	var perms os.FileMode
	switch {
	case strings.HasSuffix(requestedFile, "key"):
		perms = keyPerm
	case strings.HasSuffix(requestedFile, "crt"):
		perms = certPerm
	case strings.HasSuffix(requestedFile, "pub"):
		perms = certPerm
	default:
		perms = keyPerm
	}

	if err = ioutil.WriteFile(requestedFile, content, perms); err != nil {
		log.Printf("failed to write file %s: %v", requestedFile, err)
		return err
	}

	return err
}
