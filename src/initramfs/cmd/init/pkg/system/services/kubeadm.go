package services

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/system/conditions"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/system/runner"
	"github.com/autonomy/dianemo/src/initramfs/cmd/init/pkg/system/runner/containerd"
	"github.com/autonomy/dianemo/src/initramfs/cmd/trustd/proto"
	"github.com/autonomy/dianemo/src/initramfs/pkg/crypto/x509"
	"github.com/autonomy/dianemo/src/initramfs/pkg/grpc/middleware/auth/basic"
	"github.com/autonomy/dianemo/src/initramfs/pkg/net"
	"github.com/autonomy/dianemo/src/initramfs/pkg/userdata"
	"github.com/containerd/containerd/oci"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"google.golang.org/grpc"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
)

const kubeadmSH = `#!/bin/bash

set -eEou pipefail

cd /etc/kubernetes

apt-get update -y
apt-get install -y curl

curl -L https://download.docker.com/linux/static/stable/x86_64/docker-17.03.2-ce.tgz | tar -xz --strip-components=1 -C /bin docker/docker
chmod +x /bin/docker

trap 'kubeadm reset' ERR

{{- if .Init }}
{{- if eq .Init.Type "initial" }}
kubeadm init --config=kubeadm-config.yaml --ignore-preflight-errors=cri,kubeletversion --skip-token-print
{{- else if eq .Init.Type "dependent" }}
export KUBECONFIG=/etc/kubernetes/admin.conf
kubeadm alpha phase certs all --config kubeadm-config.yaml
kubeadm alpha phase kubelet config write-to-disk --config kubeadm-config.yaml
kubeadm alpha phase kubelet write-env-file --config kubeadm-config.yaml
kubeadm alpha phase kubeconfig kubelet --config kubeadm-config.yaml
# Workaround for clusters running with the DenyEscalatingExec admission controller.
docker run \
	--rm \
	--net=host \
	--volume /etc/kubernetes/pki/etcd:/etc/kubernetes/pki/etcd k8s.gcr.io/etcd:{{ .EtcdVersion }} \
	etcdctl \
		--ca-file /etc/kubernetes/pki/etcd/ca.crt \
		--cert-file /etc/kubernetes/pki/etcd/peer.crt \
		--key-file /etc/kubernetes/pki/etcd/peer.key \
		--endpoints=https://{{ .Init.EtcdEndpoint }}:2379 \
		member add {{ .Init.EtcdMemberName }} https://{{ .IP }}:2380
kubeadm alpha phase etcd local --config kubeadm-config.yaml
kubeadm alpha phase kubeconfig all --config kubeadm-config.yaml
{{- if not .Init.SelfHosted }}
	kubeadm alpha phase controlplane all --config kubeadm-config.yaml
{{- end }}
kubeadm alpha phase mark-master --config kubeadm-config.yaml
echo "successfully joined master node {{ .Hostname }}"
{{- end }}
{{- else }}
kubeadm join --config=kubeadm-config.yaml --ignore-preflight-errors=cri,kubeletversion
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
	contents, err := parse(data)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile("/run/kubeadm.sh", contents, os.FileMode(0700)); err != nil {
		return
	}

	if data.Services.Kubeadm.Init != nil {
		if err = writeKubeadmPKIFiles(data.Security.Kubernetes.CA); err != nil {
			return
		}
	}

	if err = writeKubeadmConfig(data.Services.Kubeadm.Configuration); err != nil {
		return
	}

	return nil
}

// PostFunc implements the Service interface.
func (k *Kubeadm) PostFunc(data *userdata.UserData) (err error) {
	if data.Services.Kubeadm.Init == nil {
		return nil
	}

	if data.Services.Kubeadm.Init.TrustEndpoint == "" {
		return nil
	}

	creds := basic.NewCredentials(
		data.Security.OS.CA.Crt,
		data.Services.Trustd.Username,
		data.Services.Trustd.Password,
	)

	var conn *grpc.ClientConn
	parts := strings.Split(data.Services.Kubeadm.Init.TrustEndpoint, ":")
	if len(parts) != 2 {
		return fmt.Errorf("trust endpoint is not valid")
	}
	i, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	conn, err = basic.NewConnection(parts[0], i, creds)
	if err != nil {
		return
	}
	client := proto.NewTrustdClient(conn)

	files := []string{
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
	if err = writeFiles(client, files); err != nil {
		return
	}

	return nil
}

// ConditionFunc implements the Service interface.
func (k *Kubeadm) ConditionFunc(data *userdata.UserData) conditions.ConditionFunc {
	var conditionFunc conditions.ConditionFunc
	switch data.Services.Kubeadm.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		if data.Services.Kubeadm.Init != nil && data.Services.Kubeadm.Init.Type == constants.KubeadmInitTypeDependent {
			conditionFunc = conditions.WaitForFilesToExist(constants.ContainerRuntimeDockerSocket, "/var/etc/kubernetes/admin.conf")
		} else {
			conditionFunc = conditions.WaitForFileToExist(constants.ContainerRuntimeDockerSocket)
		}
	case constants.ContainerRuntimeCRIO:
		if data.Services.Kubeadm.Init != nil && data.Services.Kubeadm.Init.Type == constants.KubeadmInitTypeDependent {
			conditionFunc = conditions.WaitForFilesToExist(constants.ContainerRuntimeCRIOSocket, "/var/etc/kubernetes/admin.conf")
		} else {
			conditionFunc = conditions.WaitForFileToExist(constants.ContainerRuntimeCRIOSocket)
		}
	}

	return conditionFunc
}

// Start implements the Service interface.
// nolint: dupl
func (k *Kubeadm) Start(data *userdata.UserData) error {
	// We only wan't to run kubeadm if it hasn't been ran already.
	if _, err := os.Stat("/var/etc/kubernetes/kubelet.conf"); !os.IsNotExist(err) {
		return nil
	}

	// Set the image.
	var image string
	if data.Services.Kubeadm != nil && data.Services.Kubeadm.Image != "" {
		image = data.Services.Kubeadm.Image
	} else {
		image = "gcr.io/google_containers/hyperkube:v1.11.2"
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
		{Type: "bind", Destination: "/lib/modules", Source: "/lib/modules", Options: []string{"bind", "shared", "ro"}},
		{Type: "bind", Destination: "/etc/kubernetes", Source: "/var/etc/kubernetes", Options: []string{"bind", "rw"}},
		{Type: "bind", Destination: "/etc/os-release", Source: "/etc/os-release", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/crictl", Source: "/bin/crictl", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/kubeadm", Source: "/bin/kubeadm", Options: []string{"bind", "ro"}},
		{Type: "bind", Destination: "/bin/kubeadm.sh", Source: "/run/kubeadm.sh", Options: []string{"bind", "ro"}},
	}

	switch data.Services.Kubeadm.ContainerRuntime {
	case constants.ContainerRuntimeDocker:
		mounts = append(mounts, specs.Mount{Type: "bind", Destination: "/var/lib/docker", Source: "/var/lib/docker", Options: []string{"rbind", "rshared", "rw"}})
	case constants.ContainerRuntimeCRIO:
		mounts = append(mounts, specs.Mount{Type: "bind", Destination: "/var/lib/containers", Source: "/var/lib/containers", Options: []string{"rbind", "rshared", "rw"}})
	}

	r := containerd.Containerd{}

	return r.Run(
		data,
		args,
		runner.WithContainerImage(image),
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

func writeKubeadmConfig(data string) (err error) {
	p := path.Dir(constants.KubeadmConfig)
	if err := os.MkdirAll(p, os.ModeDir); err != nil {
		return fmt.Errorf("create %s: %v", p, err)
	}
	if err = ioutil.WriteFile(constants.KubeadmConfig, []byte(data), 0400); err != nil {
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
	ip, err := net.IP()
	if err != nil {
		return nil, err
	}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	aux := struct {
		IP          string
		Hostname    string
		Init        *userdata.InitConfiguration
		EtcdVersion string
	}{
		Hostname:    hostname,
		IP:          ip.String(),
		Init:        data.Services.Kubeadm.Init,
		EtcdVersion: kubeadmconstants.DefaultEtcdVersion,
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
