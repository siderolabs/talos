package services

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/security/cis"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/conditions"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner"
	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/system/runner/containerd"
	"github.com/autonomy/talos/src/initramfs/cmd/trustd/proto"
	"github.com/autonomy/talos/src/initramfs/pkg/crypto/x509"
	"github.com/autonomy/talos/src/initramfs/pkg/grpc/middleware/auth/basic"
	"github.com/autonomy/talos/src/initramfs/pkg/userdata"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"

	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	configutil "k8s.io/kubernetes/cmd/kubeadm/app/util/config"
)

const kubeadmSH = `#!/bin/bash

set -e

apt-get update -y
apt-get install -y curl

curl -L https://download.docker.com/linux/static/stable/x86_64/docker-18.06.1-ce.tgz | tar -xz --strip-components=1 -C /bin docker/docker
chmod +x /bin/docker

cd /etc/kubernetes

trap 'kubeadm reset --force' ERR

{{- if .Bootstrap }}
kubeadm init --config=kubeadm-config.yaml --ignore-preflight-errors=cri,kubeletversion,requiredipvskernelmodulesavailable --skip-token-print
{{- else }}
kubeadm join --config=kubeadm-config.yaml --ignore-preflight-errors=cri,kubeletversion,requiredipvskernelmodulesavailable
{{- end }}
`

// Kubeadm implements the Service interface. It serves as the concrete type with
// the required methods.
type Kubeadm struct{}

// ID implements the Service interface.
func (k *Kubeadm) ID(data *userdata.UserData) string {
	return "kubeadm"
}

// PreFunc implements the Service interface.
func (k *Kubeadm) PreFunc(data *userdata.UserData) (err error) {
	if err = writeKubeadmScript(data); err != nil {
		return err
	}

	if data.IsMaster() {
		if err = writeKubeadmPKIFiles(data.Security.Kubernetes.CA); err != nil {
			return err
		}
	}

	if err = writeKubeadmConfig(data); err != nil {
		return err
	}

	return nil
}

// PostFunc implements the Service interface.
func (k *Kubeadm) PostFunc(data *userdata.UserData) error {
	if data.IsWorker() {
		return nil
	}

	if data.Services.Trustd.Endpoints == nil {
		return nil
	}

	creds := basic.NewCredentials(
		data.Security.OS.CA.Crt,
		data.Services.Trustd.Username,
		data.Services.Trustd.Password,
	)

	files := []string{
		"/var/etc/kubernetes/audit-policy.yaml",
		"/var/etc/kubernetes/encryptionconfig.yaml",
		"/var/etc/kubernetes/pki/ca.crt",
		"/var/etc/kubernetes/pki/ca.key",
		"/var/etc/kubernetes/pki/sa.key",
		"/var/etc/kubernetes/pki/sa.pub",
		"/var/etc/kubernetes/pki/front-proxy-ca.crt",
		"/var/etc/kubernetes/pki/front-proxy-ca.key",
		"/var/etc/kubernetes/pki/etcd/ca.crt",
		"/var/etc/kubernetes/pki/etcd/ca.key",
		"/var/etc/kubernetes/admin.conf",
	}

	for _, endpoint := range data.Services.Trustd.Endpoints {
		parts := strings.Split(endpoint, ":")
		if len(parts) != 2 {
			return fmt.Errorf("trust endpoint is not valid: %s", endpoint)
		}
		i, err := strconv.Atoi(parts[1])
		if err != nil {
			return err
		}
		conn, err := basic.NewConnection(parts[0], i, creds)
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

// ConditionFunc implements the Service interface.
func (k *Kubeadm) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	var socket string
	switch data.Services.Init.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		socket = constants.ContainerRuntimeDockerSocket
	case constants.ContainerRuntimeCRIO:
		socket = constants.ContainerRuntimeDockerSocket
	}

	files := []string{socket}

	if data.IsControlPlane() {
		files = append(files, "/var/etc/kubernetes/admin.conf")
	}

	return conditions.WaitForFilesToExist(files...)
}

