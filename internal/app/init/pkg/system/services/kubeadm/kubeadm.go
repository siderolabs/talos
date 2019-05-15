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
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/talos-systems/talos/internal/app/init/internal/security/cis"
	"github.com/talos-systems/talos/internal/app/trustd/proto"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/grpc/middleware/auth/basic"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/userdata"
	"google.golang.org/grpc"

	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	configutil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
)

const dirPerm os.FileMode = 0700
const certPerm os.FileMode = 0600
const keyPerm os.FileMode = 0400

// PhaseCerts shells out to kubeadm to generate the necessary PKI.
func PhaseCerts() error {
	// Run kubeadm init phase certs all. This should fill in whatever gaps
	// we have in the provided certs.
	cmd := exec.Command(
		"kubeadm",
		"init",
		"phase",
		"certs",
		"all",
		"--config",
		constants.KubeadmConfig)
	return cmd.Run()
}

func writeInitConfig(data *userdata.UserData) (b []byte, err error) {
	initConfiguration, ok := data.Services.Kubeadm.Configuration.(*kubeadmapi.InitConfiguration)
	if !ok {
		return b, errors.New("expected InitConfiguration")
	}

	// Hardcodes specific kubeadm config parameters
	initConfiguration.NodeRegistration.CRISocket = constants.ContainerdAddress
	initConfiguration.KubernetesVersion = constants.KubernetesVersion
	initConfiguration.UseHyperKubeImage = true

	// Apply CIS hardening recommendations; only generate encryption token only if we're the bootstrap node
	if err = cis.EnforceMasterRequirements(initConfiguration, data.Services.Kubeadm.IsBootstrap()); err != nil {
		return b, err
	}

	return configutil.MarshalKubeadmConfigObject(initConfiguration)
}

func writeJoinConfig(data *userdata.UserData) (b []byte, err error) {
	joinConfiguration, ok := data.Services.Kubeadm.Configuration.(*kubeadmapi.JoinConfiguration)
	if !ok {
		return b, errors.New("expected JoinConfiguration")
	}
	joinConfiguration.NodeRegistration.CRISocket = constants.ContainerdAddress
	if err = cis.EnforceWorkerRequirements(joinConfiguration); err != nil {
		return b, err
	}
	return configutil.MarshalKubeadmConfigObject(joinConfiguration)
}

// WriteConfig writes out the kubeadm config
func WriteConfig(data *userdata.UserData) (err error) {
	var b []byte

	if data.Services.Kubeadm.IsControlPlane() {
		b, err = writeInitConfig(data)
	} else {
		b, err = writeJoinConfig(data)
	}
	if err != nil {
		return err
	}

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
func FileSet() []*proto.ReadFileRequest {
	files := RequiredFiles()

	fileRequests := []*proto.ReadFileRequest{}
	// Check to see if we already have the file locally
	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			fileRequests = append(fileRequests, &proto.ReadFileRequest{Path: file})
		}
	}

	return fileRequests
}

// CreateTrustdClients handles instantiating a trustd client connection
// to each trustd endpoint defined in userdata
func CreateTrustdClients(data *userdata.UserData) ([]proto.TrustdClient, error) {
	var creds basic.Credentials
	var err error

	trustds := []proto.TrustdClient{}

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
		trustds = append(trustds, proto.NewTrustdClient(conn))
	}
	return trustds, nil
}

// Download handles the retrieval of files from a trustd endpoint
func Download(ctx context.Context, client proto.TrustdClient, file *proto.ReadFileRequest, content chan<- []byte) {
	select {
	case <-ctx.Done():
	case content <- download(ctx, client, file):
	}
}

func download(ctx context.Context, client proto.TrustdClient, file *proto.ReadFileRequest) []byte {
	var (
		resp            *proto.ReadFileResponse
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
