/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/oci"
	criconstants "github.com/containerd/cri/pkg/constants"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/talos-systems/talos/internal/app/init/internal/security/cis"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner"
	"github.com/talos-systems/talos/internal/app/init/pkg/system/runner/containerd"
	"github.com/talos-systems/talos/internal/app/trustd/proto"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/grpc/middleware/auth/basic"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/userdata"

	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	configutil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
)

// Kubeadm implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubeadm struct{}

// ID implements the Service interface.
func (k *Kubeadm) ID(data *userdata.UserData) string {
	return "kubeadm"
}

// PreFunc implements the Service interface.
func (k *Kubeadm) PreFunc(data *userdata.UserData) (err error) {
	if err = writeKubeadmConfig(data); err != nil {
		return err
	}

	if data.IsBootstrap() {
		if err = writeKubeadmPKIFiles(data.Security.Kubernetes.CA); err != nil {
			return err
		}
	} else if data.IsControlPlane() {
		if data.Services.Trustd == nil || data.Services.Trustd.BootstrapNode == "" {
			return nil
		}

		creds := basic.NewCredentials(
			data.Services.Trustd.Username,
			data.Services.Trustd.Password,
		)

		files := []string{
			constants.AuditPolicyPathInitramfs,
			constants.EncryptionConfigInitramfsPath,
		}

		conn, err := basic.NewConnection(data.Services.Trustd.BootstrapNode, constants.TrustdPort, creds)
		if err != nil {
			return err
		}

		client := proto.NewTrustdClient(conn)

		if err := writeFiles(client, files); err != nil {
			return err
		}

	}

	return nil
}

// PostFunc implements the Service interface.
func (k *Kubeadm) PostFunc(data *userdata.UserData) error {
	return nil
}

// ConditionFunc implements the Service interface.
func (k *Kubeadm) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	files := []string{defaults.DefaultAddress}

	return conditions.WaitForFilesToExist(files...)
}