// Start implements the Service interface.
// nolint: dupl
func (k *Kubeadm) Start(data *userdata.UserData) error {
	// Set the image.
	var image string
	if data.Services.Kubeadm != nil && data.Services.Kubeadm.Image != "" {
		image = data.Services.Kubeadm.Image
	} else {
		image = constants.KubernetesImage
	}

	// We only wan't to run kubeadm if it hasn't been ran already.
	if _, err := os.Stat("/var/etc/kubernetes/kubelet.conf"); !os.IsNotExist(err) {
		return nil
	}

	// Set the process arguments.
	args := runner.Args{
		ID:          k.ID(data),
		ProcessArgs: []string{"/bin/kubeadm.sh"},
	}

	// Set the mounts.
	// nolint: dupl
	mounts := []specs.Mount{
		{Type: "cgroup", Destination: "/sys/fs/cgroup", Options: []string{"ro"}},
		{Type: "bind", Destination: "/var/run", Source: "/run", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/var/lib/kubelet", Source: "/var/lib/kubelet", Options: []string{"rbind", "rshared", "rw"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/var/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/crictl", Source: "/bin/crictl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/kubeadm", Source: "/bin/kubeadm", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/kubeadm.sh", Source: "/run/kubeadm.sh", Options: []string{"bind", "ro"}},
	}

	switch data.Services.Init.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		mounts = append(mounts, specs.Mount{Type: "bind", Destination: "/var/lib/docker", Source: "/var/lib/docker", Options: []string{"rbind", "rshared", "rw"}})
	case constants.ContainerRuntimeCRIO:
		mounts = append(mounts, specs.Mount{Type: "bind", Destination: "/var/lib/containers", Source: "/var/lib/containers", Options: []string{"rbind", "rshared", "rw"}})
	}

	env := []string{}
	for key, val := range data.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, val))
	}

	r := containerd.Containerd{}

	return r.Run(
		data,
		args,
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

func writeKubeadmScript(data *userdata.UserData) (err error) {
	contents, err := parse(data)
	if err != nil {
		return err
	}

	return ioutil.WriteFile("/run/kubeadm.sh", contents, os.FileMode(0700))
}

func enforceMasterOverrides(initConfiguration *kubeadmapi.InitConfiguration) {
	initConfiguration.KubernetesVersion = constants.KubernetesVersion
	initConfiguration.UnifiedControlPlaneImage = constants.KubernetesImage
}

func writeKubeadmConfig(data *userdata.UserData) (err error) {
	var b []byte
	if data.IsBootstrap() {
		initConfiguration := data.Services.Kubeadm.Configuration.(*kubeadmapi.InitConfiguration)
		enforceMasterOverrides(initConfiguration)
		if err = cis.EnforceMasterRequirements(initConfiguration); err != nil {
			return err
		}
		b, err = configutil.MarshalKubeadmConfigObject(initConfiguration)
		if err != nil {
			return err
		}
	} else {
		joinConfiguration := data.Services.Kubeadm.Configuration.(*kubeadmapi.JoinConfiguration)
		if err = cis.EnforceWorkerRequirements(joinConfiguration); err != nil {
			return err
		}
		b, err = configutil.MarshalKubeadmConfigObject(joinConfiguration)
		if err != nil {
			return err
		}
	}

	p := path.Dir(constants.KubeadmConfig)
	if err := os.MkdirAll(p, os.ModeDir); err != nil {
		return fmt.Errorf("create %s: %v", p, err)
	}

	if err = ioutil.WriteFile(constants.KubeadmConfig, b, 0400); err != nil {
		return fmt.Errorf("write %s: %v", constants.KubeadmConfig, err)
	}

	return nil
}

func writeKubeadmPKIFiles(data *x509.PEMEncodedCertificateAndKey) (err error) {
	if err = os.MkdirAll(path.Dir(constants.KubeadmCACert), 0600); err != nil {
		return err
	}
	if err = ioutil.WriteFile(constants.KubeadmCACert, data.Crt, 0400); err != nil {
		return fmt.Errorf("write %s: %v", constants.KubeadmCACert, err)
	}

	if err = os.MkdirAll(path.Dir(constants.KubeadmCAKey), 0600); err != nil {
		return err
	}
	if err = ioutil.WriteFile(constants.KubeadmCAKey, data.Key, 0400); err != nil {
		return fmt.Errorf("write %s: %v", constants.KubeadmCAKey, err)
	}

	return nil
}

func parse(data *userdata.UserData) ([]byte, error) {
	aux := struct {
		Bootstrap bool
	}{
		Bootstrap: data.IsBootstrap(),
	}
	t, err := template.New("kubeadm").Parse(kubeadmSH)
	if err != nil {
		return nil, err
	}

	b := []byte{}
	buf := bytes.NewBuffer(b)
	err = t.Execute(buf, aux)

	return buf.Bytes(), err
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
		for _, f := range files {
		L:
			b, err := ioutil.ReadFile(f)
			if err != nil {
				log.Printf("failed to read file %s: %v", f, err)
				time.Sleep(1 * time.Second)
				goto L
			}
			req := &proto.WriteFileRequest{
				Path: f,
				Data: b,
			}
			_, err = client.WriteFile(context.Background(), req)
			if err != nil {
				log.Printf("failed to write file %s: %v", f, err)
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
