/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kubeadm

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"

	kubeadmv1beta2 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"

	"github.com/talos-systems/talos/internal/pkg/cis"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/crypto/x509"
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

	// Flex on running etcd directly on Talos or via Kubeadm
	if data.Services.Etcd != nil && data.Services.Etcd.Enabled {
		clusterConfiguration.Etcd = kubeadmv1beta2.Etcd{
			External: &kubeadmv1beta2.ExternalEtcd{
				// TODO probably need to find a better way to handle obtaining etcd addrs
				// since this becomes an ordering issue. We rely on k8s to discover etcd
				// endpoints, but need etcd endpoints to bring up k8s.
				// We'll set this to 127.0.0.1 for now since mvp will be stacked control
				// plane ( etcd living on the same hosts as masters )
				Endpoints: []string{"https://127.0.0.1:" + strconv.Itoa(constants.KubeadmEtcdListenClientPort)},
				CAFile:    constants.KubeadmEtcdCACert,
				// These are for apiserver -> etcd communication
				CertFile: constants.KubeadmAPIServerEtcdClientCert,
				KeyFile:  constants.KubeadmAPIServerEtcdClientKey,
			},
		}
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