// Start implements the Service interface.
// nolint: dupl
func (k *Kubeadm) Start(data *userdata.UserData) error {
	image := constants.KubernetesImage

	// We only wan't to run kubeadm if it hasn't been ran already.
	if _, err := os.Stat("/etc/kubernetes/kubelet.conf"); !os.IsNotExist(err) {
		return nil
	}

	// Set the process arguments.
	args := runner.Args{
		ID: k.ID(data),
	}

	ignorePreflightErrors := []string{"cri", "kubeletversion", "numcpu", "requiredipvskernelmodulesavailable"}
	ignorePreflightErrors = append(ignorePreflightErrors, data.Services.Kubeadm.IgnorePreflightErrors...)
	ignore := "--ignore-preflight-errors=" + strings.Join(ignorePreflightErrors, ",")

	// sha256 provided key to make it exactly 32 bytes, as required by kubeadm:
	//   https://github.com/kubernetes/kubernetes/blob/master/cmd/kubeadm/app/constants/constants.go : CertificateKeySize
	hashedKey := sha256.Sum256([]byte(data.Services.Kubeadm.CertificateKey))
	encoded := hex.EncodeToString(hashedKey[:])
	certificateKey := "--certificate-key=" + encoded

	switch {
	case data.IsBootstrap() == true:
		args.ProcessArgs = []string{
			"kubeadm",
			"init",
			"--config=/etc/kubernetes/kubeadm-config.yaml",
			certificateKey,
			ignore,
			"--skip-token-print",
			"--skip-certificate-key-print",
			"--experimental-upload-certs",
		}
	case data.IsControlPlane() == true:
		args.ProcessArgs = []string{
			"kubeadm",
			"join",
			"--config=/etc/kubernetes/kubeadm-config.yaml",
			certificateKey,
			ignore,
		}
	default:
		args.ProcessArgs = []string{
			"kubeadm",
			"join",
			"--config=/etc/kubernetes/kubeadm-config.yaml",
			ignore,
		}
	}

	args.ProcessArgs = append(args.ProcessArgs, data.Services.Kubeadm.ExtraArgs...)

	// Set the mounts.
	// nolint: dupl
	mounts := []specs.Mount{
		{Type: "cgroup", Destination: "/sys/fs/cgroup", Options: []string{"ro"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/crictl", Source: "/bin/crictl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/kubeadm", Source: "/bin/kubeadm", Options: []string{"bind", "ro"}},
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	r := containerd.Containerd{}

	return r.Run(
		data,
		args,
		runner.WithNamespace(criconstants.K8sContainerdNamespace),
		runner.WithContainerImage(image),
		runner.WithEnv(env),
		runner.WithOCISpecOpts(
			containerd.WithMemoryLimit(int64(1000000*512)),
			containerd.WithRootfsPropagation("slave"),
			oci.WithMounts(mounts),
			oci.WithHostNamespace(specs.PIDNamespace),
			oci.WithParentCgroupDevices,
			oci.WithPrivileged,
		),
		runner.WithType(runner.Once),
	)
}

func enforceMasterOverrides(initConfiguration *kubeadmapi.InitConfiguration) {
	initConfiguration.KubernetesVersion = constants.KubernetesVersion
	initConfiguration.UseHyperKubeImage = true
}

func writeKubeadmConfig(data *userdata.UserData) (err error) {
	var b []byte
	if data.IsBootstrap() {
		initConfiguration, ok := data.Services.Kubeadm.Configuration.(*kubeadmapi.InitConfiguration)
		if !ok {
			return errors.New("expected InitConfiguration")
		}
		initConfiguration.NodeRegistration.CRISocket = defaults.DefaultAddress
		enforceMasterOverrides(initConfiguration)
		if err = cis.EnforceMasterRequirements(initConfiguration); err != nil {
			return err
		}
		b, err = configutil.MarshalKubeadmConfigObject(initConfiguration)
		if err != nil {
			return err
		}
	} else {
		joinConfiguration, ok := data.Services.Kubeadm.Configuration.(*kubeadmapi.JoinConfiguration)
		if !ok {
			return errors.New("expected JoinConfiguration")
		}
		joinConfiguration.NodeRegistration.CRISocket = defaults.DefaultAddress
		if err = cis.EnforceWorkerRequirements(joinConfiguration); err != nil {
			return err
		}
		b, err = configutil.MarshalKubeadmConfigObject(joinConfiguration)
		if err != nil {
			return err
		}
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

func writeKubeadmPKIFiles(data *x509.PEMEncodedCertificateAndKey) (err error) {
	if data == nil {
		return nil
	}
	if data.Crt != nil {

		if err = os.MkdirAll(path.Dir(constants.KubeadmCACert), 0600); err != nil {
			return err
		}
		if err = ioutil.WriteFile(constants.KubeadmCACert, data.Crt, 0400); err != nil {
			return fmt.Errorf("write %s: %v", constants.KubeadmCACert, err)
		}
	}
	if data.Key != nil {
		if err = os.MkdirAll(path.Dir(constants.KubeadmCAKey), 0600); err != nil {
			return err
		}
		if err = ioutil.WriteFile(constants.KubeadmCAKey, data.Key, 0400); err != nil {
			return fmt.Errorf("write %s: %v", constants.KubeadmCAKey, err)
		}
	}
	return nil
}

func writeFiles(client proto.TrustdClient, files []string) (err error) {
	errChan := make(chan error)
	doneChan := make(chan bool)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelFunc()

	go func() {
		<-ctx.Done()
		errChan <- ctx.Err()
	}()

	go func() {
		var err error
		for _, f := range files {
		L:
			req := &proto.ReadFileRequest{
				Path: f,
			}
			var resp *proto.ReadFileResponse
			if resp, err = client.ReadFile(context.Background(), req); err != nil {
				log.Printf("failed to read file %s: %v", f, err)
				time.Sleep(1 * time.Second)
				goto L
			}
			if err = ioutil.WriteFile(f, resp.Data, 0400); err != nil {
				log.Printf("failed to read file %s: %v", f, err)
				time.Sleep(1 * time.Second)
				goto L
			}
		}
		doneChan <- true
	}()

	select {
	case err := <-errChan:
		return err
	case <-doneChan:
		return nil
	}
}
